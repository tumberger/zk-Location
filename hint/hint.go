package hint

import (
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
)

func init() {
	solver.RegisterHint(DecodeFloatHint)
	solver.RegisterHint(NthBitHint)
	solver.RegisterHint(PowerOfTwoHint)
	solver.RegisterHint(DecomposeMantissaForRoundingHint)
	solver.RegisterHint(NormalizeHint)
	solver.RegisterHint(AbsHint)
	solver.RegisterHint(DivHint)
	solver.RegisterHint(SqrtHint)
}

func DecodeFloatHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	v := inputs[0].Uint64()
	E := inputs[1].Uint64()
	M := inputs[2].Uint64()
	s := v >> (E + M)
	e := (v >> M) - (s << E)

	outputs[0].SetUint64(s)
	outputs[1].SetUint64(e)
	return nil
}

func NthBitHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	v := new(big.Int).Set(inputs[0])
	n := int(inputs[1].Uint64())

	outputs[0].SetUint64(uint64(v.Bit(n)))
	return nil
}

func PowerOfTwoHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	outputs[0].Lsh(big.NewInt(1), uint(inputs[0].Uint64()))
	return nil
}

func DecomposeMantissaForRoundingHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int,
) error {
	mantissa := new(big.Int).Set(inputs[0])
	two_to_shift := new(big.Int).Set(inputs[1])
	shift_max := uint(inputs[2].Uint64())
	p_idx := uint(inputs[3].Uint64())
	q_idx := uint(inputs[4].Uint64())
	r_idx := uint(inputs[5].Uint64())

	v := new(big.Int).Div(new(big.Int).Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), shift_max)), two_to_shift)

	outputs[0].Rsh(v, p_idx)
	outputs[1].SetUint64(uint64(v.Bit(int(q_idx))))
	outputs[2].SetUint64(uint64(v.Bit(int(r_idx))))
	outputs[3].And(v, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), r_idx), big.NewInt(1)))

	return nil
}

func NormalizeHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int,
) error {
	mantissa := new(big.Int).Set(inputs[0])
	mantissa_bit_length := uint64(inputs[1].Uint64())

	shift := uint64(0)
	for i := int(mantissa_bit_length - 1); i >= 0; i-- {
		if mantissa.Bit(i) != 0 {
			break
		}
		shift++
	}

	outputs[0].SetUint64(shift)

	return nil
}

func AbsHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int,
) error {
	mantissa := new(big.Int).Set(inputs[0])
	mantissa_ge_0 := mantissa.Cmp(new(big.Int).Rsh(new(big.Int).Set(field), 1)) < 0

	if mantissa_ge_0 {
		outputs[0].SetUint64(1)
		outputs[1].Set(mantissa)
	} else {
		outputs[0].SetUint64(0)
		outputs[1].Sub(new(big.Int).Set(field), mantissa)
	}

	return nil
}

func DivHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int,
) error {
	x := new(big.Int).Set(inputs[0])
	y := new(big.Int).Set(inputs[1])
	Q := uint(inputs[2].Uint64())

	outputs[0].Div(new(big.Int).Lsh(x, Q), y)

	return nil
}

func SqrtHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int,
) error {
	x := new(big.Int).Set(inputs[0])

	outputs[0].Sqrt(x)

	return nil
}
