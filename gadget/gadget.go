package gadget

import (
	"gnark-float/hint"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/rangecheck"
)

type IntGadget struct {
	api          frontend.API
	rangechecker frontend.Rangechecker
}

func New(api frontend.API) IntGadget {
	rangechecker := rangecheck.New(api)
	return IntGadget{api, rangechecker}
}

func (f *IntGadget) AssertBitLength(v frontend.Variable, bit_length uint64) {
	// TODO
	// f.rangechecker.Check(v, int(bit_length))
}

func (f *IntGadget) Abs(v frontend.Variable, length uint64) (frontend.Variable, frontend.Variable) {
	outputs, err := f.api.Compiler().NewHint(hint.AbsHint, 2, v)
	if err != nil {
		panic(err)
	}
	is_positive := outputs[0]
	f.api.AssertIsBoolean(is_positive)
	abs := f.api.Select(
		is_positive,
		v,
		f.api.Neg(v),
	)
	f.AssertBitLength(abs, length)
	return abs, is_positive
}

func (f *IntGadget) IsPositive(v frontend.Variable, length uint64) frontend.Variable {
	_, is_positive := f.Abs(v, length)
	return is_positive
}

func (f *IntGadget) Max(a, b frontend.Variable, diff_length uint64) frontend.Variable {
	return f.api.Select(
		f.IsPositive(f.api.Sub(a, b), diff_length),
		a,
		b,
	)
}

func (f *IntGadget) Min(a, b frontend.Variable, diff_length uint64) frontend.Variable {
	return f.api.Select(
		f.IsPositive(f.api.Sub(a, b), diff_length),
		b,
		a,
	)
}
