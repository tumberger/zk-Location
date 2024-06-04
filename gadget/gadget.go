package gadget

import (
	"gnark-float/hint"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/logderivarg"
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
	api               frontend.API
	rangechecker      *CommitChecker
	pow2              *PowersOfTwo
	range_size        uint
	pow2_size         uint
	num_range_decompositions uint
	num_range_chunks  uint
	num_pow2_queries  uint
}

func New(api frontend.API, range_size uint, pow2_size uint) *IntGadget {
	rangechecker := NewCommitRangechecker(api, int(range_size))
	pow2 := NewPowersOfTwoTable(api, pow2_size)
	return &IntGadget{api, rangechecker, pow2, range_size, pow2_size, 0, 0, 0}
}

func (f *IntGadget) LookupEntryConstraints() uint {
	return (1 << f.range_size) + f.pow2_size
}

func (f *IntGadget) LookupQueryConstraints() uint {
	return f.num_range_decompositions + f.num_pow2_queries + f.num_range_chunks
}

func (f *IntGadget) LookupFinalizeConstraints() uint {
	// 1 for rangecheck, 1 for pow2, 1 for multicommit (https://github.com/winderica/gnark/blob/17abec78e9610ecfe73d2dbf471550ac2c509785/std/multicommit/nativecommit.go#L100)
	return 3
}

func (f *IntGadget) AssertBitLength(v frontend.Variable, bit_length uint, mode Mode) {
	f.rangechecker.Check(v, int(bit_length), mode)
	if f.range_size > 0 {
		num_limbs := (bit_length + f.range_size - 1) / f.range_size
		if num_limbs != 1 {
			f.num_range_decompositions++
		}
		f.num_range_chunks += num_limbs
		if mode == TightForUnknownRange && bit_length % f.range_size != 0 {
			f.num_range_chunks++
		}
	}
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
	f.AssertBitLength(abs, length, Loose)
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
	f.AssertBitLength(result, uint(len(f.pow2.entries)), Loose)
	// Compute `exponent || result` and add it to the list of queries
	f.pow2.queries = append(f.pow2.queries, []frontend.Variable{f.api.Add(
		f.api.Mul(exponent, new(big.Int).Lsh(big.NewInt(1), uint(len(f.pow2.entries)))),
		result,
	)})
	f.num_pow2_queries++
	return result
}
