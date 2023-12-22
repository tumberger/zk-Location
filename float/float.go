package float

import (
	"math/big"

	"github.com/consensys/gnark/frontend"

	"gnark-float/gadget"
	"gnark-float/hint"
)

type Context struct {
	api          frontend.API
	gadget       gadget.IntGadget
	E            uint // The number of bits in the encoded exponent
	M            uint // The number of bits in the encoded mantissa
	E_MAX        *big.Int
	E_NORMAL_MIN *big.Int
	E_MIN        *big.Int
}

// `FloatVar` represents a IEEE-754 floating point number in the constraint system,
// where the number is encoded as a 1-bit sign, an E-bit exponent, and an M-bit mantissa.
// In the circuit, we don't store the encoded form, but directly record all the components,
// together with a flag indicating whether the number is abnormal (NaN or infinity).
type FloatVar struct {
	// `sign` is true if and only if the number is negative.
	sign frontend.Variable
	// `exponent` is the unbiased exponent of the number.
	// The biased exponent is in the range `[0, 2^E - 1]`, and the unbiased exponent should be
	// in the range `[-2^(E - 1) + 1, 2^(E - 1)]`.
	// However, to save constraints in subsequent operations, we shift the mantissa of a
	// subnormal number to the left so that the most significant bit of the mantissa is 1,
	// and the exponent is decremented accordingly.
	// Therefore, the minimum exponent is actually `-2^(E - 1) + 1 - M`, and our exponent
	// is in the range `[-2^(E - 1) + 1 - M, 2^(E - 1)]`.
	exponent frontend.Variable
	// `mantissa` is the mantissa of the number with explicit leading 1, and hence is either
	// 0 or in the range `[2^M, 2^(M + 1) - 1]`.
	// This is true even for subnormal numbers, where the mantissa is shifted to the left
	// to make the most significant bit 1.
	// To save constraints when handling NaN, we set the mantissa of NaN to 0.
	mantissa frontend.Variable
	// `is_abnormal` is true if and only if the number is NaN or infinity.
	is_abnormal frontend.Variable
}

func NewContext(api frontend.API, E, M uint) Context {
	E_MAX := new(big.Int).Lsh(big.NewInt(1), E-1)
	E_NORMAL_MIN := new(big.Int).Sub(big.NewInt(2), E_MAX)
	E_MIN := new(big.Int).Sub(E_NORMAL_MIN, big.NewInt(int64(M+1)))
	return Context{
		api:          api,
		gadget:       gadget.New(api),
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
	outputs, err := f.api.Compiler().NewHint(hint.DecodeFloatHint, 2, v, f.E, f.M)
	if err != nil {
		panic(err)
	}
	s := outputs[0]
	e := outputs[1]
	m := f.api.Sub(v, f.api.Add(f.api.Mul(s, new(big.Int).Lsh(big.NewInt(1), f.E+f.M)), f.api.Mul(e, new(big.Int).Lsh(big.NewInt(1), f.M))))

	// Enforce the bit length of sign, exponent and mantissa
	f.api.AssertIsBoolean(s)
	f.gadget.AssertBitLength(e, f.E)
	f.gadget.AssertBitLength(m, f.M)

	exponent_min := new(big.Int).Sub(f.E_NORMAL_MIN, big.NewInt(1))
	exponent_max := f.E_MAX

	// Compute the unbiased exponent
	exponent := f.api.Add(e, exponent_min)

	mantissa_is_zero := f.api.IsZero(m)
	mantissa_is_not_zero := f.api.Sub(big.NewInt(1), mantissa_is_zero)
	f.api.Compiler().MarkBoolean(mantissa_is_not_zero)
	exponent_is_min := f.gadget.IsEq(exponent, exponent_min)
	exponent_is_max := f.gadget.IsEq(exponent, exponent_max)

	// Find how many bits to shift the mantissa to the left to have the `(M - 1)`-th bit equal to 1
	// and prodive it as a hint to the circuit
	outputs, err = f.api.Compiler().NewHint(hint.NormalizeHint, 2, m, big.NewInt(int64(f.M)))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce `(shift, two_to_shift)` is in lookup table `[0, M]`

	// Compute the shifted mantissa. Multiplication here is safe because we already know that
	// mantissa is less than `2^M`, and `2^shift` is less than or equal to `2^M`. If `M` is not too large,
	// overflow should not happen.
	shifted_mantissa := f.api.Mul(m, two_to_shift)
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
	f.gadget.AssertBitLength(
		f.api.Sub(
			shifted_mantissa,
			f.api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), f.M-1)),
		),
		f.M-1,
	)

	exponent = f.api.Select(
		exponent_is_min,
		// If subnormal, decrement the exponent by `shift`
		f.api.Sub(exponent, shift),
		// Otherwise, keep the exponent unchanged
		exponent,
	)
	mantissa := f.api.Select(
		exponent_is_min,
		// If subnormal, shift the mantissa to the left by 1 to make its `M`-th bit 1
		f.api.Add(shifted_mantissa, shifted_mantissa),
		f.api.Select(
			f.api.And(exponent_is_max, mantissa_is_not_zero),
			// If NaN, set the mantissa to 0
			big.NewInt(0),
			// Otherwise, add `2^M` to the mantissa to make its `M`-th bit 1
			f.api.Add(m, new(big.Int).Lsh(big.NewInt(1), f.M)),
		),
	)

	return FloatVar{
		sign:        s,
		exponent:    exponent,
		mantissa:    mantissa,
		is_abnormal: exponent_is_max,
	}
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
	outputs, err := f.api.Compiler().NewHint(hint.PowerOfTwoHint, 1, shift)
	if err != nil {
		panic(err)
	}
	two_to_shift := outputs[0]
	// TODO: enforce `(u, two_to_u)` is in lookup table `[0, u_max]`

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
	outputs, err = f.api.Compiler().NewHint(hint.DecomposeMantissaForRoundingHint, 4, mantissa, two_to_shift, shift_max, p_idx, q_idx, r_idx)
	if err != nil {
		panic(err)
	}
	p := outputs[0]
	q := outputs[1]
	r := outputs[2]
	s := outputs[3]

	// Enforce the bit length of `p`, `q`, `r` and `s`
	f.api.AssertIsBoolean(q)
	f.api.AssertIsBoolean(r)
	f.gadget.AssertBitLength(p, p_len)
	f.gadget.AssertBitLength(s, s_len)

	// Concatenate `p || q`, `r || s`, and `p || q || r || s`.
	// `p || q` is what we want, i.e., the final mantissa, and `r || s` will be thrown away.
	pq := f.api.Add(p, p, q)
	rs := f.api.Add(f.api.Mul(r, new(big.Int).Lsh(big.NewInt(1), r_idx)), s)
	pqrs := f.api.Add(f.api.Mul(pq, new(big.Int).Lsh(big.NewInt(1), q_idx)), rs)

	// Enforce that `(p || q || r || s) << shift` is equal to `mantissa << shift_max`
	// Multiplication here is safe because `p || q || r || s` has `shift_max + mantissa_bit_length` bits,
	// and `2^shift` has at most `shift_max` bits, hence the product has `2 * shift_max + mantissa_bit_length`
	// bits, which (at least for f32 and f64) is less than `F::MODULUS_BIT_SIZE` and will not overflow.
	// This constraint guarantees that `p || q || r || s` is indeed `mantissa << (shift_max - shift)`.
	f.api.AssertIsEqual(
		f.api.Mul(pqrs, two_to_shift),
		f.api.Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), shift_max)),
	)

	// Determine whether `r == 1` and `s == 0`. If so, we need to round the mantissa according to `q`,
	// and otherwise, we need to round the mantissa according to `r`.
	// Also, we use `half_flag` to allow the caller to specify the rounding direction.
	is_half := f.api.And(f.gadget.IsEq(rs, new(big.Int).Lsh(big.NewInt(1), r_idx)), half_flag)
	carry := f.api.Select(is_half, q, r)

	// Round the mantissa according to `carry` and shift it back to the original position.
	return f.api.Mul(f.api.Add(pq, carry), two_to_shift)
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
	mantissa_overflow := f.gadget.IsEq(mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+1))
	// If mantissa overflows, we need to increment the exponent
	exponent = f.api.Add(exponent, mantissa_overflow)
	// Check if exponent overflows. If so, the result is abnormal.
	// Also, if the input is already abnormal, the result is of course abnormal.
	is_abnormal := f.api.Or(f.gadget.IsPositive(f.api.Sub(exponent, f.E_MAX), f.E+1), input_is_abnormal)

	return f.api.Select(
			f.api.Or(mantissa_overflow, is_abnormal),
			// If mantissa overflows, we right shift the mantissa by 1 and obtain `2^M`.
			// If the result is abnormal, we set the mantissa to infinity's mantissa.
			// We can combine both cases as inifinity's mantissa is `2^M`.
			// We will adjust the mantissa latter if the result is NaN.
			new(big.Int).Lsh(big.NewInt(1), f.M),
			mantissa,
		), f.api.Select(
			is_abnormal,
			// If the result is abnormal, we set the exponent to infinity/NaN's exponent.
			f.E_MAX,
			f.api.Select(
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
	f.api.Println(x.sign, x.exponent, x.mantissa, x.is_abnormal)
	f.api.Println(y.sign, y.exponent, y.mantissa, y.is_abnormal)
	is_nan := f.api.Or(
		f.api.And(x.is_abnormal, f.api.IsZero(x.mantissa)),
		f.api.And(y.is_abnormal, f.api.IsZero(y.mantissa)),
	)
	f.api.AssertIsEqual(f.api.Select(
		is_nan,
		0,
		x.sign,
	), f.api.Select(
		is_nan,
		0,
		y.sign,
	))
	f.api.AssertIsEqual(f.api.Sub(x.exponent, y.exponent), 0)
	f.api.AssertIsEqual(x.mantissa, y.mantissa)
	f.api.AssertIsEqual(x.is_abnormal, y.is_abnormal)
}

// Add two numbers.
func (f *Context) Add(x, y FloatVar) FloatVar {
	// Compute `y.exponent - x.exponent`'s absolute value and sign.
	// Since `delta` is the absolute value, `delta >= 0`.
	delta, ex_le_ey := f.gadget.Abs(f.api.Sub(y.exponent, x.exponent), f.E+1)

	// The exponent of the result is at most `max(x.exponent, y.exponent) + 1`, where 1 is the possible carry.
	exponent := f.api.Add(
		f.api.Select(
			ex_le_ey,
			y.exponent,
			x.exponent,
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
	delta = f.gadget.Max(
		f.api.Sub(f.M+3, delta),
		big.NewInt(0),
		f.E,
	)
	outputs, err := f.api.NewHint(
		hint.PowerOfTwoHint,
		1,
		delta,
	)
	if err != nil {
		panic(err)
	}
	two_to_delta := outputs[0]
	// TODO: enforce `(delta, two_to_delta)` is in lookup table `[0, >=M + 3]`

	// Compute the signed mantissas
	xx := f.api.Select(
		x.sign,
		f.api.Neg(x.mantissa),
		x.mantissa,
	)
	yy := f.api.Select(
		y.sign,
		f.api.Neg(y.mantissa),
		y.mantissa,
	)
	// `zz` is the mantissa of the number with smaller exponent, and `ww` is the mantissa of another number.
	zz := f.api.Select(
		ex_le_ey,
		xx,
		yy,
	)
	ww := f.api.Sub(f.api.Add(xx, yy), zz)

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
	s := f.api.Add(f.api.Mul(zz, two_to_delta), f.api.Mul(ww, new(big.Int).Lsh(big.NewInt(1), f.M+3)))

	// The shift count is at most `M + 3`, and both `zz` and `ww` have `M + 1` bits, hence the result has at most
	// `(M + 3) + (M + 1) + 1` bits, where 1 is the possible carry.
	mantissa_bit_length := (f.M + 3) + (f.M + 1) + 1

	// Get the sign of the mantissa and find how many bits to shift the mantissa to the left to have the
	// `mantissa_bit_length - 1`-th bit equal to 1.
	// Prodive these values as hints to the circuit
	outputs, err = f.api.Compiler().NewHint(hint.AbsHint, 2, s)
	if err != nil {
		panic(err)
	}
	mantissa_ge_0 := outputs[0]
	mantissa_abs := outputs[1]
	f.api.AssertIsBoolean(mantissa_ge_0)
	mantissa_lt_0 := f.api.Sub(big.NewInt(1), mantissa_ge_0)
	f.api.Compiler().MarkBoolean(mantissa_lt_0)
	outputs, err = f.api.Compiler().NewHint(hint.NormalizeHint, 2, mantissa_abs, big.NewInt(int64(mantissa_bit_length)))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce range of shift `[0, M + 4]`
	// TODO: enforce `(shift, two_to_shift)` is in lookup table

	// Compute the shifted absolute value of mantissa
	mantissa := f.api.Mul(
		f.api.Select(
			mantissa_ge_0,
			s,
			f.api.Neg(s),
		),
		two_to_shift,
	)

	mantissa_is_zero := f.api.IsZero(mantissa)
	mantissa_is_not_zero := f.api.Sub(big.NewInt(1), mantissa_is_zero)
	f.api.Compiler().MarkBoolean(mantissa_is_not_zero)

	// Enforce that the MSB of the shifted absolute value of mantissa is 1 unless the mantissa is zero.
	// Soundness holds because
	// * `mantissa` is non-negative. Otherwise, `mantissa - !mantissa_is_zero << (mantissa_bit_length - 1)`
	// will be negative and cannot fit in `mantissa_bit_length - 1` bits.
	// * `mantissa` has at most `mantissa_bit_length` bits. Otherwise,
	// `mantissa - !mantissa_is_zero << (mantissa_bit_length - 1)` will be greater than or equal to
	// `2^(mantissa_bit_length - 1)` and cannot fit in `mantissa_bit_length - 1` bits.
	// * `mantissa`'s MSB is 1 unless `mantissa_is_zero`. Otherwise, `mantissa - 1 << (mantissa_bit_length - 1)`
	// will be negative and cannot fit in `mantissa_bit_length - 1` bits.
	f.gadget.AssertBitLength(
		f.api.Sub(mantissa, f.api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))),
		mantissa_bit_length-1,
	)
	// Decrement the exponent by `shift`.
	exponent = f.api.Sub(exponent, shift)

	// `mantissa_ge_0` can be directly used to determine the sign of the result, except for the case
	// `-0 + -0`. Therefore, we first check whether the signs of `x` and `y` are the same. If so,
	// we use `x`'s sign as the sign of the result. Otherwise, we use the negation of `mantissa_ge_0`.
	sign := f.api.Select(
		f.gadget.IsEq(x.sign, y.sign),
		x.sign,
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
		f.api.Or(x.is_abnormal, y.is_abnormal),
	)

	y_is_not_abnormal := f.api.Sub(big.NewInt(1), y.is_abnormal)
	f.api.Compiler().MarkBoolean(y_is_not_abnormal)

	return FloatVar{
		sign:     sign,
		exponent: exponent,
		// Rule of addition:
		// |       | +Inf | -Inf | NaN | other |
		// |-------|------|------|-----|-------|
		// | +Inf  | +Inf |   0  | NaN | +Inf  |
		// | -Inf  |   0  | -Inf | NaN | -Inf  |
		// | NaN   |  NaN |  NaN | NaN |  NaN  |
		// | other | +Inf | -Inf | NaN |       |
		mantissa: f.api.Select(
			x.is_abnormal,
			// If `x` is abnormal ...
			f.api.Select(
				f.api.Or(
					y_is_not_abnormal,
					f.gadget.IsEq(xx, yy),
				),
				// If `y` is not abnormal, then the result is `x`.
				// If `x`'s signed mantissa is equal to `y`'s, then the result is also `x`.
				x.mantissa,
				// Otherwise, the result is 0 or NaN, whose mantissa is 0.
				big.NewInt(0),
			),
			// If `x` is not abnormal ...
			f.api.Select(
				y.is_abnormal,
				// If `y` is abnormal, then the result is `y`.
				y.mantissa,
				// Otherwise, the result is our computed mantissa.
				mantissa,
			),
		),
		is_abnormal: is_abnormal,
	}
}

// Negate the number by flipping the sign.
func (f *Context) Neg(x FloatVar) FloatVar {
	neg_sign := f.api.Sub(big.NewInt(1), x.sign)
	f.api.Compiler().MarkBoolean(neg_sign)
	return FloatVar{
		sign:        neg_sign,
		exponent:    x.exponent,
		mantissa:    x.mantissa,
		is_abnormal: x.is_abnormal,
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
	sign := f.api.Xor(x.sign, y.sign)
	mantissa := f.api.Mul(x.mantissa, y.mantissa)
	// Since both `x.mantissa` and `y.mantissa` are in the range [2^M, 2^(M + 1)), the product is
	// in the range [2^(2M), 2^(2M + 2)) and requires 2M + 2 bits to represent.
	mantissa_bit_length := (f.M + 1) * 2

	// Get the MSB of the mantissa and provide it as a hint to the circuit.
	outputs, err := f.api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(mantissa_bit_length-1)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.api.AssertIsBoolean(mantissa_msb)
	// Enforce that `mantissa_msb` is indeed the MSB of the mantissa.
	// Soundness holds because
	// * If `mantissa_msb == 0` but the actual MSB is 1, then the subtraction result will have at least
	// mantissa_bit_length bits.
	// * If `mantissa_msb == 1` but the actual MSB is 0, then the subtraction will underflow to a negative
	// value.
	f.gadget.AssertBitLength(
		f.api.Sub(mantissa, f.api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))),
		mantissa_bit_length-1,
	)
	// Shift the mantissa to the left to make the MSB 1.
	// Since `mantissa` is in the range `[2^(2M), 2^(2M + 2))`, either the MSB is 1 or the second MSB is 1.
	// Therefore, we can simply double the mantissa if the MSB is 0.
	mantissa = f.api.Add(
		mantissa,
		f.api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	// Compute the exponent of the result. We should increment the exponent if the multiplication
	// carries, i.e., if the MSB of the mantissa is 1.
	exponent := f.api.Add(f.api.Add(x.exponent, y.exponent), mantissa_msb)

	shift_max := f.M + 2
	mantissa = f.round(
		mantissa,
		mantissa_bit_length,
		// If `exponent >= E_NORMAL_MIN`, i.e., the result is normal, we don't need to clear the lower bits.
		// Otherwise, we need to clear `min(E_NORMAL_MIN - exponent, shift_max)` bits of the rounded mantissa.
		f.gadget.Max(
			f.gadget.Min(
				f.api.Sub(f.E_NORMAL_MIN, exponent),
				big.NewInt(int64(shift_max)),
				f.E+1,
			),
			big.NewInt(0),
			f.E+1,
		),
		shift_max,
		1,
	)

	mantissa_is_zero := f.api.IsZero(mantissa)
	input_is_abnormal := f.api.Or(x.is_abnormal, y.is_abnormal)
	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		mantissa_is_zero,
		exponent,
		input_is_abnormal,
	)

	return FloatVar{
		sign:     sign,
		exponent: exponent,
		// If the mantissa before fixing overflow is zero, we reset the final mantissa to 0,
		// as `Self::fix_overflow` incorrectly sets NaN's mantissa to infinity's mantissa.
		mantissa: f.api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		is_abnormal: is_abnormal,
	}
}

// Divide two numbers.
func (f *Context) Div(x, y FloatVar) FloatVar {
	// The result is negative if and only if the signs of `x` and `y` are different.
	sign := f.api.Xor(x.sign, y.sign)
	y_is_zero := f.api.IsZero(y.mantissa)

	// If the divisor is 0, we increase it to `2^M`, because we cannot represent an infinite value in circuit.
	y_mantissa := f.api.Select(
		y_is_zero,
		new(big.Int).Lsh(big.NewInt(1), f.M),
		y.mantissa,
	)
	// The result's mantissa is the quotient of `x.mantissa << (M + 2)` and `y_mantissa`.
	// Since both `x.mantissa` and `y_mantissa` are in the range `[2^M, 2^(M + 1))`, the quotient is in the range
	// `(2^(M + 1), 2^(M + 3))` and requires `M + 3` bits to represent.
	mantissa_bit_length := (f.M + 2) + 1
	// Compute `(x.mantissa << (M + 2)) / y_mantissa` and get the MSB of the quotient.
	// Provide the quotient and the MSB as hints to the circuit.
	outputs, err := f.api.Compiler().NewHint(hint.DivHint, 1, x.mantissa, y_mantissa, big.NewInt(int64(f.M+2)))
	if err != nil {
		panic(err)
	}
	mantissa := outputs[0]
	outputs, err = f.api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(f.M+2)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.api.AssertIsBoolean(mantissa_msb)
	flipped_mantissa_msb := f.api.Sub(big.NewInt(1), mantissa_msb)
	f.api.Compiler().MarkBoolean(flipped_mantissa_msb)
	// Compute the remainder `(x.mantissa << (M + 2)) % y_mantissa`.
	remainder := f.api.Sub(f.api.Mul(x.mantissa, new(big.Int).Lsh(big.NewInt(1), f.M+2)), f.api.Mul(mantissa, y_mantissa))
	// Enforce that `0 <= remainder < y_mantissa`.
	f.gadget.AssertBitLength(remainder, f.M+1)
	f.gadget.AssertBitLength(f.api.Sub(y_mantissa, f.api.Add(remainder, big.NewInt(1))), f.M+1)
	// Enforce that `mantissa_msb` is indeed the MSB of the mantissa.
	// Soundness holds because
	// * If `mantissa_msb == 0` but the actual MSB is 1, then the subtraction result will have at least
	// mantissa_bit_length bits.
	// * If `mantissa_msb == 1` but the actual MSB is 0, then the subtraction will underflow to a negative
	// value.
	f.gadget.AssertBitLength(f.api.Sub(mantissa, f.api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), mantissa_bit_length-1))), mantissa_bit_length-1)

	// Since `mantissa` is in the range `[2^(2M), 2^(2M + 2))`, either the MSB is 1 or the second MSB is 1.
	// Therefore, we can simply double the mantissa if the MSB is 0.
	mantissa = f.api.Add(
		mantissa,
		f.api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	// Compute the exponent of the result. We should decrement the exponent if the division
	// borrows, i.e., if the MSB of the mantissa is 0.
	exponent := f.api.Sub(f.api.Sub(x.exponent, y.exponent), flipped_mantissa_msb)

	shift_max := f.M + 2
	mantissa = f.round(
		mantissa,
		mantissa_bit_length,
		// If `exponent >= E_NORMAL_MIN`, i.e., the result is normal, we don't need to clear the lower bits.
		// Otherwise, we need to clear `min(E_NORMAL_MIN - exponent, shift_max)` bits of the rounded mantissa.
		f.gadget.Max(
			f.gadget.Min(
				f.api.Sub(f.E_NORMAL_MIN, exponent),
				big.NewInt(int64(shift_max)),
				f.E+1,
			),
			big.NewInt(0),
			f.E+1,
		),
		shift_max,
		f.api.IsZero(remainder),
	)

	// If `y` is infinity, the result is zero.
	// If `y` is NaN, the result is NaN.
	// Since both zero and NaN have mantissa 0, we can combine both cases and set the mantissa to 0
	// when `y` is abnormal.
	mantissa_is_zero := f.api.Or(f.api.IsZero(mantissa), y.is_abnormal)

	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		mantissa_is_zero,
		exponent,
		f.api.Or(x.is_abnormal, y_is_zero),
	)

	return FloatVar{
		sign:     sign,
		exponent: exponent,
		// If the mantissa before fixing overflow is zero, we reset the final mantissa to 0,
		// as `Self::fix_overflow` incorrectly sets NaN's mantissa to infinity's mantissa.
		mantissa: f.api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		is_abnormal: is_abnormal,
	}
}

func (f *Context) less(x, y FloatVar, allow_eq uint) frontend.Variable {
	xe_ge_ye := f.gadget.IsPositive(f.api.Sub(x.exponent, y.exponent), f.E+1)
	xm_ge_ym := f.gadget.IsPositive(f.api.Sub(x.mantissa, y.mantissa), f.M+1)

	b := f.api.Select(
		f.api.Or(
			f.api.And(x.is_abnormal, f.api.IsZero(x.mantissa)),
			f.api.And(y.is_abnormal, f.api.IsZero(y.mantissa)),
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
		f.api.Select(
			f.gadget.IsEq(x.sign, y.sign),
			f.api.Select(
				f.gadget.IsEq(x.exponent, y.exponent),
				f.api.Select(
					f.gadget.IsEq(x.mantissa, y.mantissa),
					allow_eq,
					f.api.Select(
						x.sign,
						xm_ge_ym,
						f.api.Sub(big.NewInt(1), xm_ge_ym),
					),
				),
				f.api.Select(
					x.sign,
					xe_ge_ye,
					f.api.Sub(big.NewInt(1), xe_ge_ye),
				),
			),
			f.api.Select(
				f.api.IsZero(f.api.Add(x.mantissa, y.mantissa)),
				allow_eq,
				x.sign,
			),
		),
	)
	f.api.Compiler().MarkBoolean(b)
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
