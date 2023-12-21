package f64

import (
	"math"
	"math/big"

	"github.com/consensys/gnark/frontend"
	comparator "github.com/consensys/gnark/std/math/cmp"

	"gnark-float/gadget"
	"gnark-float/hint"
)

type F64 struct {
	Sign        frontend.Variable
	Exponent    frontend.Variable
	Mantissa    frontend.Variable
	Is_abnormal frontend.Variable
}

type Float struct {
	Api    frontend.API
	Gadget gadget.IntGadget
}

func (f *Float) NewF64(v frontend.Variable) F64 {
	outputs, err := f.Api.Compiler().NewHint(hint.DecodeFloatHint, 2, v)
	if err != nil {
		panic(err)
	}
	s := outputs[0]
	e := outputs[1]
	m := f.Api.Sub(v, f.Api.Add(f.Api.Mul(s, new(big.Int).Lsh(big.NewInt(1), 63)), f.Api.Mul(e, new(big.Int).Lsh(big.NewInt(1), 52))))

	f.Api.AssertIsBoolean(s)
	f.Gadget.AssertBitLength(e, 11)
	f.Gadget.AssertBitLength(m, 52)

	exponent := f.Api.Sub(e, big.NewInt(1023))

	mantissa_is_zero := f.Api.IsZero(m)
	mantissa_is_not_zero := f.Api.Sub(big.NewInt(1), mantissa_is_zero)
	f.Api.Compiler().MarkBoolean(mantissa_is_not_zero)
	exponent_is_min := f.Gadget.IsEq(exponent, big.NewInt(-1023))
	exponent_is_max := f.Gadget.IsEq(exponent, big.NewInt(1024))

	outputs, err = f.Api.Compiler().NewHint(hint.NormalizeHint, 2, m, big.NewInt(52))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce (shift, two_to_shift) is in lookup table [0, 52]

	shifted_mantissa := f.Api.Mul(m, two_to_shift)
	f.Gadget.AssertBitLength(
		f.Api.Sub(
			shifted_mantissa,
			f.Api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), 51)),
		),
		51,
	)

	exponent = f.Api.Select(
		exponent_is_min,
		f.Api.Sub(f.Api.Neg(shift), big.NewInt(1023)),
		exponent,
	)
	mantissa := f.Api.Select(
		exponent_is_min,
		f.Api.Add(shifted_mantissa, shifted_mantissa),
		f.Api.Select(
			f.Api.And(exponent_is_max, mantissa_is_not_zero),
			big.NewInt(0),
			f.Api.Add(m, new(big.Int).Lsh(big.NewInt(1), 52)),
		),
	)

	return F64{
		Sign:        s,
		Exponent:    exponent,
		Mantissa:    mantissa,
		Is_abnormal: exponent_is_max,
	}
}

func (f *Float) round(
	mantissa frontend.Variable,
	mantissa_bit_length uint64,
	shift frontend.Variable,
	shift_max uint64,
	half_flag frontend.Variable,
) frontend.Variable {
	outputs, err := f.Api.Compiler().NewHint(hint.PowerOfTwoHint, 1, shift)
	if err != nil {
		panic(err)
	}
	two_to_shift := outputs[0]
	// TODO: enforce (u, two_to_u) is in lookup table [0, u_max]

	r_idx := shift_max + mantissa_bit_length - 54
	q_idx := r_idx + 1
	p_idx := q_idx + 1
	p_len := uint64(52)
	s_len := r_idx

	outputs, err = f.Api.Compiler().NewHint(hint.DecomposeMantissaForRoundingHint, 4, mantissa, two_to_shift, shift_max, p_idx, q_idx, r_idx)
	if err != nil {
		panic(err)
	}

	p := outputs[0]
	q := outputs[1]
	r := outputs[2]
	s := outputs[3]

	f.Api.AssertIsBoolean(q)
	f.Api.AssertIsBoolean(r)
	f.Gadget.AssertBitLength(p, p_len)
	f.Gadget.AssertBitLength(s, s_len)

	qq := f.Api.Add(p, p, q)
	rr := f.Api.Add(f.Api.Mul(r, new(big.Int).Lsh(big.NewInt(1), uint(r_idx))), s)

	f.Api.AssertIsEqual(
		f.Api.Mul(f.Api.Add(f.Api.Mul(qq, new(big.Int).Lsh(big.NewInt(1), uint(q_idx))), rr), two_to_shift),
		f.Api.Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), uint(shift_max))),
	)

	is_half := f.Api.And(f.Gadget.IsEq(rr, new(big.Int).Lsh(big.NewInt(1), uint(r_idx))), half_flag)

	carry := f.Api.Select(is_half, q, r)

	return f.Api.Mul(f.Api.Add(qq, carry), two_to_shift)
}

func (f *Float) fixOverflow(
	mantissa frontend.Variable,
	exponent frontend.Variable,
	input_is_abnormal frontend.Variable,
) (frontend.Variable, frontend.Variable, frontend.Variable) {
	mantissa_overflow := f.Gadget.IsEq(mantissa, new(big.Int).Lsh(big.NewInt(1), 53))
	exponent = f.Api.Add(exponent, mantissa_overflow)
	Is_abnormal := f.Api.Or(f.Gadget.IsPositive(f.Api.Sub(exponent, big.NewInt(1024)), 12), input_is_abnormal)

	return f.Api.Select(
			f.Api.Or(mantissa_overflow, Is_abnormal),
			new(big.Int).Lsh(big.NewInt(1), 52),
			mantissa,
		), f.Api.Select(
			Is_abnormal,
			big.NewInt(1024),
			exponent,
		), Is_abnormal
}

func (f *Float) AssertIsEqual(x, y F64) {
	is_nan := f.Api.Or(
		f.Api.And(x.Is_abnormal, f.Api.IsZero(x.Mantissa)),
		f.Api.And(y.Is_abnormal, f.Api.IsZero(y.Mantissa)),
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
	f.Api.AssertIsEqual(x.Is_abnormal, y.Is_abnormal)
}

func (f *Float) Add(x, y F64) F64 {
	D := 55

	delta, ex_le_ey := f.Gadget.Abs(f.Api.Sub(y.Exponent, x.Exponent), 12)
	exponent := f.Api.Add(
		f.Api.Select(
			ex_le_ey,
			y.Exponent,
			x.Exponent,
		),
		big.NewInt(1),
	)
	delta = f.Gadget.Max(
		f.Api.Sub(D, delta),
		big.NewInt(0),
		11,
	)
	outputs, err := f.Api.NewHint(
		hint.PowerOfTwoHint,
		1,
		delta,
	)
	if err != nil {
		panic(err)
	}
	two_to_delta := outputs[0]
	// TODO: enforce (delta, two_to_delta) is in lookup table [0, >=55]

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
	zz := f.Api.Select(
		ex_le_ey,
		xx,
		yy,
	)
	ww := f.Api.Sub(f.Api.Add(xx, yy), zz)

	s := f.Api.Add(f.Api.Mul(zz, two_to_delta), f.Api.Mul(ww, new(big.Int).Lsh(big.NewInt(1), uint(D))))

	L := D + 53 + 1

	outputs, err = f.Api.Compiler().NewHint(hint.AbsHint, 2, s)
	if err != nil {
		panic(err)
	}
	mantissa_ge_0 := outputs[0]
	mantissa_abs := outputs[1]
	f.Api.AssertIsBoolean(mantissa_ge_0)
	mantissa_lt_0 := f.Api.Sub(big.NewInt(1), mantissa_ge_0)
	f.Api.Compiler().MarkBoolean(mantissa_lt_0)

	outputs, err = f.Api.Compiler().NewHint(hint.NormalizeHint, 2, mantissa_abs, big.NewInt(int64(L)))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce range of shift [0, 56]
	// TODO: enforce (shift, two_to_shift) is in lookup table

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
	f.Gadget.AssertBitLength(
		f.Api.Sub(mantissa, f.Api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))),
		uint64(L-1),
	)
	exponent = f.Api.Select(
		mantissa_is_zero,
		big.NewInt(-1075),
		f.Api.Sub(exponent, shift),
	)

	sign := f.Api.Select(
		f.Gadget.IsEq(x.Sign, y.Sign),
		x.Sign,
		mantissa_lt_0,
	)

	mantissa = f.round(
		mantissa,
		uint64(L),
		big.NewInt(0),
		0,
		1,
	)

	mantissa, exponent, Is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		f.Api.Or(x.Is_abnormal, y.Is_abnormal),
	)

	y_is_not_abnormal := f.Api.Sub(big.NewInt(1), y.Is_abnormal)
	f.Api.Compiler().MarkBoolean(y_is_not_abnormal)

	return F64{
		Sign:     sign,
		Exponent: exponent,
		Mantissa: f.Api.Select(
			x.Is_abnormal,
			f.Api.Select(
				f.Api.Or(
					y_is_not_abnormal,
					f.Gadget.IsEq(xx, yy),
				),
				x.Mantissa,
				big.NewInt(0),
			),
			f.Api.Select(
				y.Is_abnormal,
				y.Mantissa,
				mantissa,
			),
		),
		Is_abnormal: Is_abnormal,
	}
}

func (f *Float) Neg(x F64) F64 {
	neg_sign := f.Api.Sub(big.NewInt(1), x.Sign)
	f.Api.Compiler().MarkBoolean(neg_sign)
	return F64{
		Sign:        neg_sign,
		Exponent:    x.Exponent,
		Mantissa:    x.Mantissa,
		Is_abnormal: x.Is_abnormal,
	}
}

func (f *Float) Sub(x, y F64) F64 {
	return f.Add(x, f.Neg(y))
}

func (f *Float) Mul(x, y F64) F64 {
	sign := f.Api.Xor(x.Sign, y.Sign)
	mantissa := f.Api.Mul(x.Mantissa, y.Mantissa)
	L := 106
	outputs, err := f.Api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(L-1)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.Api.AssertIsBoolean(mantissa_msb)
	f.Gadget.AssertBitLength(
		f.Api.Sub(mantissa, f.Api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))),
		uint64(L-1),
	)
	mantissa = f.Api.Add(
		mantissa,
		f.Api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	exponent := f.Api.Add(f.Api.Add(x.Exponent, y.Exponent), mantissa_msb)

	U := 54
	mantissa = f.round(
		mantissa,
		uint64(L),
		f.Gadget.Max(
			f.Gadget.Min(
				f.Api.Sub(f.Api.Neg(exponent), big.NewInt(1022)),
				big.NewInt(int64(U)),
				12,
			),
			big.NewInt(0),
			12,
		),
		uint64(U),
		1,
	)

	mantissa_is_zero := f.Api.IsZero(mantissa)
	exponent = f.Api.Select(
		mantissa_is_zero,
		big.NewInt(-1075),
		exponent,
	)
	input_is_abnormal := f.Api.Or(x.Is_abnormal, y.Is_abnormal)
	mantissa, exponent, Is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		input_is_abnormal,
	)

	return F64{
		Sign:     sign,
		Exponent: exponent,
		Mantissa: f.Api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		Is_abnormal: Is_abnormal,
	}
}

func (f *Float) Div(x, y F64) F64 {
	Q := 54
	sign := f.Api.Xor(x.Sign, y.Sign)
	x_is_zero := f.Api.IsZero(x.Mantissa)
	y_is_zero := f.Api.IsZero(y.Mantissa)

	y_mantissa := f.Api.Select(
		y_is_zero,
		big.NewInt(1<<52),
		y.Mantissa,
	)
	outputs, err := f.Api.Compiler().NewHint(hint.DivHint, 1, x.Mantissa, y_mantissa, big.NewInt(int64(Q)))
	if err != nil {
		panic(err)
	}
	mantissa := outputs[0]
	outputs, err = f.Api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(Q)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.Api.AssertIsBoolean(mantissa_msb)
	flipped_mantissa_msb := f.Api.Sub(big.NewInt(1), mantissa_msb)
	f.Api.Compiler().MarkBoolean(flipped_mantissa_msb)
	L := Q + 1
	remainder := f.Api.Sub(f.Api.Mul(x.Mantissa, new(big.Int).Lsh(big.NewInt(1), uint(Q))), f.Api.Mul(mantissa, y_mantissa))
	f.Gadget.AssertBitLength(remainder, 53)
	f.Gadget.AssertBitLength(f.Api.Sub(y_mantissa, f.Api.Add(remainder, big.NewInt(1))), 53)
	f.Gadget.AssertBitLength(f.Api.Sub(mantissa, f.Api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))), uint64(L-1))

	mantissa = f.Api.Add(
		mantissa,
		f.Api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	exponent := f.Api.Sub(f.Api.Sub(x.Exponent, y.Exponent), flipped_mantissa_msb)

	U := 54
	mantissa = f.round(
		mantissa,
		uint64(L),
		f.Gadget.Max(
			f.Gadget.Min(
				f.Api.Sub(f.Api.Neg(exponent), big.NewInt(1022)),
				big.NewInt(int64(U)),
				12,
			),
			big.NewInt(0),
			12,
		),
		uint64(U),
		f.Api.IsZero(remainder),
	)

	mantissa, exponent, Is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		f.Api.Or(x.Is_abnormal, y_is_zero),
	)
	is_not_abnormal := f.Api.Sub(big.NewInt(1), Is_abnormal)
	f.Api.Compiler().MarkBoolean(is_not_abnormal)

	mantissa = f.Api.Select(
		f.Api.Or(x_is_zero, y.Is_abnormal),
		big.NewInt(0),
		mantissa,
	)
	mantissa_is_zero := f.Api.IsZero(mantissa)
	exponent = f.Api.Select(
		f.Api.And(mantissa_is_zero, is_not_abnormal),
		big.NewInt(-1075),
		exponent,
	)

	return F64{
		Sign:        sign,
		Exponent:    exponent,
		Mantissa:    mantissa,
		Is_abnormal: Is_abnormal,
	}
}

// Exponent Bitwidth for float64
var k = 11

func (f *Float) GreaterThan(floatOne F64, floatTwo F64) frontend.Variable {

	// Make exponents positive for correct comparison
	floatOne.Exponent = f.Api.Add(floatOne.Exponent, int(math.Pow(2, float64(k-1)))-1)
	floatTwo.Exponent = f.Api.Add(floatTwo.Exponent, int(math.Pow(2, float64(k-1)))-1)

	// Check if e1 > e2?
	eLT := f.Api.Sub(1, comparator.IsLess(f.Api, floatOne.Exponent, floatTwo.Exponent))
	// Check if e1 == e2?
	eEQ := f.Api.IsZero(f.Api.Sub(floatOne.Exponent, floatTwo.Exponent))
	// Check if x[1] > y[1]?
	mLT := f.Api.Sub(1, comparator.IsLess(f.Api, floatOne.Mantissa, floatTwo.Mantissa))

	eEQANDmLT := f.Api.Select(eEQ, mLT, 0)

	return f.Api.Select(eLT, f.Api.Sub(1, eEQANDmLT), eEQANDmLT)
}

func ToFloat64(in float64) F64 {

	// ToDo - Handle Sign Bit

	var ret F64

	f := new(big.Float).SetFloat64(in).SetPrec(23)
	m := new(big.Float)
	ret.Exponent = f.MantExp(m) - 1
	out, acc := m.Float32()
	if acc < 0 {
		println(out)
	}
	ret.Mantissa = int(math.Float32bits(out)&0x007FFFFF) + int(math.Pow(2, 23))

	return ret
}
