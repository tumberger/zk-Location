package math

import (
	float "gnark-float/f64"
	utils "gnark-float/util"

	"math"

	"github.com/consensys/gnark/frontend"
)

// ToDo REFACTOR - usage of k and p should be implicit based on Float64/32
// needs to be handled better
// Exponent Bitwidth for float64
var k = 11
var p = 52

// ToDo REFACTOR - Fix Sign and Is_abnormal
// ToDo REFACTOR - Constant values as structs in util
func Atan2(f *float.Float, x, y float.F64) float.F64 {

	piE := utils.PiE
	piM := utils.PiM

	// TODO: Check if zero, do not divide by 0
	quotient := f.Div(x, y)

	sign := f.Api.Select(y.Sign, f.Api.Select(x.Sign, 0, 1), f.Api.Select(x.Sign, 1, 0))
	result := atanRemez(f, quotient)

	tmp := float.F64{
		Sign:        sign,
		Exponent:    result.Exponent,
		Mantissa:    result.Mantissa,
		Is_abnormal: 0,
	}

	tmpTwo := float.F64{
		Sign:        0,
		Exponent:    piE,
		Mantissa:    piM,
		Is_abnormal: 0,
	}

	addPi := f.Add(tmp, tmpTwo)
	tmpTwo.Sign = 1
	subPi := f.Add(tmp, tmpTwo)

	atan2S := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Sign, addPi.Sign), sign)
	atan2E := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Exponent, addPi.Exponent), result.Exponent)
	atan2M := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Mantissa, addPi.Mantissa), result.Mantissa)

	ret := float.F64{
		Sign:        atan2S,
		Exponent:    atan2E,
		Mantissa:    atan2M,
		Is_abnormal: 0,
	}

	return ret
}

// ToDo REFACTOR - Fix Sign and Is_abnormal
// ToDo REFACTOR - Constant values as structs in util
func atanRemez(f *float.Float, x float.F64) float.F64 {
	piM := utils.PiM
	halfPiE := utils.HalfPiE

	var coefficient = [33]int{
		0, -6, 11823596,
		1, -3, 8975490,
		0, -2, 11055062,
		1, -2, 12719124,
		0, -4, 14134724,
		0, -3, 11396472,
		0, -8, 11760836,
		1, -2, 11204846,
		0, -15, 9710032,
		0, -1, 16777200, 0, -28, 16446218}

	tmp := float.F64{
		Sign:        0,
		Exponent:    0,
		Mantissa:    utils.BaseM,
		Is_abnormal: 0,
	}
	greaterOne := f.GreaterThan(x, tmp)

	tmp = float.F64{
		Sign:        0,
		Exponent:    0,
		Mantissa:    int(math.Pow(2, float64(p))),
		Is_abnormal: 0,
	}

	recipical := f.Div(tmp, x)

	x.Exponent = f.Api.Select(greaterOne, recipical.Exponent, x.Exponent)
	x.Mantissa = f.Api.Select(greaterOne, recipical.Mantissa, x.Mantissa)

	u := float.F64{
		Sign:        coefficient[0],
		Exponent:    coefficient[1],
		Mantissa:    coefficient[2],
		Is_abnormal: 0,
	}

	for i := 3; i < 33; i += 3 {

		mult := f.Mul(u, x)

		tmp = float.F64{
			Sign:        coefficient[i],
			Exponent:    coefficient[i+1],
			Mantissa:    coefficient[i+2],
			Is_abnormal: 0,
		}

		tmpTwo := float.F64{
			Sign:        u.Sign,
			Exponent:    mult.Exponent,
			Mantissa:    mult.Mantissa,
			Is_abnormal: 0,
		}

		u = f.Add(tmpTwo, tmp)
	}

	sign := f.Api.Select(u.Sign, 0, 1)

	tmp = float.F64{
		Sign:        0,
		Exponent:    halfPiE,
		Mantissa:    piM,
		Is_abnormal: 0,
	}

	tmpTwo := float.F64{
		Sign:        sign,
		Exponent:    u.Exponent,
		Mantissa:    u.Mantissa,
		Is_abnormal: 0,
	}

	sub := f.Add(tmp, tmpTwo)

	e := f.Api.Select(greaterOne, sub.Exponent, sub.Exponent)
	m := f.Api.Select(greaterOne, sub.Mantissa, sub.Mantissa)

	return float.F64{
		Sign:        sign,
		Exponent:    e,
		Mantissa:    m,
		Is_abnormal: 0,
	}
}

var Pi = float.F64{
	Sign:        0,
	Exponent:    utils.PiE,
	Mantissa:    utils.PiM,
	Is_abnormal: 0,
}

var HalfPi = float.F64{
	Sign:        0,
	Exponent:    utils.HalfPiE,
	Mantissa:    utils.ThreeHalfPieM,
	Is_abnormal: 0,
}

func FloatSine(f *float.Float, x float.F64) float.F64 {

	var ret = float.F64{
		Sign:        0,
		Exponent:    -126,
		Mantissa:    utils.BaseM,
		Is_abnormal: 0,
	}

	// The Taylor Series approximation's inaccuracy increases when the input is close to pi
	// we mitigate this with the symmetry of the function
	// Since sin(x) is symmetric at pi/2, we fold across the symmetry axis in case term > pi/2
	check := f.GreaterThan(x, HalfPi)
	folding := f.Add(Pi, x)

	var term = float.F64{
		Sign:        0,
		Exponent:    f.Api.Select(check, folding.Exponent, x.Exponent),
		Mantissa:    f.Api.Select(check, folding.Mantissa, x.Mantissa),
		Is_abnormal: 0,
	}

	// Calculate term*x^2 / 2i*(2i+1) in each loop iteration
	xSquare := f.Mul(term, term)

	for i := 1; i < 11; i++ {

		if (i % 2) == 0 {
			term.Sign = f.Api.IsZero(term.Sign)
			ret = f.Add(ret, term)
		} else {
			ret = f.Add(ret, term)
		}

		nominator := f.Mul(term, xSquare)

		dnm := float.ToFloat64(float64(2 * i * (2*i + 1)))

		// ToDo - quick fix because ToFloat does not consider sign bit or un-normal numbers
		dnm.Sign = 0
		dnm.Is_abnormal = 0

		tmp := f.Div(nominator, dnm)

		term.Exponent = tmp.Exponent
		term.Mantissa = tmp.Mantissa
	}

	ret.Sign = x.Sign
	return ret
}

// ToDo - This currently uses the Newton Rhapson algorithm
// Shifting nth Root Algorithm for square root calculation can be better
// as NR algorithm requires one division per recursive approximation
// Can use lookups for the shifts as in float implementation (?)
// ToDo REFACTOR - Fix Sign and Is_abnormal
func SqRootFloatNewton(f *float.Float, term float.F64) float.F64 {
	var ret float.F64

	var x1 = float.F64{
		Sign:        0,
		Exponent:    frontend.Variable(1),
		Mantissa:    frontend.Variable(utils.BaseM),
		Is_abnormal: 0,
	}

	var x2 = float.F64{
		Sign:        0,
		Exponent:    1,
		Mantissa:    utils.BaseM,
		Is_abnormal: 0,
	}

	//TODO Optimize by bringing small numbers close to 1 and divide again by 10^
	// Calculate Square root approximation
	for i := 1; i < 13; i++ {

		summand1 := f.Div(x1, x2) // divide by 2
		tmp := f.Mul(x1, x2)      // multiply by 2
		summand2 := f.Div(term, tmp)

		addition := f.Add(summand1, summand2)

		x1.Exponent = addition.Exponent
		x1.Mantissa = addition.Mantissa
	}

	ret.Sign = 0
	ret.Exponent = x1.Exponent
	ret.Mantissa = x1.Mantissa
	ret.Is_abnormal = 0

	return ret
}
