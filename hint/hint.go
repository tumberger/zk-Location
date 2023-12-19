package hint

import (
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
)

func init() {
	solver.RegisterHint(DecodeFloatHint)
	solver.RegisterHint(ValueHint)
	solver.RegisterHint(NthBitHint)
	solver.RegisterHint(PowerOfTwoHint)
	solver.RegisterHint(DecomposeMantissaForRoundingHint)
	solver.RegisterHint(NormalizeHint)
	solver.RegisterHint(AbsHint)
	solver.RegisterHint(DivHint)
}

func DecodeFloatHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	v := inputs[0].Uint64()
	s := v >> 63
	e := (v >> 52) - (s << 11)

	outputs[0] = new(big.Int).SetUint64(s)
	outputs[1] = new(big.Int).SetUint64(e)
	return nil
}

func ValueHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	outputs[0] = new(big.Int).Set(inputs[0])
	return nil
}

func NthBitHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	v := inputs[0]
	n := int(inputs[1].Uint64())

	outputs[0] = new(big.Int).SetUint64(uint64(v.Bit(n)))
	return nil
}

func PowerOfTwoHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	outputs[0] = new(big.Int).Lsh(big.NewInt(1), uint(inputs[0].Uint64()))
	return nil
}

func DecomposeMantissaForRoundingHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int) error {
	mantissa := inputs[0]
	two_to_shift := inputs[1]
	shift_max := uint(inputs[2].Uint64())
	p_idx := uint(inputs[3].Uint64())
	q_idx := uint(inputs[4].Uint64())
	r_idx := uint(inputs[5].Uint64())

	v := new(big.Int).Div(new(big.Int).Mul(mantissa, new(big.Int).Lsh(big.NewInt(1), shift_max)), two_to_shift)

	p := new(big.Int).Rsh(v, p_idx)
	q := new(big.Int).SetUint64(uint64(v.Bit(int(q_idx))))
	r := new(big.Int).SetUint64(uint64(v.Bit(int(r_idx))))
	s := new(big.Int).And(v, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), r_idx), big.NewInt(1)))

	outputs[0] = p
	outputs[1] = q
	outputs[2] = r
	outputs[3] = s

	return nil
}

func NormalizeHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int) error {
	mantissa := inputs[0]
	mantissa_bit_length := uint64(inputs[1].Uint64())

	shift := uint64(0)
	for i := int(mantissa_bit_length - 1); i >= 0; i-- {
		if mantissa.Bit(i) != 0 {
			break
		}
		shift++
	}

	outputs[0] = new(big.Int).SetUint64(shift)
	outputs[1] = new(big.Int).Lsh(big.NewInt(1), uint(shift))

	return nil
}

func AbsHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int) error {
	mantissa := inputs[0]
	mantissa_ge_0 := mantissa.Cmp(new(big.Int).Rsh(field, 1)) < 0

	if mantissa_ge_0 {
		outputs[0].SetUint64(1)
		outputs[1] = mantissa
	} else {
		outputs[0].SetUint64(0)
		outputs[1] = new(big.Int).Sub(field, mantissa)
	}

	return nil
}

func DivHint(
	field *big.Int,
	inputs []*big.Int,
	outputs []*big.Int) error {

	x := inputs[0]
	y := inputs[1]
	Q := uint(inputs[2].Uint64())

	outputs[0] = new(big.Int).Div(new(big.Int).Lsh(x, Q), y)

	return nil
}
