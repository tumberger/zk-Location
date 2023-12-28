package gadget

import (
	"gnark-float/hint"
	"gnark-float/logderivarg"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/rangecheck"
)

type PowersOfTwo struct {
	entries [][]frontend.Variable
	queries [][]frontend.Variable
}

func NewPowersOfTwoTable(api frontend.API, size uint) *PowersOfTwo {
	t := &PowersOfTwo{}

	t.entries = make([][]frontend.Variable, size)
	for i := uint(0); i < size; i++ {
		// The i-th entry is `i || 2^i`
		t.entries[i] = []frontend.Variable{new(big.Int).Add(
			new(big.Int).Lsh(big.NewInt(int64(i)), size),
			new(big.Int).Lsh(big.NewInt(1), i),
		)}
	}

	api.Compiler().Defer(t.commit)

	return t
}

func (t *PowersOfTwo) commit(api frontend.API) error {
	return logderivarg.Build(api, t.entries, t.queries)
}

type IntGadget struct {
	api          frontend.API
	rangechecker frontend.Rangechecker
	pow2         *PowersOfTwo
}

func New(api frontend.API, pow2_size uint) IntGadget {
	rangechecker := rangecheck.New(api)
	pow2 := NewPowersOfTwoTable(api, pow2_size)
	return IntGadget{api, rangechecker, pow2}
}

func (f *IntGadget) AssertBitLength(v frontend.Variable, bit_length uint) {
	f.rangechecker.Check(v, int(bit_length))
}

func (f *IntGadget) Abs(v frontend.Variable, length uint) (frontend.Variable, frontend.Variable) {
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

func (f *IntGadget) IsPositive(v frontend.Variable, length uint) frontend.Variable {
	_, is_positive := f.Abs(v, length)
	return is_positive
}

func (f *IntGadget) Max(a, b frontend.Variable, diff_length uint) frontend.Variable {
	return f.api.Select(
		f.IsPositive(f.api.Sub(a, b), diff_length),
		a,
		b,
	)
}

func (f *IntGadget) Min(a, b frontend.Variable, diff_length uint) frontend.Variable {
	return f.api.Select(
		f.IsPositive(f.api.Sub(a, b), diff_length),
		b,
		a,
	)
}

func (f *IntGadget) IsEq(a, b frontend.Variable) frontend.Variable {
	return f.api.IsZero(f.api.Sub(a, b))
}

func (f *IntGadget) QueryPowerOf2(exponent frontend.Variable) frontend.Variable {
	outputs, err := f.api.Compiler().NewHint(hint.PowerOfTwoHint, 1, exponent)
	if err != nil {
		panic(err)
	}
	result := outputs[0]
	// Make sure the result is small
	f.rangechecker.Check(result, len(f.pow2.entries))
	// Compute `exponent || result` and add it to the list of queries
	f.pow2.queries = append(f.pow2.queries, []frontend.Variable{f.api.Add(
		f.api.Mul(exponent, new(big.Int).Lsh(big.NewInt(1), uint(len(f.pow2.entries)))),
		result,
	)})
	return result
}
