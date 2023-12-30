package math

import (
	float "gnark-float/float"

	"fmt"
	"math"
)

// ToDo REFACTOR - Fix Sign and IsAbnormal
// ToDo REFACTOR - Constant values as structs in util
func Atan2(f *float.Context, x, y float.FloatVar) float.FloatVar {

	pi := f.NewF64Constant(math.Pi)

	// TODO: Check if zero, do not divide by 0
	quotient := f.Div(x, y)
	result := AtanRemez64(f, quotient)

	addPi := f.Add(result, pi)
	subPi := f.Sub(result, pi)

	// The result of Atan2 depends on the signs of x and y
	atan2S := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Sign, addPi.Sign), result.Sign)
	atan2E := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Exponent, addPi.Exponent), result.Exponent)
	atan2M := f.Api.Select(x.Sign, f.Api.Select(y.Sign, subPi.Mantissa, addPi.Mantissa), result.Mantissa)

	ret := float.FloatVar{
		Sign:       atan2S,
		Exponent:   atan2E,
		Mantissa:   atan2M,
		IsAbnormal: 0,
	}

	// TODO: Zero handling -- see below:
	// if x=0 AND y>0: atan2 = pi/2
	// if x=0 AND y<0: atan2 = -pi/2
	// if x=0 AND y=0: atan2 = undefined

	fmt.Printf("Result of atan2(): {%d, %d, %d}\n", ret.Sign, ret.Exponent, ret.Mantissa)

	return ret
}

func AtanRemez64(f *float.Context, x float.FloatVar) float.FloatVar {

	halfPi := f.NewF64Constant(math.Pi / 2.0)

	// We approximate the arctan(x) in the range [0,1] with a polynomial of degree 24,
	// the Remez algorithm has supplied us with the appropriate constants
	// (The lower the degree, the lower the accuracy, but also the less constraints!)
	var coefficient = [25]float.FloatVar{
		f.NewF64Constant(-0.000942885517390737),
		f.NewF64Constant(0.012831303689781028),
		f.NewF64Constant(-0.08114401696242823),
		f.NewF64Constant(0.31521931513648976),
		f.NewF64Constant(-0.8366759947462465),
		f.NewF64Constant(1.5941310579396186),
		f.NewF64Constant(-2.225620203806413),
		f.NewF64Constant(2.283041197386529),
		f.NewF64Constant(-1.716279012920493),
		f.NewF64Constant(0.9814600474792705),
		f.NewF64Constant(-0.5135300638813421),
		f.NewF64Constant(0.28006786416868995),
		f.NewF64Constant(-0.0649531804791716),
		f.NewF64Constant(-0.07417760886128402),
		f.NewF64Constant(-0.0034470515096669467),
		f.NewF64Constant(0.11167263969100766),
		f.NewF64Constant(-7.11657573061619e-05),
		f.NewF64Constant(-0.14285027929722588),
		f.NewF64Constant(-4.888898128412832e-07),
		f.NewF64Constant(0.2000000246846688),
		f.NewF64Constant(-8.33552299173139e-10),
		f.NewF64Constant(-0.3333333333160862),
		f.NewF64Constant(-1.8894178462249048e-13),
		f.NewF64Constant(1.0000000000000009),
		f.NewF64Constant(-4.1904552294565837e-19),
	}

	oneConst := f.NewF64Constant(float64(1))

	sign := x.Sign
	x.Sign = 0
	// TODO: Proper abnormal handling, check that x isn't 0 and act accordingly if x=0

	// Approximate atan by atan(x) = pi/2 - atan(1/x) if x>1
	greaterOne := f.IsGt(x, oneConst)
	reciprocal := f.Div(oneConst, x)

	x.Exponent = f.Api.Select(greaterOne, reciprocal.Exponent, x.Exponent)
	x.Mantissa = f.Api.Select(greaterOne, reciprocal.Mantissa, x.Mantissa)
	u := coefficient[0]

	for i := 1; i < 25; i++ {

		mult := f.Mul(u, x)
		u = f.Add(mult, coefficient[i])
	}

	sub := f.Sub(halfPi, u)

	resultE := f.Api.Select(greaterOne, sub.Exponent, u.Exponent)
	resultM := f.Api.Select(greaterOne, sub.Mantissa, u.Mantissa)
	res := float.FloatVar{
		Sign:       sign,
		Exponent:   resultE,
		Mantissa:   resultM,
		IsAbnormal: 0,
	}
	return res
}

// ToDo REFACTOR - Fix Sign and IsAbnormal
// ToDo REFACTOR - Constant values as structs in util
func AtanRemez32(f *float.Context, x float.FloatVar) float.FloatVar {

	halfPi := f.NewF32Constant(math.Pi / 2.0)

	// We approximate the arctan(x) in the range [0,1] with a polynomial of degree 10,
	// the Remez algorithm has supplied us with the appropriate constants
	var coefficient = [11]float.FloatVar{
		f.NewF32Constant(0.022023164),
		f.NewF32Constant(-0.13374522),
		f.NewF32Constant(0.32946652),
		f.NewF32Constant(-0.37905943),
		f.NewF32Constant(0.1053119),
		f.NewF32Constant(0.16982068),
		f.NewF32Constant(0.005476566),
		f.NewF32Constant(-0.33393043),
		f.NewF32Constant(0.000035324891),
		f.NewF32Constant(0.99999905),
		f.NewF32Constant(0.0000000073035884),
	}

	oneConst := f.NewF32Constant(float32(1))

	sign := x.Sign
	x.Sign = 0
	// TODO: Proper abnormal handling, check that x isn't 0 and act accordingly if x=0

	// Approximate atan by atan(x) = pi/2 - atan(1/x) if x>1
	greaterOne := f.IsGt(x, oneConst)
	reciprocal := f.Div(oneConst, x)

	x.Exponent = f.Api.Select(greaterOne, reciprocal.Exponent, x.Exponent)
	x.Mantissa = f.Api.Select(greaterOne, reciprocal.Mantissa, x.Mantissa)
	u := coefficient[0]

	for i := 1; i < 11; i++ {

		mult := f.Mul(u, x)
		u = f.Add(mult, coefficient[i])
	}

	sub := f.Sub(halfPi, u)

	resultE := f.Api.Select(greaterOne, sub.Exponent, u.Exponent)
	resultM := f.Api.Select(greaterOne, sub.Mantissa, u.Mantissa)
	res := float.FloatVar{
		Sign:       sign,
		Exponent:   resultE,
		Mantissa:   resultM,
		IsAbnormal: 0,
	}
	return res
}

func SinTaylor64(f *float.Context, x float.FloatVar) float.FloatVar {
	ret := f.NewF64Constant(float64(0))
	pi := f.NewF64Constant(math.Pi)
	halfPi := f.NewF64Constant(math.Pi / 2.0)

	// TODO: Assert x <= pi

	// TODO: Zero handling: Return 0 in case x = 0

	// The Taylor Series approximation's inaccuracy increases when the input is close to pi
	// we mitigate this with the symmetry of the function
	// Since sin(x) is symmetric at pi/2, we fold across the symmetry axis in case term > pi/2
	greaterHalfPi := f.IsGt(x, halfPi)
	folding := f.Sub(pi, x)

	var term = float.FloatVar{
		Sign:       0,
		Exponent:   f.Api.Select(greaterHalfPi, folding.Exponent, x.Exponent),
		Mantissa:   f.Api.Select(greaterHalfPi, folding.Mantissa, x.Mantissa),
		IsAbnormal: 0,
	}

	xSquare := f.Mul(term, term)
	// Calculate term*x^2 / 2i*(2i+1) in each loop iteration
	for i := 1; i <= 15; i++ {

		if (i % 2) == 0 {
			ret = f.Sub(ret, term)
		} else {
			ret = f.Add(ret, term)
		}

		nominator := f.Mul(term, xSquare)
		denominator := f.NewF64Constant(float64(2 * i * (2*i + 1)))

		term = f.Div(nominator, denominator)
	}

	ret.Sign = x.Sign
	return ret
}
