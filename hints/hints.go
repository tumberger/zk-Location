package hints

import (
	"fmt"
	"math"
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
)

func init() {
	solver.RegisterHint(GetHints()...)
}

func GetHints() []solver.Hint {
	return []solver.Hint{
		LeftShiftHint,
		RightShiftHint,
		ModuloHint,
		Lookup32Hint,
		MSNZBHint,
		FloorHint,
		ExtractIntHint,
	}
}

func LeftShiftHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 2 {
		return fmt.Errorf("left shift hint expects 2 operands")
	}

	shift := inputs[1].Uint64()
	results[0].Lsh(inputs[0], uint(shift))
	println("Left shift result is", results[0].String())

	return nil
}

// Right shift hint for unsigned inputs
func RightShiftHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 3 {
		return fmt.Errorf("right shift hint expects 3 operands")
	}

	shift := inputs[2].Uint64()
	negative := inputs[0].Uint64()
	if uint(negative) != 0 {
		results[0].Neg(new(big.Int).Rsh(new(big.Int).Neg(inputs[1]), uint(shift)))
	} else {
		results[0].Rsh(inputs[1], uint(shift))
	}
	println("Right shift result is", results[0].String())

	return nil
}

func Lookup32Hint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 1 {
		return fmt.Errorf("lookup hint expects only 1 operand")
	}
	// Values for 32-bit (g = 7)
	lookup := [128]uint64{256, 254, 252, 250, 248, 246, 244, 242, 240, 239, 237, 235, 234, 232, 230, 229,
		227, 225, 224, 222, 221, 219, 218, 217, 215, 214, 212, 211, 210, 208, 207, 206,
		204, 203, 202, 201, 199, 198, 197, 196, 195, 193, 192, 191, 190, 189, 188, 187,
		186, 185, 184, 183, 182, 181, 180, 179, 178, 177, 176, 175, 174, 173, 172, 171,
		170, 169, 168, 168, 167, 166, 165, 164, 163, 163, 162, 161, 160, 159, 159, 158,
		157, 156, 156, 155, 154, 153, 153, 152, 151, 151, 150, 149, 148, 148, 147, 146,
		146, 145, 144, 144, 143, 143, 142, 141, 141, 140, 140, 139, 138, 138, 137, 137,
		136, 135, 135, 134, 134, 133, 133, 132, 132, 131, 131, 130, 130, 129, 129, 128}

	index := inputs[0].Uint64()
	results[0] = new(big.Int).SetUint64(lookup[uint(index)])
	println("Lookup result is", results[0].String())

	return nil
}

func ModuloHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 2 {
		return fmt.Errorf("modulo operation expects 2 operands")
	}

	remainder := new(big.Int).Rem(inputs[0], inputs[1])

	//println("Remainder is", remainder.String())

	results[0] = remainder

	return nil
}

func MSNZBHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 1 {
		return fmt.Errorf("MSNZB hint expects only 1 operand")
	}

	msnzb := inputs[0].BitLen()
	res := uint64(msnzb)
	if res > 0 {
		res = res - 1
	}
	results[0] = new(big.Int).SetUint64(uint64(res))
	fmt.Println("MSNZB result is: ", results[0].String())

	return nil
}

func FloorHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 4 {
		return fmt.Errorf("floor hint expects 4 operands")
	}
	fmt.Println("Exponent is: ", inputs[2].String())
	k := uint(inputs[0].Uint64())
	p := uint(inputs[1].Uint64())
	e := int(inputs[2].Int64()) - (int(math.Pow(2, float64(k-1))) - 1) // bring back exponent
	mantissa := uint(inputs[3].Uint64())
	println(e)
	if e >= 0 {
		shift := p - uint(e)
		println(shift)
		// all 1's for number of e most sig bits:   11..e..11 | 00...00
		mask := (uint(math.Pow(2, float64(e+1)) - 1)) << shift
		//mask := (uint(math.Pow(2,float64(p+1))) - 1) & (uint(math.Pow(2,float64(shift))))
		println("Mask:", mask)
		results[0] = new(big.Int).SetUint64(uint64(mantissa & mask))
	} else {
		results[0] = new(big.Int).SetUint64(0) //uint64(math.Pow(2,float64(p))))
	}

	fmt.Println("Masking result is: ", results[0].String())

	return nil
}

func ExtractIntHint(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	if len(inputs) != 4 {
		return fmt.Errorf("int extraction hint expects 4 operands")
	}

	k := uint(inputs[0].Uint64())
	p, acc := inputs[1].Float64()
	if acc < 0 {
		println("Do nothing.")
	}
	e, acc := inputs[2].Float64()
	if acc < 0 {
		println("Do nothing.")
	}
	e = e - (math.Pow(2, float64(k-1)) - 1)
	m, acc := inputs[3].Float64()
	if acc < 0 {
		println("Do nothing.")
	}

	normalized := m / math.Pow(2, p)
	results[0] = new(big.Int).SetUint64(uint64(math.Pow(2, e) * normalized))

	fmt.Println("Integer extraction result is: ", results[0].String())

	return nil
}
