package f64

import (
	"math/big"

	"github.com/consensys/gnark/frontend"

	"gnark-float/gadget"
	"gnark-float/hint"
)

type F64 struct {
	sign        frontend.Variable
	exponent    frontend.Variable
	mantissa    frontend.Variable
	is_abnormal frontend.Variable
}

type Float struct {
	api    frontend.API
	gadget gadget.IntGadget
}

func (f *Float) NewF64(v frontend.Variable) F64 {
	outputs, err := f.api.Compiler().NewHint(hint.DecodeFloatHint, 2, v)
	if err != nil {
		panic(err)
	}
	s := outputs[0]
	e := outputs[1]
	m := f.api.Sub(v, f.api.Add(f.api.Mul(s, new(big.Int).Lsh(big.NewInt(1), 63)), f.api.Mul(e, new(big.Int).Lsh(big.NewInt(1), 52))))

	f.api.AssertIsBoolean(s)
	f.gadget.AssertBitLength(e, 11)
	f.gadget.AssertBitLength(m, 52)

	exponent := f.api.Sub(e, big.NewInt(1023))

	mantissa_is_zero := f.api.IsZero(m)
	mantissa_is_not_zero := f.api.Sub(big.NewInt(1), mantissa_is_zero)
	f.api.Compiler().MarkBoolean(mantissa_is_not_zero)
	exponent_is_min := f.gadget.IsEq(exponent, big.NewInt(-1023))
	exponent_is_max := f.gadget.IsEq(exponent, big.NewInt(1024))

	outputs, err = f.api.Compiler().NewHint(hint.NormalizeHint, 2, m, big.NewInt(52))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce (shift, two_to_shift) is in lookup table [0, 52]

	shifted_mantissa := f.api.Mul(m, two_to_shift)
	f.gadget.AssertBitLength(
		f.api.Sub(
			shifted_mantissa,
			f.api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), 51)),
		),
		51,
	)

	exponent = f.api.Select(
		exponent_is_min,
		f.api.Sub(f.api.Neg(shift), big.NewInt(1023)),
		exponent,
	)
	mantissa := f.api.Select(
		exponent_is_min,
		f.api.Add(shifted_mantissa, shifted_mantissa),
		f.api.Select(
			f.api.And(exponent_is_max, mantissa_is_not_zero),
			big.NewInt(0),
			f.api.Add(m, new(big.Int).Lsh(big.NewInt(1), 52)),
		),
	)

	return F64{
		sign:        s,
		exponent:    exponent,
		mantissa:    mantissa,
		is_abnormal: exponent_is_max,
	}
}

func (f *Float) round(
	mantissa frontend.Variable,
	mantissa_bit_length uint64,
	shift frontend.Variable,
	shift_max uint64,
	half_flag frontend.Variable,
) frontend.Variable {
	outputs, err := f.api.Compiler().NewHint(hint.PowerOfTwoHint, 1, shift)
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

	outputs, err = f.api.Compiler().NewHint(hint.DecomposeMantissaForRoundingHint, 4, mantissa, two_to_shift, shift_max, p_idx, q_idx, r_idx)
	if err != nil {
		panic(err)
	}

	p := outputs[0]
	q := outputs[1]
	r := outputs[2]
	s := outputs[3]

	f.api.AssertIsBoolean(q)
	f.api.AssertIsBoolean(r)
	f.gadget.AssertBitLength(p, p_len)
	f.gadget.AssertBitLength(s, s_len)

	qq := f.api.Add(p, p, q)
	rr := f.api.Add(f.api.Mul(r, new(big.Int).Lsh(big.NewInt(1), uint(r_idx))), s)

	f.api.AssertIsEqual(
		f.api.Mul(f.api.Add(f.api.Mul(qq, new(big.Int).Lsh(big.NewInt(1), uint(q_idx))), rr), two_to_shift),
		f.api.Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), uint(shift_max))),
	)

	is_half := f.api.And(f.gadget.IsEq(rr, new(big.Int).Lsh(big.NewInt(1), uint(r_idx))), half_flag)

	carry := f.api.Select(is_half, q, r)

	return f.api.Mul(f.api.Add(qq, carry), two_to_shift)
}

func (f *Float) fixOverflow(
	mantissa frontend.Variable,
	exponent frontend.Variable,
	input_is_abnormal frontend.Variable,
) (frontend.Variable, frontend.Variable, frontend.Variable) {
	mantissa_overflow := f.gadget.IsEq(mantissa, new(big.Int).Lsh(big.NewInt(1), 53))
	exponent = f.api.Add(exponent, mantissa_overflow)
	is_abnormal := f.api.Or(f.gadget.IsPositive(f.api.Sub(exponent, big.NewInt(1024)), 12), input_is_abnormal)

	return f.api.Select(
			f.api.Or(mantissa_overflow, is_abnormal),
			new(big.Int).Lsh(big.NewInt(1), 52),
			mantissa,
		), f.api.Select(
			is_abnormal,
			big.NewInt(1024),
			exponent,
		), is_abnormal
}

func (f *Float) AssertIsEqual(x, y F64) {
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

func (f *Float) Add(x, y F64) F64 {
	D := 55

	delta, ex_le_ey := f.gadget.Abs(f.api.Sub(y.exponent, x.exponent), 12)
	exponent := f.api.Add(
		f.api.Select(
			ex_le_ey,
			y.exponent,
			x.exponent,
		),
		big.NewInt(1),
	)
	delta = f.gadget.Max(
		f.api.Sub(D, delta),
		big.NewInt(0),
		11,
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
	// TODO: enforce (delta, two_to_delta) is in lookup table [0, >=55]

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
	zz := f.api.Select(
		ex_le_ey,
		xx,
		yy,
	)
	ww := f.api.Sub(f.api.Add(xx, yy), zz)

	s := f.api.Add(f.api.Mul(zz, two_to_delta), f.api.Mul(ww, new(big.Int).Lsh(big.NewInt(1), uint(D))))

	L := D + 53 + 1

	outputs, err = f.api.Compiler().NewHint(hint.AbsHint, 2, s)
	if err != nil {
		panic(err)
	}
	mantissa_ge_0 := outputs[0]
	mantissa_abs := outputs[1]
	f.api.AssertIsBoolean(mantissa_ge_0)
	mantissa_lt_0 := f.api.Sub(big.NewInt(1), mantissa_ge_0)
	f.api.Compiler().MarkBoolean(mantissa_lt_0)

	outputs, err = f.api.Compiler().NewHint(hint.NormalizeHint, 2, mantissa_abs, big.NewInt(int64(L)))
	if err != nil {
		panic(err)
	}
	shift := outputs[0]
	two_to_shift := outputs[1]
	// TODO: enforce range of shift [0, 56]
	// TODO: enforce (shift, two_to_shift) is in lookup table

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
	f.gadget.AssertBitLength(
		f.api.Sub(mantissa, f.api.Mul(mantissa_is_not_zero, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))),
		uint64(L-1),
	)
	exponent = f.api.Select(
		mantissa_is_zero,
		big.NewInt(-1075),
		f.api.Sub(exponent, shift),
	)

	sign := f.api.Select(
		f.gadget.IsEq(x.sign, y.sign),
		x.sign,
		mantissa_lt_0,
	)

	mantissa = f.round(
		mantissa,
		uint64(L),
		big.NewInt(0),
		0,
		1,
	)

	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		f.api.Or(x.is_abnormal, y.is_abnormal),
	)

	y_is_not_abnormal := f.api.Sub(big.NewInt(1), y.is_abnormal)
	f.api.Compiler().MarkBoolean(y_is_not_abnormal)

	return F64{
		sign:     sign,
		exponent: exponent,
		mantissa: f.api.Select(
			x.is_abnormal,
			f.api.Select(
				f.api.Or(
					y_is_not_abnormal,
					f.gadget.IsEq(xx, yy),
				),
				x.mantissa,
				big.NewInt(0),
			),
			f.api.Select(
				y.is_abnormal,
				y.mantissa,
				mantissa,
			),
		),
		is_abnormal: is_abnormal,
	}
}

func (f *Float) Neg(x F64) F64 {
	neg_sign := f.api.Sub(big.NewInt(1), x.sign)
	f.api.Compiler().MarkBoolean(neg_sign)
	return F64{
		sign:        neg_sign,
		exponent:    x.exponent,
		mantissa:    x.mantissa,
		is_abnormal: x.is_abnormal,
	}
}

func (f *Float) Sub(x, y F64) F64 {
	return f.Add(x, f.Neg(y))
}

func (f *Float) Mul(x, y F64) F64 {
	sign := f.api.Xor(x.sign, y.sign)
	mantissa := f.api.Mul(x.mantissa, y.mantissa)
	L := 106
	outputs, err := f.api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(L-1)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.api.AssertIsBoolean(mantissa_msb)
	f.gadget.AssertBitLength(
		f.api.Sub(mantissa, f.api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))),
		uint64(L-1),
	)
	mantissa = f.api.Add(
		mantissa,
		f.api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	exponent := f.api.Add(f.api.Add(x.exponent, y.exponent), mantissa_msb)

	U := 54
	mantissa = f.round(
		mantissa,
		uint64(L),
		f.gadget.Max(
			f.gadget.Min(
				f.api.Sub(f.api.Neg(exponent), big.NewInt(1022)),
				big.NewInt(int64(U)),
				12,
			),
			big.NewInt(0),
			12,
		),
		uint64(U),
		1,
	)

	mantissa_is_zero := f.api.IsZero(mantissa)
	exponent = f.api.Select(
		mantissa_is_zero,
		big.NewInt(-1075),
		exponent,
	)
	input_is_abnormal := f.api.Or(x.is_abnormal, y.is_abnormal)
	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		input_is_abnormal,
	)

	return F64{
		sign:     sign,
		exponent: exponent,
		mantissa: f.api.Select(
			mantissa_is_zero,
			big.NewInt(0),
			mantissa,
		),
		is_abnormal: is_abnormal,
	}
}

func (f *Float) Div(x, y F64) F64 {
	Q := 54
	sign := f.api.Xor(x.sign, y.sign)
	x_is_zero := f.api.IsZero(x.mantissa)
	y_is_zero := f.api.IsZero(y.mantissa)

	y_mantissa := f.api.Select(
		y_is_zero,
		big.NewInt(1<<52),
		y.mantissa,
	)
	outputs, err := f.api.Compiler().NewHint(hint.DivHint, 1, x.mantissa, y_mantissa, big.NewInt(int64(Q)))
	if err != nil {
		panic(err)
	}
	mantissa := outputs[0]
	outputs, err = f.api.Compiler().NewHint(hint.NthBitHint, 1, mantissa, big.NewInt(int64(Q)))
	if err != nil {
		panic(err)
	}
	mantissa_msb := outputs[0]
	f.api.AssertIsBoolean(mantissa_msb)
	flipped_mantissa_msb := f.api.Sub(big.NewInt(1), mantissa_msb)
	f.api.Compiler().MarkBoolean(flipped_mantissa_msb)
	L := Q + 1
	remainder := f.api.Sub(f.api.Mul(x.mantissa, new(big.Int).Lsh(big.NewInt(1), uint(Q))), f.api.Mul(mantissa, y_mantissa))
	f.gadget.AssertBitLength(remainder, 53)
	f.gadget.AssertBitLength(f.api.Sub(y_mantissa, f.api.Add(remainder, big.NewInt(1))), 53)
	f.gadget.AssertBitLength(f.api.Sub(mantissa, f.api.Mul(mantissa_msb, new(big.Int).Lsh(big.NewInt(1), uint(L-1)))), uint64(L-1))

	mantissa = f.api.Add(
		mantissa,
		f.api.Select(
			mantissa_msb,
			big.NewInt(0),
			mantissa,
		),
	)
	exponent := f.api.Sub(f.api.Sub(x.exponent, y.exponent), flipped_mantissa_msb)

	U := 54
	mantissa = f.round(
		mantissa,
		uint64(L),
		f.gadget.Max(
			f.gadget.Min(
				f.api.Sub(f.api.Neg(exponent), big.NewInt(1022)),
				big.NewInt(int64(U)),
				12,
			),
			big.NewInt(0),
			12,
		),
		uint64(U),
		f.api.IsZero(remainder),
	)

	mantissa, exponent, is_abnormal := f.fixOverflow(
		mantissa,
		exponent,
		f.api.Or(x.is_abnormal, y_is_zero),
	)
	is_not_abnormal := f.api.Sub(big.NewInt(1), is_abnormal)
	f.api.Compiler().MarkBoolean(is_not_abnormal)

	mantissa = f.api.Select(
		f.api.Or(x_is_zero, y.is_abnormal),
		big.NewInt(0),
		mantissa,
	)
	mantissa_is_zero := f.api.IsZero(mantissa)
	exponent = f.api.Select(
		f.api.And(mantissa_is_zero, is_not_abnormal),
		big.NewInt(-1075),
		exponent,
	)

	return F64{
		sign:        sign,
		exponent:    exponent,
		mantissa:    mantissa,
		is_abnormal: is_abnormal,
	}
}
