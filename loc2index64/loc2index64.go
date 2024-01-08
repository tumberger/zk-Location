package loc2index64

import (
	float "gnark-float/float"
	maths "gnark-float/math"
	util "gnark-float/util"

	"math"
	//"fmt"

	"github.com/consensys/gnark/frontend"
	comparator "github.com/consensys/gnark/std/math/cmp"
)

type loc2Index64Wrapper struct {

	// SECRET INPUTS
	LatS frontend.Variable
	LatE frontend.Variable
	LatM frontend.Variable
	LatA frontend.Variable
	LngS frontend.Variable
	LngE frontend.Variable
	LngM frontend.Variable
	LngA frontend.Variable

	// PUBLIC INPUTS
	Resolution frontend.Variable `gnark:",public"`
	I          frontend.Variable `gnark:",public"`
	J          frontend.Variable `gnark:",public"`
	K          frontend.Variable `gnark:",public"`
}

// TODO: Comments
func (circuit *loc2Index64Wrapper) Define(api frontend.API) error {

	api.AssertIsEqual(circuit.LatA, 0)
	api.AssertIsEqual(circuit.LngA, 0)

	f := float.NewContext(api, util.IEEE64ExponentBitwidth, util.IEEE64Precision)
	lat := float.FloatVar{
		Sign:       circuit.LatS,
		Exponent:   circuit.LatE,
		Mantissa:   circuit.LatM,
		IsAbnormal: circuit.LatA,
	}
	lng := float.FloatVar{
		Sign:       circuit.LngS,
		Exponent:   circuit.LngE,
		Mantissa:   circuit.LngM,
		IsAbnormal: circuit.LngA,
	}
	resolution := circuit.Resolution
	pi := f.NewF64Constant(math.Pi)
	halfPi := f.NewF64Constant(math.Pi / 2.0)
	doublePi := f.NewF64Constant(math.Pi * 2.0)

	// Lat can't be more than pi/2, Lng can't be more than pi and max resolution is 15
	api.AssertIsEqual(f.IsGt(lat, halfPi), 0)
	api.AssertIsEqual(f.IsGt(lng, pi), 0)
	api.AssertIsLessOrEqual(resolution, util.MaxResolution)

	// We calculate the 3d vector (x,y,z), starting with x

	// Adding half pi to latitude to apply cos() -- lat always in range [-pi/2, pi/2]
	term := f.Add(lat, halfPi)
	cosLat := maths.SinTaylor64(&f, term)

	// Adding half pi to longitude to apply cos() -- lng always in range [-pi, pi]
	tmp := f.Add(lng, halfPi)
	// TODO: If it makes no big difference in regards to constraints: (input % 2pi) - pi
	// can be applied on the input at the start of SinTaylor and the next lines can be deleted
	isGreater := f.IsGt(tmp, pi)
	shifted := f.Sub(tmp, doublePi)
	term.Sign = api.Select(isGreater, shifted.Sign, tmp.Sign)
	term.Exponent = api.Select(isGreater, shifted.Exponent, tmp.Exponent)
	term.Mantissa = api.Select(isGreater, shifted.Mantissa, tmp.Mantissa)
	term.IsAbnormal = 0

	cosLng := maths.SinTaylor64(&f, term)
	x := f.Mul(cosLat, cosLng)

	sinLng := maths.SinTaylor64(&f, lng)
	y := f.Mul(cosLat, sinLng)

	z := maths.SinTaylor64(&f, lat)

	calc := closestFaceCalculations(&f, x, y, z, lng)
	sqDistance := calc[0]

	theta := calculateTheta(&f, z, cosLat, calc[1], calc[2], calc[3], calc[4], resolution)
	r := calculateR(&f, sqDistance, resolution)
	hex2d := calculateHex2d(&f, theta, r)

	ijk := hex2dToCoordIJK(&f, hex2d[0], hex2d[1])

	api.AssertIsEqual(circuit.I, ijk[0])
	api.AssertIsEqual(circuit.J, ijk[1])
	api.AssertIsEqual(circuit.K, ijk[2])

	return nil
}

func scaleR(f *float.Context, r float.FloatVar, resolution frontend.Variable) float.FloatVar {
	multiplier := f.NewF64Constant(1.0)
	power := util.Sqrt7
	// `0 <= resolution <= 15` tightly fits into 4 bits
	bits := f.Api.ToBinary(resolution, 4)
	// The square and multiply algorithm
	for _, bit := range bits {
		t := f.Mul(multiplier, f.NewF64Constant(power))
		multiplier = float.FloatVar{
			Sign:       0,
			Exponent:   f.Api.Select(bit, t.Exponent, multiplier.Exponent),
			Mantissa:   f.Api.Select(bit, t.Mantissa, multiplier.Mantissa),
			IsAbnormal: 0,
		}
		power *= power
	}

	return f.Mul(r, multiplier)
}

// Calculating r: In the original code the variable r is calculated from the square distance
// by a series of computations, with the first step being "r = acos(1 - sqd/2)" and the second
// step "r = tan(r)". Since acos(x) = atan(sqrt((1-x)^2) / x) the first two steps can be
// summarized to r = tan( acos(1 - sqd/2) ) = tan( atan( sqrt( (1-(1-sqd/2))^2 ) / (1-sqd/2) ))
// = sqrt( (1-(1-sqd/2))^2 ) / (1-sqd/2) = sqrt( (-1)*(sqd-4)*sqd ) / (2 - sqd)
func calculateR(f *float.Context, sqDist float.FloatVar, resolution frontend.Variable) float.FloatVar {

	// Nominator = (4 - sqDist) * sqDist  ---  Divisor = 2 - sqDist
	nominator := f.Mul(f.Sub(f.NewF64Constant(4.0), sqDist), sqDist)
	divisor := f.Sub(f.NewF64Constant(2.0), sqDist)
	sqrNom := f.Sqrt(nominator)
	quotient := f.Div(sqrNom, divisor)

	r := f.Div(quotient, f.NewF64Constant(util.ResConst))

	return scaleR(f, r, resolution)
}

func closestFaceCalculations(f *float.Context, x2, y2, z2, lng float.FloatVar) [5]float.FloatVar {

	var ret [5]float.FloatVar

	// Starting with square distance 5
	sqDist := f.NewF64Constant(5.0)
	cosFaceLat := f.NewF64Constant(0)
	sinFaceLat := f.NewF64Constant(0)
	azimuth := f.NewF64Constant(0)
	lngDiff := f.NewF64Constant(0)

	// We determine the face which has the smallest square distance from its center point to
	// our lat,lng coordinates and set all variables which depend on the face for later use
	for i := 0; i < 60; i += 3 {

		d := f.Sub(f.NewF64Constant(util.FaceCenterPoint[i]), x2)
		s1 := f.Mul(d, d)

		d = f.Sub(f.NewF64Constant(util.FaceCenterPoint[i+1]), y2)
		s2 := f.Mul(d, d)

		d = f.Sub(f.NewF64Constant(util.FaceCenterPoint[i+2]), z2)
		s3 := f.Mul(d, d)

		tmp := f.Add(s1, s2)
		dist := f.Add(tmp, s3)

		check := f.IsGt(sqDist, dist)

		face := i / 3
		currCFL := f.NewF64Constant(util.CosFaceLat[face])
		currSFL := f.NewF64Constant(util.SinFaceLat[face])
		currAz := f.NewF64Constant(util.Azimuth[face])
		currFaceLng := f.NewF64Constant(util.FaceCenterGeoLng[face])

		// Set values accordingly if square distance is new lowest value
		// variables sqDist, cosFaceLat and azimuth are always positive
		sqDist.Exponent = f.Api.Select(check, dist.Exponent, sqDist.Exponent)
		sqDist.Mantissa = f.Api.Select(check, dist.Mantissa, sqDist.Mantissa)
		cosFaceLat.Exponent = f.Api.Select(check, currCFL.Exponent, cosFaceLat.Exponent)
		cosFaceLat.Mantissa = f.Api.Select(check, currCFL.Mantissa, cosFaceLat.Mantissa)
		sinFaceLat.Sign = f.Api.Select(check, currSFL.Sign, sinFaceLat.Sign)
		sinFaceLat.Exponent = f.Api.Select(check, currSFL.Exponent, sinFaceLat.Exponent)
		sinFaceLat.Mantissa = f.Api.Select(check, currSFL.Mantissa, sinFaceLat.Mantissa)
		azimuth.Exponent = f.Api.Select(check, currAz.Exponent, azimuth.Exponent)
		azimuth.Mantissa = f.Api.Select(check, currAz.Mantissa, azimuth.Mantissa)

		tmpDiff := f.Sub(lng, currFaceLng)
		lngDiff.Sign = f.Api.Select(check, tmpDiff.Sign, lngDiff.Sign)
		lngDiff.Exponent = f.Api.Select(check, tmpDiff.Exponent, lngDiff.Exponent)
		lngDiff.Mantissa = f.Api.Select(check, tmpDiff.Mantissa, lngDiff.Mantissa)
	}

	ret[0] = sqDist
	ret[1] = cosFaceLat
	ret[2] = sinFaceLat
	ret[3] = azimuth
	ret[4] = lngDiff

	return ret
}

func calculateInputsToAtan2(f *float.Context, z, cosLat, cosFaceLat, sinFaceLat, lngDiff float.FloatVar) [2]float.FloatVar {

	var ret [2]float.FloatVar
	pi := f.NewF64Constant(math.Pi)
	halfPi := f.NewF64Constant(math.Pi / 2.0)
	doublePi := f.NewF64Constant(math.Pi * 2.0)

	// Adjustments for sin() function
	// TODO: If it makes no big difference in regards to constraints: (input % 2pi) - pi
	// can be applied on the input at the start of SinTaylor and the next lines can be deleted

	// lngDiff in range [-2pi, 2pi], convert to abs(lngDiff) and adjust for sin() function
	sign := lngDiff.Sign
	lngDiff.Sign = frontend.Variable(0)
	term := f.Sub(lngDiff, pi)
	term.Sign = f.Api.Select(sign, term.Sign, f.Neg(term).Sign) // symmetry of sin()
	sinLngDiff := maths.SinTaylor64(f, term)

	// Adjustments to abs(lngDiff) for cos() function
	// First we add pi/2 for cos adjustment and then subtract 2pi if we're out of range
	cosArg := f.Add(lngDiff, halfPi)
	isGreater := f.IsGt(cosArg, pi)
	shifted := f.Sub(cosArg, doublePi)
	term.Sign = f.Api.Select(isGreater, shifted.Sign, cosArg.Sign)
	term.Exponent = f.Api.Select(isGreater, shifted.Exponent, cosArg.Exponent)
	term.Mantissa = f.Api.Select(isGreater, shifted.Mantissa, cosArg.Mantissa)
	cosLngDiff := maths.SinTaylor64(f, term)

	arg1 := f.Mul(sinLngDiff, cosLat)
	// arg2 is set up of two summands which we determine separately
	arg2Part1 := f.Mul(z, cosFaceLat)
	tmp := f.Mul(sinFaceLat, cosLat)
	arg2Part2 := f.Mul(tmp, cosLngDiff)
	arg2 := f.Sub(arg2Part1, arg2Part2)

	ret[0] = arg1
	ret[1] = arg2

	return ret
}

// This function brings input into the range [0,2pi] if input is outside
// by adding or subtracting 2pi depending on which "side" of the range the input is
func posAngleRads(f *float.Context, rads float.FloatVar) float.FloatVar {

	doublePi := f.NewF64Constant(math.Pi * 2.0)

	increase := f.Add(rads, doublePi)
	tmp := float.FloatVar{
		Sign:       0,
		Exponent:   f.Api.Select(rads.Sign, increase.Exponent, rads.Exponent),
		Mantissa:   f.Api.Select(rads.Sign, increase.Mantissa, rads.Mantissa),
		IsAbnormal: 0,
	}
	check := f.IsGt(tmp, doublePi)
	decrease := f.Sub(tmp, doublePi)

	return float.FloatVar{
		Sign:       0,
		Exponent:   f.Api.Select(check, decrease.Exponent, tmp.Exponent),
		Mantissa:   f.Api.Select(check, decrease.Mantissa, tmp.Mantissa),
		IsAbnormal: 0,
	}
}

func calculateTheta(f *float.Context, z, cosLat, cosFaceLat, sinFaceLat, azimuth, lngDiff float.FloatVar, resolution frontend.Variable) float.FloatVar {

	// Calculate atan2
	args := calculateInputsToAtan2(f, z, cosLat, cosFaceLat, sinFaceLat, lngDiff)
	atan2 := maths.Atan2(f, args[0], args[1])

	// Applying posAngleRads to bring in range [0,2pi] and then subtracting from azimuth value
	sub := posAngleRads(f, atan2)
	diff := f.Sub(azimuth, sub)
	theta := posAngleRads(f, diff)

	// Apply rotation in case of odd resolution
	tmp := f.Sub(theta, f.NewF64Constant(util.Ap7rot))
	thetaOdd := posAngleRads(f, tmp)

	// `0 <= resolution <= 15` tightly fits into 4 bits
	lsb := f.Api.ToBinary(resolution, 4)[0]

	return float.FloatVar{
		Sign:       0,
		Exponent:   f.Api.Select(lsb, thetaOdd.Exponent, theta.Exponent),
		Mantissa:   f.Api.Select(lsb, thetaOdd.Mantissa, theta.Mantissa),
		IsAbnormal: 0,
	}
}

func calculateHex2d(f *float.Context, theta, r float.FloatVar) [2]float.FloatVar {

	var ret [2]float.FloatVar
	pi := f.NewF64Constant(math.Pi)
	halfPi := f.NewF64Constant(math.Pi / 2.0)
	oneAndHalfPi := f.NewF64Constant(math.Pi * 1.5)
	doublePi := f.NewF64Constant(math.Pi * 2.0)

	// Adjustments for sin() function -- theta is in range [0, 2pi]
	// shift and flip sign because of symmetry of sin()
	term := f.Neg(f.Sub(theta, pi))
	sinTheta := maths.SinTaylor64(f, term)

	// Adjustments for cos() function
	// shift to left by 1.5 * pi
	// if in range [0, 0.5 * pi] before shift, shift first quarter to the right by 2 * pi
	isLessHalfPi := f.IsLt(theta, halfPi)
	shifted1 := f.Sub(theta, oneAndHalfPi)
	shifted2 := f.Add(shifted1, doublePi)
	term = float.FloatVar{
		Sign:       f.Api.Select(isLessHalfPi, shifted2.Sign, shifted1.Sign),
		Exponent:   f.Api.Select(isLessHalfPi, shifted2.Exponent, shifted1.Exponent),
		Mantissa:   f.Api.Select(isLessHalfPi, shifted2.Mantissa, shifted1.Mantissa),
		IsAbnormal: 0,
	}
	cosTheta := maths.SinTaylor64(f, term)

	ret[0] = f.Mul(cosTheta, r) // x
	ret[1] = f.Mul(sinTheta, r) // y

	return ret
}

// TODO: Comments
func hex2dToCoordIJK(f *float.Context, x, y float.FloatVar) [3]frontend.Variable {

	// Take absolute values of x and y, then put them back to original
	a1 := x
	a1.Sign = frontend.Variable(0)
	a2 := y
	a2.Sign = frontend.Variable(0)
	x2 := f.Div(a2, f.NewF64Constant(util.Sin60))
	tmp := f.Div(x2, f.NewF64Constant(2.0))
	x1 := f.Add(a1, tmp)

	m1 := f.Floor(x1)
	m2 := f.Floor(x2)
	m1int := f.ToInt(m1)
	m2int := f.ToInt(m2)

	r1 := f.Sub(x1, m1)
	r2 := f.Sub(x2, m2)

	doubleR1 := f.Add(r1, r1)
	m1PlusOne := f.Api.Add(m1int, 1)
	m2PlusOne := f.Api.Add(m2int, 1)

	// Check if r1 < 1/2?
	r1CaseA := f.IsLt(r1, f.NewF64Constant(0.5))
	// Check if r1 < 1/3?
	r1CaseA1 := f.IsLt(r1, f.NewF64Constant((1.0 / 3.0)))
	// Check if r1 < 2/3?
	r1CaseB1 := f.IsLt(r1, f.NewF64Constant((2.0 / 3.0)))
	// Check if 1-r1 <= r2?
	oneMinus := f.Sub(f.NewF64Constant(1.0), r1)
	iCaseA2First := f.IsLe(oneMinus, r2)
	// Check if 2*r1 > r2?
	iCaseA2Second := f.IsGt(doubleR1, r2)
	// Check if r2 > 2*r1-1?
	doubleOneMinus := f.Sub(doubleR1, f.NewF64Constant(1.0))
	iCaseB1First := f.IsGt(r2, doubleOneMinus)
	// Check if 1-r1 > r2?
	iCaseB1Second := f.IsGt(oneMinus, r2)

	// First get I
	iCoord := f.Api.Select(r1CaseA,
		f.Api.Select(r1CaseA1, m1int,
			f.Api.Select(iCaseA2First, f.Api.Select(iCaseA2Second, m1PlusOne, m1int), m1int)),
		f.Api.Select(r1CaseB1,
			f.Api.Select(iCaseB1First,
				f.Api.Select(iCaseB1Second, m1int, m1PlusOne), m1PlusOne), m1PlusOne))

	// Next is J
	onePlus := f.Add(r1, f.NewF64Constant(1.0))
	valueR2PathA := f.Div(onePlus, f.NewF64Constant(2.0))
	valueR2PathB := f.Div(r1, f.NewF64Constant(2.0))
	check1 := f.IsGt(valueR2PathA, r2)
	check2 := f.IsGt(oneMinus, r2)
	check3 := f.IsGt(valueR2PathB, r2)

	caseAjCoord := f.Api.Select(r1CaseA1, f.Api.Select(check1, m2int, m2PlusOne),
		f.Api.Select(check2, m2int, m2PlusOne))
	caseBjCoord := f.Api.Select(r1CaseB1, f.Api.Select(check2, m2int, m2PlusOne),
		f.Api.Select(check3, m2int, m2PlusOne))
	jCoord := f.Api.Select(r1CaseA, caseAjCoord, caseBjCoord)

	iGreater := comparator.IsLess(f.Api, jCoord, iCoord)
	// In case only x is negative: i = -i + j
	// in case only y is negative: i = i - j
	// in case x AND y negative: i = -i
	iCoordNegative := f.Api.Select(x.Sign,
		f.Api.Select(y.Sign, 1, iGreater), f.Api.Select(y.Sign, f.Api.Sub(1, iGreater), 0))
	iCoord = f.Api.Select(x.Sign,
		f.Api.Select(y.Sign,
			iCoord, f.Api.Select(iGreater, f.Api.Sub(iCoord, jCoord), f.Api.Sub(jCoord, iCoord))),
		f.Api.Select(y.Sign,
			f.Api.Select(iGreater, f.Api.Sub(iCoord, jCoord), f.Api.Sub(jCoord, iCoord)), iCoord))

	return NormalizeIJK(f.Api, iCoordNegative, iCoord, y.Sign, jCoord, 0, 0)

}

// TODO: Comments
func NormalizeIJK(api frontend.API, iCoordNegative frontend.Variable, iCoord frontend.Variable, jCoordNegative frontend.Variable, jCoord frontend.Variable, kCoordNegative frontend.Variable, kCoord frontend.Variable) [3]frontend.Variable {

	iGreaterj := comparator.IsLess(api, jCoord, iCoord)
	jTmp := api.Select(jCoordNegative,
		api.Select(iGreaterj, api.Sub(iCoord, jCoord), api.Sub(jCoord, iCoord)),
		api.Add(iCoord, jCoord))
	jTmpNegative := api.Select(jCoordNegative, api.Sub(1, iGreaterj), 0)
	iGreaterk := comparator.IsLess(api, kCoord, iCoord)
	kTmp := api.Select(kCoordNegative,
		api.Select(iGreaterk, api.Sub(iCoord, kCoord), api.Sub(kCoord, iCoord)),
		api.Add(iCoord, kCoord))
	kTmpNegative := api.Select(kCoordNegative, api.Sub(1, iGreaterk), 0)

	// if i < 0
	iCoord = api.Select(iCoordNegative, 0, iCoord)
	jCoord = api.Select(iCoordNegative, jTmp, jCoord)
	jCoordNegative = api.Select(iCoordNegative, jTmpNegative, jCoordNegative)
	kCoord = api.Select(iCoordNegative, kTmp, kCoord)
	kCoordNegative = api.Select(iCoordNegative, kTmpNegative, kCoordNegative)

	jGreaterk := comparator.IsLess(api, kCoord, jCoord)
	kTmp = api.Select(kCoordNegative,
		api.Select(jGreaterk, api.Sub(jCoord, kCoord), api.Sub(kCoord, jCoord)),
		api.Add(jCoord, kCoord))
	kTmpNegative = api.Select(kCoordNegative, api.Sub(1, jGreaterk), 0)

	// if j < 0
	iCoord = api.Select(jCoordNegative, api.Add(iCoord, jCoord), iCoord)
	jCoord = api.Select(jCoordNegative, 0, jCoord)
	kCoord = api.Select(jCoordNegative, kTmp, kCoord)
	kCoordNegative = api.Select(jCoordNegative, kTmpNegative, kCoordNegative)

	// if k < 0
	iCoord = api.Select(kCoordNegative, api.Add(iCoord, kCoord), iCoord)
	jCoord = api.Select(kCoordNegative, api.Add(jCoord, kCoord), jCoord)
	kCoord = api.Select(kCoordNegative, 0, kCoord)

	iGreaterj = comparator.IsLess(api, jCoord, iCoord)
	min := api.Select(iGreaterj, jCoord, iCoord)
	min = api.Select(comparator.IsLess(api, kCoord, min), kCoord, min)

	i := api.Sub(iCoord, min)
	j := api.Sub(jCoord, min)
	k := api.Sub(kCoord, min)

	return [3]frontend.Variable{i, j, k}
}
