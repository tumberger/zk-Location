package float

import (
	"math"
	"math/big"

	"github.com/consensys/gnark/frontend"

	"gnark-float/gadget"
	"gnark-float/hint"
	"gnark-float/util"
)

type Context struct {
	Api          frontend.API
	Gadget       *gadget.IntGadget
	E            uint // The number of bits in the encoded exponent
	M            uint // The number of bits in the encoded mantissa
	E_MAX        *big.Int
	E_NORMAL_MIN *big.Int
	E_MIN        *big.Int
}

// `FloatVar` represents an IEEE-754 floating point number in the constraint system,
// where the number is encoded as a 1-bit sign, an E-bit exponent, and an M-bit mantissa.
// In the circuit, we don't store the encoded form, but directly record all the components,
// together with a flag indicating whether the number is abnormal (NaN or infinity).
type FloatVar struct {
	// `Sign` is true if and only if the number is negative.
	Sign frontend.Variable
	// `Exponent` is the unbiased Exponent of the number.
	// The biased Exponent is in the range `[0, 2^E - 1]`, and the unbiased Exponent should be
	// in the range `[-2^(E - 1) + 1, 2^(E - 1)]`.
	// However, to save constraints in subsequent operations, we shift the mantissa of a
	// subnormal number to the left so that the most significant bit of the mantissa is 1,
	// and the Exponent is decremented accordingly.
	// Therefore, the minimum Exponent is actually `-2^(E - 1) + 1 - M`, and our Exponent
	// is in the range `[-2^(E - 1) + 1 - M, 2^(E - 1)]`.
	Exponent frontend.Variable
	// `Mantissa` is the Mantissa of the number with explicit leading 1, and hence is either
	// 0 or in the range `[2^M, 2^(M + 1) - 1]`.
	// This is true even for subnormal numbers, where the Mantissa is shifted to the left
	// to make the most significant bit 1.
	// To save constraints when handling NaN, we set the Mantissa of NaN to 0.
	Mantissa frontend.Variable
	// `IsAbnormal` is true if and only if the number is NaN or infinity.
	IsAbnormal frontend.Variable
}

func NewContext(api frontend.API, range_size uint, E, M uint) Context {
	E_MAX := new(big.Int).Lsh(big.NewInt(1), E-1)
	E_NORMAL_MIN := new(big.Int).Sub(big.NewInt(2), E_MAX)
	E_MIN := new(big.Int).Sub(E_NORMAL_MIN, big.NewInt(int64(M+1)))
	return Context{
		Api:          api,
		Gadget:       gadget.New(api, range_size, M+E+1),
		E:            E,
		M:            M,
		E_MAX:        E_MAX,
		E_NORMAL_MIN: E_NORMAL_MIN,
		E_MIN:        E_MIN,
	}
}

// Allocate a variable in the constraint system from a value.
// This function decomposes the value into sign, exponent, and mantissa,
// and enforces they are well-formed.
func (f *Context) NewFloat(v frontend.Variable) FloatVar {
	// Extract sign, exponent, and mantissa from the value
	outputs, err := f.Api.Compiler().NewHint(hint.DecodeFloatHint, 2, v, f.E, f.M)
	if err != nil {
		panic(err)
	}
	s := outputs[0]
	e := outputs[1]
	m := f.Api.Sub(v, f.Api.Add(f.Api.Mul(s, new(big.Int).Lsh(big.NewInt(1), f.E+f.M)), f.Api.Mul(e, new(big.Int).Lsh(big.NewInt(1), f.M))))

	// Enforce the bit length of sign, exponent and mantissa
	f.Api.AssertIsBoolean(s)
	f.Gadget.AssertBitLength(e, f.E, gadget.TightForUnknownRange)
	f.Gadget.AssertBitLength(m, f.M, gadget.TightForUnknownRange)

	exponent_min := new(big.Int).Sub(f.E_NORMAL_MIN, big.NewInt(1))
	exponent_max := f.E_MAX

	// Compute the unbiased exponent
	exponent := f.Api.Add(e, exponent_min)

	mantissa_is_zero := f.Api.IsZero(m)
	mantissa_is_not_zero := f.Api.Sub(big.NewInt(1), mantissa_is_zero)
	f.Api.Compiler().MarkBoolean(mantissa_is_not_zero)
	exponent_is_min := f.Gadget.IsEq(exponent, exponent_min)
	exponent_is_max := f.Gadget.IsEq(exponent, exponent_max)

	// Find how many bits to shift the mantissa to the left to have the `(M - 1)`-th bit equal to 1
	// and prodive it as a hint to the circuit
	outputs, err = f.Api.Compiler().NewHint(hint.NormalizeHint, 1, m, f.M)
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	// Enforce that `shift` is small and `two_to_shift` is equal to `2^shift`.
	// Theoretically, `shift` should be in the range `[0, M]`, but the circuit can only guarantee
	// that `shift` is in the range `[0, M + E]`.
	// However, we will check the range of `shifted_mantissa` later, which will implicitly provide
	// tight upper bounds for `shift` and `two_to_shift`, and thus soundness still holds.
	f.Gadget.AssertBitLength(shift, uint(big.NewInt(int64(f.M)).BitLen()), gadget.Loose)
	two_to_shift := f.Gadget.QueryPowerOf2(shift)

	// Compute the shifted mantissa. Multiplication here is safe because we already know that
	// mantissa is less than `2^M`, and `2^shift` is less than or equal to `2^(M + E)`. If `M` is
	// not too large, overflow should not happen.
	shifted_mantissa := f.Api.Mul(m, two_to_shift)
	// Enforce the shifted mantissa, after removing the leading bit, has only `M - 1` bits,
	// where the leading bit is set to 0 if the mantissa is zero, and set to 1 otherwise.
	// This does not bound the value of `shift` if the mantissa is zero, but it is fine since
	// `shift` is not used in this case.
	// On the other hand, if the mantissa is not zero, then this implies that:
	// 1. `shift` is indeed the left shift count that makes the `(M - 1)`-th bit 1, since otherwise,
	// `shifted_mantissa - F::from(1u128 << (M - 1))` will underflow to a negative number, which
	// takes `F::MODULUS_BIT_SIZE > M - 1` bits to represent.
	// 2. `shifted_mantissa` is less than `2^M`, since otherwise,
	// `shifted_mantissa - F::from(1u128 << (M - 1))` will be greater than `2^(M - 1) - 1`, which
	// takes at least `M` bits to represent.
	f.Gadget.AssertBitLength(
		f.Api.Sub(
			shifted_mantissa,
			f.Api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), f.M-1)),
		),
		f.M-1,
		gadget.TightForSmallAbs,
	)

	exponent =
		f.Api.Select(
			exponent_is_min,
			f.Api.Select(
				mantissa_is_zero,
				// If zero, set the exponent to 0's exponent
				f.Api.Sub(exponent_min, f.M),
				// If subnormal, decrement the exponent by `shift`
				f.Api.Sub(exponent, shift),
			),
			// Otherwise, keep the exponent unchanged
			exponent,
		)
	mantissa := f.Api.Select(
		exponent_is_min,
		// If subnormal, shift the mantissa to the left by 1 to make its `M`-th bit 1
		f.Api.Add(shifted_mantissa, shifted_mantissa),
		f.Api.Select(
			f.Api.And(exponent_is_max, mantissa_is_not_zero),
			// If NaN, set the mantissa to 0
			big.NewInt(0),
			// Otherwise, add `2^M` to the mantissa to make its `M`-th bit 1
			f.Api.Add(m, new(big.Int).Lsh(big.NewInt(1), f.M)),
		),
	)

	return FloatVar{
		Sign:       s,
		Exponent:   exponent,
		Mantissa:   mantissa,
		IsAbnormal: exponent_is_max,
	}
}

// Allocate a constant in the constraint system.
func (f *Context) NewConstant(v uint64) FloatVar {
	components := util.ComponentsOf(v, uint64(f.E), uint64(f.M))

	return FloatVar{
		Sign:       components[0],
		Exponent:   components[1],
		Mantissa:   components[2],
		IsAbnormal: components[3],
	}
}

func (f *Context) NewF32Constant(v float32) FloatVar {
	return f.NewConstant(uint64(math.Float32bits(v)))
}

func (f *Context) NewF64Constant(v float64) FloatVar {
	return f.NewConstant(math.Float64bits(v))
}

// Round the mantissa.
// Note that the precision for subnormal numbers should be smaller than normal numbers, but in
// our representation, the mantissa of subnormal numbers also has `M + 1` bits, and we have to set
// the lower bits of the mantissa to 0 to reduce the precision and make the behavior consistent
// with the specification.
// For this purpose, we use `shift` to specify how many bits in the mantissa should be set to 0.
// If the number is normal or we are confident that the lower bits are already 0, we can set
// `shift` to 0.
// `max_shift` is the maximum value of `shift` that we allow, and a precise upper bound will help
// reduce the number of constraints.
// `half_flag` is a flag that indicates whether we should determine the rounding direction according
// to the equality between the remainder and 1/2.
func (f *Context) round(
	mantissa frontend.Variable,
	mantissa_bit_length uint,
	shift frontend.Variable,
	shift_max uint,
	half_flag frontend.Variable,
) frontend.Variable {
	// Enforce that `two_to_shift` is equal to `2^shift`, where `shift` is known to be small.
	two_to_shift := f.Gadget.QueryPowerOf2(shift)

	r_idx := shift_max + mantissa_bit_length - f.M - 2
	q_idx := r_idx + 1
	p_idx := q_idx + 1
	p_len := f.M
	s_len := r_idx

	// Rewrite the shifted mantissa as `p || q || r || s` (big-endian), where
	// * `p` has `M` bits
	// * `q` and `r` have 1 bit each
	// * `s` has the remaining bits
	// and prodive `p, q, r, s` as hints to the circuit.
	// Instead of right shifting the mantissa by `shift` bits, we left shift the mantissa by
	// `shift_max - shift` bits.
	// In the first case, we need to check `s` has `mantissa_bit_length - M - 2` bits, and the shifted
	// out part (we call it `t`) is greater than or equal to 0 and less than `2^shift`, where the latter
	// is equivalent to checking both `t` and `2^shift - t - 1` have `shift_max` bits.
	// However, in the second case, we only need to check `s` has `shift_max + mantissa_bit_length - M - 2`
	// bits, which costs less than the first case.
	outputs, err := f.Api.Compiler().NewHint(hint.DecomposeMantissaForRoundingHint, 4, mantissa, two_to_shift, shift_max, p_idx, q_idx, r_idx)
	if err != nil {
		panic(err)
	}
	p := outputs[0]
	q := outputs[1]
	r := outputs[2]
	s := outputs[3]

	// Enforce the bit length of `p`, `q`, `r` and `s`
	f.Api.AssertIsBoolean(q)
	f.Api.AssertIsBoolean(r)
	f.Gadget.AssertBitLength(p, p_len, gadget.Loose)
	f.Gadget.AssertBitLength(s, s_len, gadget.TightForSmallAbs)

	// Concatenate `p || q`, `r || s`, and `p || q || r || s`.
	// `p || q` is what we want, i.e., the final mantissa, and `r || s` will be thrown away.
	pq := f.Api.Add(p, p, q)
	rs := f.Api.Add(f.Api.Mul(r, new(big.Int).Lsh(big.NewInt(1), r_idx)), s)
	pqrs := f.Api.Add(f.Api.Mul(pq, new(big.Int).Lsh(big.NewInt(1), q_idx)), rs)

	// Enforce that `(p || q || r || s) << shift` is equal to `mantissa << shift_max`
	// Multiplication here is safe because `p || q || r || s` has `shift_max + mantissa_bit_length` bits,
	// and `2^shift` has at most `shift_max` bits, hence the product has `2 * shift_max + mantissa_bit_length`
	// bits, which (at least for f32 and f64) is less than `F::MODULUS_BIT_SIZE` and will not overflow.
	// This constraint guarantees that `p || q || r || s` is indeed `mantissa << (shift_max - shift)`.
	f.Api.AssertIsEqual(
		f.Api.Mul(pqrs, two_to_shift),
		f.Api.Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), shift_max)),
	)

	// Determine whether `r == 1` and `s == 0`. If so, we need to round the mantissa according to `q`,
	// and otherwise, we need to round the mantissa according to `r`.
	// Also, we use `half_flag` to allow the caller to specify the rounding direction.
	is_half := f.Api.And(f.Gadget.IsEq(rs, new(big.Int).Lsh(big.NewInt(1), r_idx)), half_flag)
	carry := f.Api.Select(is_half, q, r)

	// Round the mantissa according to `carry` and shift it back to the original position.
	return f.Api.Mul(f.Api.Add(pq, carry), two_to_shift)
}

// Fix mantissa and exponent overflow.
func (f *Context) fixOverflow(
	mantissa frontend.Variable,
	mantissa_is_zero frontend.Variable,
	exponent frontend.Variable,
	input_is_abnormal frontend.Variable,
) (frontend.Variable, frontend.Variable, frontend.Variable) {
	// Check if mantissa overflows
	// Since the mantissa without carry is always smaller than `2^(M + 1)`, overflow only happens
	// when the original mantissa is `2^(M + 1) - 1` and the carry is 1. Therefore, the only possible
	// value of mantissa in this case is `2^(M + 1)`.
	mantissa_overflow := f.Gadget.IsEq(mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+1))
	// If mantissa overflows, we need to increment the exponent
	exponent = f.Api.Add(exponent, mantissa_overflow)
	// Check if exponent overflows. If so, the result is abnormal.
	// Also, if the input is already abnormal, the result is of course abnormal.
	is_abnormal := f.Api.Or(f.Gadget.IsPositive(f.Api.Sub(exponent, f.E_MAX), f.E+1), input_is_abnormal)

	return f.Api.Select(
			f.Api.Or(mantissa_overflow, is_abnormal),
			// If mantissa overflows, we right shift the mantissa by 1 and obtain `2^M`.
			// If the result is abnormal, we set the mantissa to infinity's mantissa.
			// We can combine both cases as inifinity's mantissa is `2^M`.
			// We will adjust the mantissa latter if the result is NaN.
			new(big.Int).Lsh(big.NewInt(1), f.M),
			mantissa,
		), f.Api.Select(
			is_abnormal,
			// If the result is abnormal, we set the exponent to infinity/NaN's exponent.
			f.E_MAX,
			f.Api.Select(
				mantissa_is_zero,
				// If the result is 0, we set the exponent to 0's exponent.
				f.E_MIN,
				// Otherwise, return the original exponent.
				exponent,
			),
		), is_abnormal
}

// Enforce the equality between two numbers.
func (f *Context) AssertIsEqual(x, y FloatVar) {
	is_nan := f.Api.Or(
		f.Api.And(x.IsAbnormal, f.Api.IsZero(x.Mantissa)),
		f.Api.And(y.IsAbnormal, f.Api.IsZero(y.Mantissa)),
	)
	f.Api.AssertIsEqual(f.Api.Select(
		is_nan,
		0,
		x.Sign,
	), f.Api.Select(
		is_nan,
		0,
		y.Sign,
	))
	f.Api.AssertIsEqual(f.Api.Sub(x.Exponent, y.Exponent), 0)
	f.Api.AssertIsEqual(x.Mantissa, y.Mantissa)
	f.Api.AssertIsEqual(x.IsAbnormal, y.IsAbnormal)
}

// Enforce the equality between two numbers, relaxed to checking ULP <1 (optimized)
func (f *Context) AssertIsEqualOrULP(x, y FloatVar) {
	is_nan := f.Api.Or(
		f.Api.And(x.IsAbnormal, f.Api.IsZero(x.Mantissa)),
		f.Api.And(y.IsAbnormal, f.Api.IsZero(y.Mantissa)),
	)
	f.Api.AssertIsEqual(f.Api.Select(
		is_nan,
		0,
		x.Sign,
	), f.Api.Select(
		is_nan,
		0,
		y.Sign,
	))

	// x.e == y.e && (x.m == y.m + 1 || y.m == x.m + 1) ,
	cmpOne := f.Api.And(
		f.Gadget.IsEq(x.Exponent, y.Exponent),
		f.Api.Or(
			f.Gadget.IsEq(x.Mantissa, f.Api.Add(y.Mantissa, big.NewInt(1))),
			f.Gadget.IsEq(y.Mantissa, f.Api.Add(x.Mantissa, big.NewInt(1))),
		),
	)

	// x.e == y.e + 1 && x.m == 2^M && y.m == 2^(M + 1) - 1 ,
	cmpTwo := f.Api.And(
		f.Gadget.IsEq(x.Exponent, f.Api.Add(y.Exponent, big.NewInt(1))),
		f.Api.And(
			f.Gadget.IsEq(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), f.M)),
			f.Gadget.IsEq(y.Mantissa, f.Api.Sub(new(big.Int).Lsh(big.NewInt(1), f.M+1), big.NewInt(1))),
		),
	)

	// y.e == x.e + 1 && y.m == 2^M && x.m == 2^(M + 1) - 1
	cmpThree := f.Api.And(
		f.Gadget.IsEq(y.Exponent, f.Api.Add(x.Exponent, big.NewInt(1))),
		f.Api.And(
			f.Gadget.IsEq(y.Mantissa, new(big.Int).Lsh(big.NewInt(1), f.M)),
			f.Gadget.IsEq(x.Mantissa, f.Api.Sub(new(big.Int).Lsh(big.NewInt(1), f.M+1), big.NewInt(1))),
		),
	)

	cmp := f.Api.Or(cmpOne, f.Api.Or(cmpTwo, cmpThree))

	condOne := f.Gadget.IsEq(cmp, 1)

	equalityOne := f.Gadget.IsEq(f.Api.Sub(x.Exponent, y.Exponent), 0)
	equalityTwo := f.Gadget.IsEq(x.Mantissa, y.Mantissa)
	equalityThree := f.Gadget.IsEq(x.IsAbnormal, y.IsAbnormal)

	condTwo := f.Api.And(equalityOne, f.Api.And(equalityTwo, equalityThree))

	f.Api.AssertIsEqual(f.Api.Or(condOne, condTwo), 1)
}

// Enforce the equality between two numbers, relaxed to checking ULP <X
func (f *Context) AssertIsEqualOrCustomULP32(x, y FloatVar, ulp float32) {
	is_nan := f.Api.Or(
		f.Api.And(x.IsAbnormal, f.Api.IsZero(x.Mantissa)),
		f.Api.And(y.IsAbnormal, f.Api.IsZero(y.Mantissa)),
	)
	f.Api.AssertIsEqual(f.Api.Select(
		is_nan,
		0,
		x.Sign,
	), f.Api.Select(
		is_nan,
		0,
		y.Sign,
	))

	ulpFloat := f.NewF32Constant(ulp)

	cmp := f.IsLe(f.Abs(f.Sub(x, y)), ulpFloat)

	condOne := f.Gadget.IsEq(cmp, 1)

	equalityOne := f.Gadget.IsEq(f.Api.Sub(x.Exponent, y.Exponent), 0)
	equalityTwo := f.Gadget.IsEq(x.Mantissa, y.Mantissa)
	equalityThree := f.Gadget.IsEq(x.IsAbnormal, y.IsAbnormal)

	condTwo := f.Api.And(equalityOne, f.Api.And(equalityTwo, equalityThree))

	f.Api.AssertIsEqual(f.Api.Or(condOne, condTwo), 1)
}

// Enforce the equality between two numbers, relaxed to checking ULP <X
func (f *Context) AssertIsEqualOrCustomULP64(x, y FloatVar, ulp float64) {
	is_nan := f.Api.Or(
		f.Api.And(x.IsAbnormal, f.Api.IsZero(x.Mantissa)),
		f.Api.And(y.IsAbnormal, f.Api.IsZero(y.Mantissa)),
	)
	f.Api.AssertIsEqual(f.Api.Select(
		is_nan,
		0,
		x.Sign,
	), f.Api.Select(
		is_nan,
		0,
		y.Sign,
	))

	ulpFloat := f.NewF64Constant(ulp)

	cmp := f.IsLe(f.Abs(f.Sub(x, y)), ulpFloat)

	condOne := f.Gadget.IsEq(cmp, 1)

	equalityOne := f.Gadget.IsEq(f.Api.Sub(x.Exponent, y.Exponent), 0)
	equalityTwo := f.Gadget.IsEq(x.Mantissa, y.Mantissa)
	equalityThree := f.Gadget.IsEq(x.IsAbnormal, y.IsAbnormal)

	condTwo := f.Api.And(equalityOne, f.Api.And(equalityTwo, equalityThree))

	f.Api.AssertIsEqual(f.Api.Or(condOne, condTwo), 1)
}

// Add two numbers.
func (f *Context) Add(x, y FloatVar) FloatVar {
	// Compute `y.exponent - x.exponent`'s absolute value and sign.
	// Since `delta` is the absolute value, `delta >= 0`.
	delta, ex_le_ey := f.Gadget.Abs(f.Api.Sub(y.Exponent, x.Exponent), f.E+1)

	// The exponent of the result is at most `max(x.exponent, y.exponent) + 1`, where 1 is the possible carry.
	exponent := f.Api.Add(
		f.Api.Select(
			ex_le_ey,
			y.Exponent,
			x.Exponent,
		),
		big.NewInt(1),
	)
	// Then we are going to use `delta` to align the mantissas of `x` and `y`.
	// If `delta` is 0, we don't need to shift the mantissas.
	// Otherwise, we need to shift the mantissa of the number with smaller exponent to the right by `delta` bits.
	// Now we check if `delta >= M + 3`, i.e., if the difference is too large.
	// If so, the mantissa of the number with smaller exponent will be completely shifted out, and hence the
	// effect of shifting by `delta` bits is the same as shifting by `M + 3` bits.
	// Therefore, the actual right shift count is `min(delta, M + 3)`.
	// As discussed in `Self::round`, we can shift left by `M + 3 - min(delta, M + 3) = max(M + 3 - delta, 0)`
	// bits instead of shifting right by `min(delta, M + 3)` bits in order to save constraints.
	delta = f.Gadget.Max(
		f.Api.Sub(f.M+3, delta),
		big.NewInt(0),
		f.E,
	)
	// Enforce that `two_to_delta` is equal to `2^delta`, where `delta` is known to be small.
	two_to_delta := f.Gadget.QueryPowerOf2(delta)

	// Compute the signed mantissas
	xx := f.Api.Select(
		x.Sign,
		f.Api.Neg(x.Mantissa),
		x.Mantissa,
	)
	yy := f.Api.Select(
		y.Sign,
		f.Api.Neg(y.Mantissa),
		y.Mantissa,
	)
	// `zz` is the mantissa of the number with smaller exponent, and `ww` is the mantissa of another number.
	zz := f.Api.Select(
		ex_le_ey,
		xx,
		yy,
	)
	ww := f.Api.Sub(f.Api.Add(xx, yy), zz)

	// Align `zz` and `ww`.
	// Naively, we can shift `zz` to the right by `delta` bits and keep `ww` unchanged.
	// However, as mentioned above, we left shift `zz` by `M + 3 - min(delta, M + 3)` bits and `ww` by `M + 3`
	// bits instead for circuit efficiency.
	// Also, note that if `exponent` is subnormal and w.l.o.g. `x.exponent < y.exponent`, then `zz` has
	// `E_NORMAL_MIN - x.exponent` trailing 0s, and `ww` has `E_NORMAL_MIN - y.exponent` trailing 0s.
	// Hence, `zz * 2^delta` has `E_NORMAL_MIN - x.exponent + M + 3 - y.exponent + x.exponent` trailing 0s,
	// and `ww << (M + 3)` has `E_NORMAL_MIN - y.exponent + M + 3` trailing 0s.
	// This implies that `s` also has `E_NORMAL_MIN - y.exponent + M + 3` trailing 0s.
	// Generally, `s` should have `max(E_NORMAL_MIN - max(x.exponent, y.exponent), 0) + M + 3` trailing 0s.
	s := f.Api.Add(f.Api.Mul(zz, two_to_delta), f.Api.Mul(ww, new(big.Int).Lsh(big.NewInt(1), f.M+3)))

	// The shift count is at most `M + 3`, and both `zz` and `ww` have `M + 1` bits, hence the result has at most
	// `(M + 3) + (M + 1) + 1` bits, where 1 is the possible carry.
	mantissa_bit_length := (f.M + 3) + (f.M + 1) + 1

	// Get the sign of the mantissa and find how many bits to shift the mantissa to the left to have the
	// `mantissa_bit_length - 1`-th bit equal to 1.
	// Prodive these values as hints to the circuit
	outputs, err := f.Api.Compiler().NewHint(hint.AbsHint, 2, s)
	if err != nil {
		panic(err)
	}
	mantissa_ge_0 := outputs[0]
	mantissa_abs := outputs[1]
	f.Api.AssertIsBoolean(mantissa_ge_0)
	mantissa_lt_0 := f.Api.Sub(big.NewInt(1), mantissa_ge_0)
	f.Api.Compiler().MarkBoolean(mantissa_lt_0)
	outputs, err = f.Api.Compiler().NewHint(hint.NormalizeHint, 1, mantissa_abs, big.NewInt(int64(mantissa_bit_length)))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	// Enforce that `shift` is small and `two_to_shift` is equal to `2^shift`.
	// Theoretically, `shift` should be in the range `[0, M + 4]`, but the circuit can only guarantee
	// that `shift` is in the range `[0, M + E]`.
	// However, we will check the range of `|s| * two_to_shift` later, which will implicitly provide
	// tight upper bounds for `shift` and `two_to_shift`, and thus soundness still holds.
	f.Gadget.AssertBitLength(shift, uint(big.NewInt(int64(mantissa_bit_length)).BitLen()), gadget.Loose)
	two_to_shift := f.Gadget.QueryPowerOf2(shift)

	// Compute the shifted absolute value of mantissa
	mantissa := f.Api.Mul(
		f.Api.Select(
			mantissa_ge_0,
			s,
			f.Api.Neg(s),
		),
		two_to_shift,
	)

	mantissa_is_zero := f.Api.IsZero(mantissa)
	mantissa_is_not_zero := f.Api.Sub(big.NewInt(1), mantissa_is_zero)
	f.Api.Compiler().MarkBoolean(mantissa_is_not_zero)

	// Enforce that the MSB of the shifted absolute value of mantissa is 1 unless the mantissa is zero.
	// Soundness holds because
	// * `mantissa` is non-negative. Otherwise, `mantissa - !mantissa_is_zero << (mantissa_bit_length - 1)`
	// will be negative and cannot fit in `mantissa_bit_length - 1` bits.
	// * `mantissa` has at most `mantissa_bit_length` bits. Otherwise,
	// `mantissa - !mantissa_is_zero << (mantissa_bit_length - 1)` will be greater than or equal to
	// `2^(mantissa_bit_length - 1)` and cannot fit in `mantissa_bit_length - 1` bits.
	// * `mantissa`'s MSB is 1 unless `mantissa_is_zero`. Otherwise, `mantissa - 1 << (mantissa_bit_length - 1)`
	// will be negative and cannot fit in `mantissa_bit_length - 1` bits.
	f.Gadget.AssertBitLength(
		f.Api.Sub(mantissa, f.Api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))),
		mantissa_bit_length-1,
		gadget.TightForSmallAbs,
	)
	// Decrement the exponent by `shift`.
	exponent = f.Api.Sub(exponent, shift)

	// `mantissa_ge_0` can be directly used to determine the sign of the result, except for the case
	// `-0 + -0`. Therefore, we first check whether the signs of `x` and `y` are the same. If so,
	// we use `x`'s sign as the sign of the result. Otherwise, we use the negation of `mantissa_ge_0`.
	sign := f.Api.Select(
		f.Gadget.IsEq(x.Sign, y.Sign),
		x.Sign,
		mantissa_lt_0,
	)

	mantissa = f.round(
		mantissa,
		mantissa_bit_length,
		// If the result is subnormal, we need to clear the lowest `E_NORMAL_MIN - exponent` bits of rounded
		// mantissa. However, as we are going to show below, the required bits are already 0.
		// Note that `mantissa` is the product of `s` and `2^shift`, where `2^shift` has `shift` trailing 0s,
		// `s` has `E_NORMAL_MIN - max(x.exponent, y.exponent) + M + 3` trailing 0s.
		// In summary, `mantissa` has `E_NORMAL_MIN - max(x.exponent, y.exponent) + M + 3 + shift` trailing 0s
		// and `(M + 3) + (M + 1) + 1` bits in total.
		// After rounding, we choose the `M + 1` MSBs as the rounded mantissa, which should contain at least
		// `E_NORMAL_MIN - max(x.exponent, y.exponent) + shift - 1` trailing 0s.
		// Since `exponent == max(x.exponent, y.exponent) + 1 - shift`, the lowest `E_NORMAL_MIN - exponent`
		// bits of the rounded mantissa should be 0.
		big.NewInt(0),
		0,
		1,
	)

	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		mantissa_is_zero,
		exponent,
		f.Api.Or(x.IsAbnormal, y.IsAbnormal),
	)

	y_is_not_abnormal := f.Api.Sub(big.NewInt(1), y.IsAbnormal)
	f.Api.Compiler().MarkBoolean(y_is_not_abnormal)

	return FloatVar{
		Sign:     sign,
		Exponent: exponent,
		// Rule of addition:
		// |       | +Inf | -Inf | NaN | other |
		// |-------|------|------|-----|-------|
		// | +Inf  | +Inf |  NaN | NaN | +Inf  |
		// | -Inf  |  NaN | -Inf | NaN | -Inf  |
		// | NaN   |  NaN |  NaN | NaN |  NaN  |
		// | other | +Inf | -Inf | NaN |       |
		Mantissa: f.Api.Select(
			x.IsAbnormal,
			// If `x` is abnormal ...
			f.Api.Select(
				f.Api.Or(
					y_is_not_abnormal,
					f.Gadget.IsEq(xx, yy),
				),
				// If `y` is not abnormal, then the result is `x`.
				// If `x`'s signed mantissa is equal to `y`'s, then the result is also `x`.
				x.Mantissa,
				// Otherwise, the result is 0 or NaN, whose mantissa is 0.
				big.NewInt(0),
			),
			// If `x` is not abnormal ...
			f.Api.Select(
				y.IsAbnormal,
				// If `y` is abnormal, then the result is `y`.
				y.Mantissa,
				// Otherwise, the result is our computed mantissa.
				mantissa,
			),
		),
		IsAbnormal: is_abnormal,
	}
}

// Compute the absolute value of the number.
func (f *Context) Abs(x FloatVar) FloatVar {
	return FloatVar{
		Sign:       0,
		Exponent:   x.Exponent,
		Mantissa:   x.Mantissa,
		IsAbnormal: x.IsAbnormal,
	}
}

// Negate the number by flipping the sign.
func (f *Context) Neg(x FloatVar) FloatVar {
	neg_sign := f.Api.Sub(big.NewInt(1), x.Sign)
	f.Api.Compiler().MarkBoolean(neg_sign)
	return FloatVar{
		Sign:       neg_sign,
		Exponent:   x.Exponent,
		Mantissa:   x.Mantissa,
		IsAbnormal: x.IsAbnormal,
	}
}

// Subtract two numbers.
// This is implemented by negating `y` and adding it to `x`.
func (f *Context) Sub(x, y FloatVar) FloatVar {
	return f.Add(x, f.Neg(y))
}

// Multiply two numbers.
func (f *Context) Mul(x, y FloatVar) FloatVar {
	// The result is negative if and only if the signs of x and y are different.
	sign := f.Api.Xor(x.Sign, y.Sign)
	mantissa := f.Api.Mul(x.Mantissa, y.Mantissa)
	// Since both `x.mantissa` and `y.mantissa` are in the range [2^M, 2^(M + 1)), the product is
	// in the range [2^(2M), 2^(2M + 2)) and requires 2M + 2 bits to represent.
	mantissa_bit_length := (f.M + 1) * 2

	// Get the MSB of the mantissa and provide it as a hint to the circuit.
	outputs, err := f.Api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(mantissa_bit_length-1)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.Api.AssertIsBoolean(mantissa_msb)
	// Enforce that `mantissa_msb` is indeed the MSB of the mantissa.
	// Soundness holds because
	// * If `mantissa_msb == 0` but the actual MSB is 1, then the subtraction result will have at least
	// mantissa_bit_length bits.
	// * If `mantissa_msb == 1` but the actual MSB is 0, then the subtraction will underflow to a negative
	// value.
	f.Gadget.AssertBitLength(
		f.Api.Sub(mantissa, f.Api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))),
		mantissa_bit_length-1,
		gadget.TightForSmallAbs,
	)
	// Shift the mantissa to the left to make the MSB 1.
	// Since `mantissa` is in the range `[2^(2M), 2^(2M + 2))`, either the MSB is 1 or the second MSB is 1.
	// Therefore, we can simply double the mantissa if the MSB is 0.
	mantissa = f.Api.Add(
		mantissa,
		f.Api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	// Compute the exponent of the result. We should increment the exponent if the multiplication
	// carries, i.e., if the MSB of the mantissa is 1.
	exponent := f.Api.Add(f.Api.Add(x.Exponent, y.Exponent), mantissa_msb)

	shift_max := f.M + 2
	mantissa = f.round(
		mantissa,
		mantissa_bit_length,
		// If `exponent >= E_NORMAL_MIN`, i.e., the result is normal, we don't need to clear the lower bits.
		// Otherwise, we need to clear `min(E_NORMAL_MIN - exponent, shift_max)` bits of the rounded mantissa.
		f.Gadget.Max(
			f.Gadget.Min(
				f.Api.Sub(f.E_NORMAL_MIN, exponent),
				big.NewInt(int64(shift_max)),
				f.E+1,
			),
			big.NewInt(0),
			f.E+1,
		),
		shift_max,
		1,
	)

	mantissa_is_zero := f.Api.IsZero(mantissa)
	input_is_abnormal := f.Api.Or(x.IsAbnormal, y.IsAbnormal)
	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		mantissa_is_zero,
		exponent,
		input_is_abnormal,
	)

	return FloatVar{
		Sign:     sign,
		Exponent: exponent,
		// If the mantissa before fixing overflow is zero, we reset the final mantissa to 0,
		// as `Self::fix_overflow` incorrectly sets NaN's mantissa to infinity's mantissa.
		Mantissa: f.Api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		IsAbnormal: is_abnormal,
	}
}

// Divide two numbers.
func (f *Context) Div(x, y FloatVar) FloatVar {
	// The result is negative if and only if the signs of `x` and `y` are different.
	sign := f.Api.Xor(x.Sign, y.Sign)
	y_is_zero := f.Api.IsZero(y.Mantissa)

	// If the divisor is 0, we increase it to `2^M`, because we cannot represent an infinite value in circuit.
	y_mantissa := f.Api.Select(
		y_is_zero,
		new(big.Int).Lsh(big.NewInt(1), f.M),
		y.Mantissa,
	)
	// The result's mantissa is the quotient of `x.mantissa << (M + 2)` and `y_mantissa`.
	// Since both `x.mantissa` and `y_mantissa` are in the range `[2^M, 2^(M + 1))`, the quotient is in the range
	// `(2^(M + 1), 2^(M + 3))` and requires `M + 3` bits to represent.
	mantissa_bit_length := (f.M + 2) + 1
	// Compute `(x.mantissa << (M + 2)) / y_mantissa` and get the MSB of the quotient.
	// Provide the quotient and the MSB as hints to the circuit.
	outputs, err := f.Api.Compiler().NewHint(hint.DivHint, 1, x.Mantissa, y_mantissa, big.NewInt(int64(f.M+2)))
	if err != nil {
		panic(err)
	}
	mantissa := outputs[0]
	outputs, err = f.Api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(f.M+2)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.Api.AssertIsBoolean(mantissa_msb)
	flipped_mantissa_msb := f.Api.Sub(big.NewInt(1), mantissa_msb)
	f.Api.Compiler().MarkBoolean(flipped_mantissa_msb)
	// Compute the remainder `(x.mantissa << (M + 2)) % y_mantissa`.
	remainder := f.Api.Sub(f.Api.Mul(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+2)), f.Api.Mul(mantissa, y_mantissa))
	// Enforce that `0 <= remainder < y_mantissa`.
	f.Gadget.AssertBitLength(remainder, f.M+1, gadget.Loose)
	f.Gadget.AssertBitLength(f.Api.Sub(y_mantissa, f.Api.Add(remainder, big.NewInt(1))), f.M+1, gadget.Loose)
	// Enforce that `mantissa_msb` is indeed the MSB of the mantissa.
	// Soundness holds because
	// * If `mantissa_msb == 0` but the actual MSB is 1, then the subtraction result will have at least
	// mantissa_bit_length bits.
	// * If `mantissa_msb == 1` but the actual MSB is 0, then the subtraction will underflow to a negative
	// value.
	f.Gadget.AssertBitLength(f.Api.Sub(mantissa, f.Api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))), mantissa_bit_length-1, gadget.TightForUnknownRange)

	// Since `mantissa` is in the range `[2^(2M), 2^(2M + 2))`, either the MSB is 1 or the second MSB is 1.
	// Therefore, we can simply double the mantissa if the MSB is 0.
	mantissa = f.Api.Add(
		mantissa,
		f.Api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	// Compute the exponent of the result. We should decrement the exponent if the division
	// borrows, i.e., if the MSB of the mantissa is 0.
	exponent := f.Api.Sub(f.Api.Sub(x.Exponent, y.Exponent), flipped_mantissa_msb)

	shift_max := f.M + 2
	mantissa = f.round(
		mantissa,
		mantissa_bit_length,
		// If `exponent >= E_NORMAL_MIN`, i.e., the result is normal, we don't need to clear the lower bits.
		// Otherwise, we need to clear `min(E_NORMAL_MIN - exponent, shift_max)` bits of the rounded mantissa.
		f.Gadget.Max(
			f.Gadget.Min(
				f.Api.Sub(f.E_NORMAL_MIN, exponent),
				big.NewInt(int64(shift_max)),
				f.E+1,
			),
			big.NewInt(0),
			f.E+1,
		),
		shift_max,
		f.Api.IsZero(remainder),
	)

	// If `y` is infinity, the result is zero.
	// If `y` is NaN, the result is NaN.
	// Since both zero and NaN have mantissa 0, we can combine both cases and set the mantissa to 0
	// when `y` is abnormal.
	mantissa_is_zero := f.Api.Or(f.Api.IsZero(mantissa), y.IsAbnormal)

	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		mantissa_is_zero,
		exponent,
		f.Api.Or(x.IsAbnormal, y_is_zero),
	)

	return FloatVar{
		Sign:     sign,
		Exponent: exponent,
		// If the mantissa before fixing overflow is zero, we reset the final mantissa to 0,
		// as `Self::fix_overflow` incorrectly sets NaN's mantissa to infinity's mantissa.
		Mantissa: f.Api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		IsAbnormal: is_abnormal,
	}
}

func (f *Context) Sqrt(x FloatVar) FloatVar {
	delta := f.E_MIN
	if delta.Bit(0) == 1 {
		delta = new(big.Int).Sub(delta, big.NewInt(1))
	}
	// Get the LSB of the exponent and provide it as a hint to the circuit.
	outputs, err := f.Api.Compiler().NewHint(hint.NthBitHint, 1, f.Api.Sub(x.Exponent, delta), 0)
	if err != nil {
		panic(err)
	}
	e_lsb := outputs[0]
	f.Api.AssertIsBoolean(e_lsb)

	// Compute `x.exponent >> 1`
	exponent := f.Api.Mul(
		f.Api.Sub(x.Exponent, e_lsb),
		f.Api.Inverse(big.NewInt(2)),
	)
	// Enforce that `|exponent|` only has `E - 1` bits.
	// This ensures that `e_lsb` is indeed the LSB of the exponent, as otherwise `x.exponent` will be odd,
	// but `x.exponent * 2^(-1)` will not fit in `E - 1` bits for all odd `x.exponent` between `E_MIN` and `E_MAX`.
	f.Gadget.Abs(exponent, f.E-1)

	// TODO: `M + 3` is obtained by empirical analysis. We need to find why it works.
	mantissa_bit_length := f.M + 3
	// We are going to find `n` such that `n^2 <= m < (n + 1)^2`, and `r = m - n^2` decides the rounding direction.
	// To this end, we shift `x.mantissa` to the left to allow a more accurate `r`.
	// TODO: `mantissa_bit_length * 2 - (M + 2)` is obtained by empirical analysis. We need to find why it works.
	m := f.Api.Mul(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length*2-(f.M+2)))
	// `sqrt(2^e * m) == sqrt(2^(e - 1) * 2m)`
	// If `e` is even, then the result is `sqrt(2^e * m) = 2^(e >> 1) * sqrt(m)`.
	// If `e` is odd, then the result is `sqrt(2^(e - 1) * 2m) = 2^(e >> 1) * sqrt(2m)`.
	m = f.Api.Select(
		e_lsb,
		f.Api.Add(m, m),
		m,
	)

	// Compute `sqrt(m)` and find how many bits to shift the mantissa to the left to have the
	// `mantissa_bit_length - 1`-th bit equal to 1.
	// Prodive these values as hints to the circuit.
	outputs, err = f.Api.Compiler().NewHint(hint.SqrtHint, 1, m)
	if err != nil {
		panic(err)
	}
	n := outputs[0]

	// Compute the remainder `r = m - n^2`.
	r := f.Api.Sub(m, f.Api.Mul(n, n))
	// Enforce that `n^2 <= m < (n + 1)^2`.
	f.Gadget.AssertBitLength(r, mantissa_bit_length+1, gadget.Loose)                             // n^2 <= m  =>  m - n^2 >= 0
	f.Gadget.AssertBitLength(f.Api.Sub(f.Api.Add(n, n), r), mantissa_bit_length+1, gadget.Loose) // (n + 1)^2 > m  =>  n^2 + 2n + 1 - m > 0  =>  n^2 + 2n - m >= 0

	n_is_zero := f.Api.IsZero(n)
	n_is_not_zero := f.Api.Sub(big.NewInt(1), n_is_zero)
	f.Api.Compiler().MarkBoolean(n_is_not_zero)

	mantissa := f.round(
		n,
		mantissa_bit_length,
		// The result is always normal or 0, as `exponent = (x.exponent >> 1) - shift > E_NORMAL_MIN`.
		// Therefore, we don't need to clear the lower bits.
		big.NewInt(0),
		0,
		f.Api.IsZero(r),
	)

	// If `x` is negative and `x` is not `-0`, the result is NaN.
	// If `x` is NaN, the result is NaN.
	// If `x` is +infinty, the result is +infinity.
	// Below we combine all these cases.
	is_abnormal := f.Api.Or(
		f.Api.And(x.Sign, n_is_not_zero),
		x.IsAbnormal,
	)

	return FloatVar{
		Sign: x.Sign, // Edge case: sqrt(-0.0) = -0.0
		Exponent: f.Api.Select(
			is_abnormal,
			// If the result is abnormal, we set the exponent to infinity/NaN's exponent.
			f.E_MAX,
			f.Api.Select(
				n_is_zero,
				// If the result is 0, we set the exponent to 0's exponent.
				f.E_MIN,
				// Otherwise, return the original exponent.
				exponent,
			),
		),
		Mantissa: f.Api.Select(
			x.Sign,
			// If `x` is negative, we set the mantissa to NaN's mantissa.
			big.NewInt(0),
			mantissa,
		),
		IsAbnormal: is_abnormal,
	}
}

func (f *Context) less(x, y FloatVar, allow_eq uint) frontend.Variable {
	xe_ge_ye := f.Gadget.IsPositive(f.Api.Sub(x.Exponent, y.Exponent), f.E+1)
	xm_ge_ym := f.Gadget.IsPositive(f.Api.Sub(x.Mantissa, y.Mantissa), f.M+1)

	b := f.Api.Select(
		f.Api.Or(
			f.Api.And(x.IsAbnormal, f.Api.IsZero(x.Mantissa)),
			f.Api.And(y.IsAbnormal, f.Api.IsZero(y.Mantissa)),
		),
		// If either `x` or `y` is NaN, the result is always false.
		0,
		/*
		 * Equivalent to:
		 * ```
		 * if x.sign == y.sign {
		 *     if x.exponent == y.exponent {
		 *         if x.mantissa == y.mantissa {
		 *             return allow_eq;
		 *         } else {
		 *             if x.sign {
		 *                 return x.mantissa > y.mantissa;
		 *             } else {
		 *                 return x.mantissa < y.mantissa;
		 *             }
		 *         }
		 *     } else {
		 *         if x.sign {
		 *             return x.exponent > y.exponent;
		 *         } else {
		 *             return x.exponent < y.exponent;
		 *         }
		 *     }
		 * } else {
		 *     if x.mantissa + y.mantissa == 0 {
		 *         return allow_eq;
		 *     } else {
		 *         return x.sign;
		 *     }
		 * }
		 * ```
		 */
		f.Api.Select(
			f.Gadget.IsEq(x.Sign, y.Sign),
			f.Api.Select(
				f.Gadget.IsEq(x.Exponent, y.Exponent),
				f.Api.Select(
					f.Gadget.IsEq(x.Mantissa, y.Mantissa),
					allow_eq,
					f.Api.Select(
						x.Sign,
						xm_ge_ym,
						f.Api.Sub(big.NewInt(1), xm_ge_ym),
					),
				),
				f.Api.Select(
					x.Sign,
					xe_ge_ye,
					f.Api.Sub(big.NewInt(1), xe_ge_ye),
				),
			),
			f.Api.Select(
				f.Api.IsZero(f.Api.Add(x.Mantissa, y.Mantissa)),
				allow_eq,
				x.Sign,
			),
		),
	)
	f.Api.Compiler().MarkBoolean(b)
	return b
}

func (f *Context) IsLt(x, y FloatVar) frontend.Variable {
	return f.less(x, y, 0)
}

func (f *Context) IsLe(x, y FloatVar) frontend.Variable {
	return f.less(x, y, 1)
}

func (f *Context) IsGt(x, y FloatVar) frontend.Variable {
	return f.less(y, x, 0)
}

func (f *Context) IsGe(x, y FloatVar) frontend.Variable {
	return f.less(y, x, 1)
}

func (f *Context) Trunc(x FloatVar) FloatVar {
	e_ge_0 := f.Gadget.IsPositive(x.Exponent, f.E)
	e := f.Api.Select(
		e_ge_0,
		x.Exponent,
		big.NewInt(-1),
	)
	two_to_e := f.Gadget.QueryPowerOf2(f.Gadget.Max(
		f.Api.Sub(f.M, e),
		big.NewInt(0),
		f.E,
	))
	// Instead of computing `x.Mantissa >> e` directly, we compute `(x.Mantissa << (M + 1)) >> e` first and
	// decompose it later to save constraints.
	m := f.Api.Mul(f.Api.Mul(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+1)), f.Api.Inverse(two_to_e))
	outputs, err := f.Api.Compiler().NewHint(hint.TruncHint, 1, m, f.M+1)
	if err != nil {
		panic(err)
	}
	q := outputs[0]
	r := f.Api.Sub(m, f.Api.Mul(q, new(big.Int).Lsh(big.NewInt(1), f.M+1)))
	// Enforce `q` to be small
	f.Gadget.AssertBitLength(q, f.M+1, gadget.Loose)
	// Enforce that `0 <= r < 2^(M + 1)`, where `2^(M + 1)` is the divisor.
	f.Gadget.AssertBitLength(r, f.M+1, gadget.TightForSmallAbs)

	return FloatVar{
		Sign:       x.Sign,
		Exponent:   f.Api.Select(e_ge_0, x.Exponent, f.E_MIN),
		Mantissa:   f.Api.Mul(q, two_to_e),
		IsAbnormal: x.IsAbnormal,
	}
}

func (f *Context) Floor(x FloatVar) FloatVar {
	e_ge_0 := f.Gadget.IsPositive(x.Exponent, f.E)
	e := f.Api.Select(
		e_ge_0,
		x.Exponent,
		big.NewInt(-1),
	)
	two_to_e := f.Gadget.QueryPowerOf2(f.Gadget.Max(
		f.Api.Sub(f.M, e),
		big.NewInt(0),
		f.E,
	))
	// Instead of computing `x.Mantissa >> e` directly, we compute `(x.Mantissa << (M + 1)) >> e` first and
	// decompose it later to save constraints.
	m := f.Api.Mul(f.Api.Mul(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+1)), f.Api.Inverse(two_to_e))
	outputs, err := f.Api.Compiler().NewHint(hint.FloorHint, 1, m, f.M+1, x.Sign)
	if err != nil {
		panic(err)
	}
	// If `x` is positive, then `q` is the floor of `m / 2^(M + 1)`, and the remainder `r` is positive.
	// Otherwise, `q` is the ceiling of `m / 2^(M + 1)`, and the remainder `r` is negative.
	q := outputs[0]
	r := f.Api.Sub(m, f.Api.Mul(q, new(big.Int).Lsh(big.NewInt(1), f.M+1)))
	// Enforce `q` to be small
	f.Gadget.AssertBitLength(q, f.M+1, gadget.Loose)
	// Enforce that `0 <= |r| < 2^(M + 1)`, where `2^(M + 1)` is the divisor.
	f.Gadget.AssertBitLength(f.Api.Select(x.Sign, f.Api.Neg(r), r), f.M+1, gadget.TightForSmallAbs)

	mantissa := f.Api.Mul(q, two_to_e)
	// `mantissa` may overflow when `x` is negative, so we need to fix it.
	mantissa_overflow := f.Gadget.IsEq(mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+1))
	mantissa = f.Api.Select(
		mantissa_overflow,
		new(big.Int).Lsh(big.NewInt(1), f.M),
		mantissa,
	)
	e = f.Api.Add(e, mantissa_overflow)

	return FloatVar{
		Sign: x.Sign,
		Exponent: f.Api.Select(
			f.Api.And(
				f.Api.IsZero(mantissa),
				f.Api.Sub(big.NewInt(1), x.IsAbnormal),
			),
			f.E_MIN,
			e,
		),
		Mantissa:   mantissa,
		IsAbnormal: x.IsAbnormal,
	}
}

func (f *Context) Ceil(x FloatVar) FloatVar {
	return f.Neg(f.Floor(f.Neg(x)))
}

// Convert a float to an integer (f64 to i64, f32 to i32).
// A negative integer will be represented as `r - |x|`, where `r` is the order of the native field.
// The caller should ensure that `x` is obtained from `Trunc`, `Floor` or `Ceil`.
// Also, `x`'s exponent should not be too large. Otherwise, the proof verification will fail.
func (f *Context) ToInt(x FloatVar) frontend.Variable {
	exponent_is_min := f.Gadget.IsEq(x.Exponent, f.E_MIN)
	two_to_e := f.Gadget.QueryPowerOf2(f.Api.Select(
		exponent_is_min,
		big.NewInt(0),
		x.Exponent,
	))
	v := f.Api.Mul(f.Api.Mul(x.Mantissa, two_to_e), f.Api.Inverse(new(big.Int).Lsh(big.NewInt(1), f.M)))
	return f.Api.Select(
		x.Sign,
		f.Api.Neg(v),
		v,
	)
}

func (f *Context) Select(c frontend.Variable, x, y FloatVar) FloatVar {
	return FloatVar{
		Sign:       f.Api.Select(c, x.Sign, y.Sign),
		Exponent:   f.Api.Select(c, x.Exponent, y.Exponent),
		Mantissa:   f.Api.Select(c, x.Mantissa, y.Mantissa),
		IsAbnormal: f.Api.Select(c, x.IsAbnormal, y.IsAbnormal),
	}
}
