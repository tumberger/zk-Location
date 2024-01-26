package loc2index64

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"

	float "gnark-float/float"
	"gnark-float/hint"
	maths "gnark-float/math"
	util "gnark-float/util"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"

	"github.com/consensys/gnark/test"
)

type loc2Index64Params struct {
	LatS       int `json:"LatS,string"`
	LatE       int `json:"LatE,string"`
	LatM       int `json:"LatM,string"`
	LatA       int `json:"LatA,string"`
	LngS       int `json:"LngS,string"`
	LngE       int `json:"LngE,string"`
	LngM       int `json:"LngM,string"`
	LngA       int `json:"LngA,string"`
	Resolution int `json:"Resolution,string"`
}

const loc2Index64 = `{
    "LatS": "1",
	"LatE": "0",
	"LatM": "4527589427376396",
	"LatA": "0",
    "LngS": "0",
	"LngE": "0",
	"LngM": "5370059476281135",
	"LngA": "0",
	"Resolution": "0"
}`

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

func (circuit *loc2Index64Wrapper) Define(api frontend.API) error {

	api.AssertIsEqual(circuit.LatA, 0)
	api.AssertIsEqual(circuit.LngA, 0)

	f := float.NewContext(api, 0, util.IEEE64ExponentBitwidth, util.IEEE64Precision)
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

	r := calculateR(&f, calc[0], resolution)
	hex2d := calculateHex2d(&f, z, cosLat, sinLng, cosLng, calc[1], calc[2], calc[3], calc[4], calc[5], calc[6], calc[7], calc[8], r, resolution)

	ijk := hex2dToCoordIJK(&f, hex2d[0], hex2d[1])

	api.AssertIsEqual(circuit.I, ijk[0])
	api.AssertIsEqual(circuit.J, ijk[1])
	api.AssertIsEqual(circuit.K, ijk[2])

	return nil
}

func setupLoc2IndexWrapper() (loc2Index64Wrapper, loc2Index64Wrapper) {
	var data loc2Index64Params
	err := json.Unmarshal([]byte(loc2Index64), &data)
	if err != nil {
		panic(err)
	}

	lat := math.Pow(2, float64(data.LatE)) * (float64(data.LatM) / math.Pow(2, float64(52)))
	if data.LatS == 1 {
		lat = -lat
	}
	lng := math.Pow(2, float64(data.LngE)) * (float64(data.LngM) / math.Pow(2, float64(52)))
	if data.LngS == 1 {
		lng = -lng
	}

	fmt.Printf("lat, lng: %f, %f\n", lat, lng)

	// Calculate I, J, K using the H3 library in C
	i, j, k, err := util.ExecuteLatLngToIJK(data.Resolution, util.RadiansToDegrees(lat), util.RadiansToDegrees(lng))
	if err != nil {
		panic(err)
	}

	// Update witness values with calculated I, J, K
	assignment := loc2Index64Wrapper{
		LatS:       data.LatS,
		LatE:       data.LatE,
		LatM:       data.LatM,
		LatA:       data.LatA,
		LngS:       data.LngS,
		LngE:       data.LngE,
		LngM:       data.LngM,
		LngA:       data.LngA,
		I:          i,
		J:          j,
		K:          k,
		Resolution: data.Resolution,
	}

	circuit := loc2Index64Wrapper{
		// The circuit does not need actual values for I, J, K since these are
		// calculated within the circuit itself when running the proof or solving
		LatS:       data.LatS,
		LatE:       data.LatE,
		LatM:       data.LatM,
		LatA:       data.LatA,
		LngS:       data.LngS,
		LngE:       data.LngE,
		LngM:       data.LngM,
		LngA:       data.LngA,
		Resolution: data.Resolution,
	}
	return circuit, assignment
}

type loc2Index64Circuit struct {

	// SECRET INPUTS
	Lat frontend.Variable `gnark:",secret"`
	Lng frontend.Variable `gnark:",secret"`

	// PUBLIC INPUTS
	Resolution frontend.Variable `gnark:",public"`
	I          frontend.Variable `gnark:",public"`
	J          frontend.Variable `gnark:",public"`
	K          frontend.Variable `gnark:",public"`
}

func (c *loc2Index64Circuit) Define(api frontend.API) error {

	ctx := float.NewContext(api, 0, util.IEEE64ExponentBitwidth, util.IEEE64Precision)
	lat := ctx.NewFloat(c.Lat)
	lng := ctx.NewFloat(c.Lng)

	resolution := c.Resolution

	pi := ctx.NewF64Constant(math.Pi)
	halfPi := ctx.NewF64Constant(math.Pi / 2.0)
	doublePi := ctx.NewF64Constant(math.Pi * 2.0)

	// Lat can't be more than pi/2, Lng can't be more than pi and max resolution is 15
	api.AssertIsEqual(ctx.IsGt(lat, halfPi), 0)
	api.AssertIsEqual(ctx.IsGt(lng, pi), 0)
	api.AssertIsLessOrEqual(resolution, util.MaxResolution)

	// Adding half pi to latitude to apply cos() -- lat always in range [-pi/2, pi/2]
	term := ctx.Add(lat, halfPi)

	// Adding half pi to longitude to apply cos() -- lng always in range [-pi, pi]
	tmp := ctx.Add(lng, halfPi)

	// TODO: If it makes no big difference in regards to constraints: (input % 2pi) - pi
	// can be applied on the input at the start of SinTaylor and the next lines can be deleted
	isGreater := ctx.IsGt(tmp, pi)
	shifted := ctx.Sub(tmp, doublePi)
	term.Sign = api.Select(isGreater, shifted.Sign, tmp.Sign)
	term.Exponent = api.Select(isGreater, shifted.Exponent, tmp.Exponent)
	term.Mantissa = api.Select(isGreater, shifted.Mantissa, tmp.Mantissa)
	term.IsAbnormal = 0

	// Compute alpha, beta, gamma, delta for Latitude
	alphaLat := ctx.NewFloat(0)
	betaLat := ctx.NewFloat(0)
	gammaLat := ctx.NewFloat(0)
	deltaLat := ctx.NewFloat(0)
	precomputeLat, err := ctx.Api.Compiler().NewHint(hint.PrecomputeHint64, 16, c.Lat, ctx.E, ctx.M)
	if err != nil {
		panic(err)
	}
	alphaLat.Sign = precomputeLat[0]
	alphaLat.Exponent = precomputeLat[1]
	alphaLat.Mantissa = precomputeLat[2]
	alphaLat.IsAbnormal = precomputeLat[3]

	betaLat.Sign = precomputeLat[4]
	betaLat.Exponent = precomputeLat[5]
	betaLat.Mantissa = precomputeLat[6]
	betaLat.IsAbnormal = precomputeLat[7]

	gammaLat.Sign = precomputeLat[8]
	gammaLat.Exponent = precomputeLat[9]
	gammaLat.Mantissa = precomputeLat[10]
	gammaLat.IsAbnormal = precomputeLat[11]

	deltaLat.Sign = precomputeLat[12]
	deltaLat.Exponent = precomputeLat[13]
	deltaLat.Mantissa = precomputeLat[14]
	deltaLat.IsAbnormal = precomputeLat[15]

	// Compute alpha, beta, gamma, delta for Longitude
	alphaLng := ctx.NewFloat(0)
	betaLng := ctx.NewFloat(0)
	gammaLng := ctx.NewFloat(0)
	deltaLng := ctx.NewFloat(0)
	precomputeLng, err := ctx.Api.Compiler().NewHint(hint.PrecomputeHint64, 16, c.Lng, ctx.E, ctx.M)
	if err != nil {
		panic(err)
	}
	alphaLng.Sign = precomputeLng[0]
	alphaLng.Exponent = precomputeLng[1]
	alphaLng.Mantissa = precomputeLng[2]
	alphaLng.IsAbnormal = precomputeLng[3]

	betaLng.Sign = precomputeLng[4]
	betaLng.Exponent = precomputeLng[5]
	betaLng.Mantissa = precomputeLng[6]
	betaLng.IsAbnormal = precomputeLng[7]

	gammaLng.Sign = precomputeLng[8]
	gammaLng.Exponent = precomputeLng[9]
	gammaLng.Mantissa = precomputeLng[10]
	gammaLng.IsAbnormal = precomputeLng[11]

	deltaLng.Sign = precomputeLng[12]
	deltaLng.Exponent = precomputeLng[13]
	deltaLng.Mantissa = precomputeLng[14]
	deltaLng.IsAbnormal = precomputeLng[15]

	// Check 1 (Identity) for Latitude and Longitude
	deltaLatSquared := ctx.Mul(deltaLat, deltaLat)
	gammaLatSquared := ctx.Mul(gammaLat, gammaLat)
	identityLat := ctx.Add(gammaLatSquared, deltaLatSquared)
	deltaLngSquared := ctx.Mul(deltaLng, deltaLng)
	gammaLngSquared := ctx.Mul(gammaLng, gammaLng)
	identityLng := ctx.Add(gammaLngSquared, deltaLngSquared)
	ctx.AssertIsEqualOrCustomULP64(identityLat, ctx.NewF64Constant(1), 2.0)
	ctx.AssertIsEqualOrCustomULP64(identityLng, ctx.NewF64Constant(1), 2.0)

	// Check 2 for Latitude and Longitude
	ctx.AssertIsEqualOrCustomULP64(ctx.Mul(alphaLat, deltaLat), gammaLat, 2.0)
	ctx.AssertIsEqualOrULP(ctx.Mul(alphaLng, deltaLng), gammaLng)

	// Check 3 for Latitude and Longitude
	ctx.AssertIsEqualOrULP(ctx.Mul(ctx.NewF64Constant(2), ctx.Mul(gammaLat, deltaLat)), betaLat)
	ctx.AssertIsEqualOrULP(ctx.Mul(ctx.NewF64Constant(2), ctx.Mul(gammaLng, deltaLng)), betaLng)

	// Calculate Cosine for Latitude and Longitude
	cosLat := ctx.Sub(ctx.Mul(deltaLat, deltaLat), ctx.Mul(gammaLat, gammaLat))
	cosLng := ctx.Sub(ctx.Mul(deltaLng, deltaLng), ctx.Mul(gammaLng, gammaLng))

	// Calculate Sin for Latitude and Longitude
	z := betaLat
	sinLng := betaLng

	// Calculate x & z for 3D Cartesian
	x := ctx.Mul(cosLat, cosLng)
	y := ctx.Mul(cosLat, sinLng)

	calc := closestFaceCalculations(&ctx, x, y, z, lng)

	r := calculateR(&ctx, calc[0], resolution)

	hex2d := calculateHex2d(&ctx, z, cosLat, sinLng, cosLng, calc[1], calc[2], calc[3], calc[4], calc[5], calc[6], calc[7], calc[8], r, resolution)

	ijk := hex2dToCoordIJK(&ctx, hex2d[0], hex2d[1])

	api.AssertIsEqual(c.I, ijk[0])
	api.AssertIsEqual(c.J, ijk[1])
	api.AssertIsEqual(c.K, ijk[2])

	return nil
}

func TestLoc2Index64(t *testing.T) {
	assert := test.NewAssert(t)

	file, err := os.Open("../data/f64/loc2index64.txt")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		fmt.Printf("lat: %f, lng: %f\n", math.Float64frombits(lat.Uint64()), math.Float64frombits(lng.Uint64()))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Calculate I, J, K using the H3 library in C - just for comparison and debugging
		latFloat, _ := util.HexToFloat(data[0])
		lngFloat, _ := util.HexToFloat(data[1])
		fmt.Printf("latFloat: %f, lngFloat: %f\n", latFloat, lngFloat)
		iInt, jInt, kInt, err := util.ExecuteLatLngToIJK(15, latFloat, lngFloat)
		if err != nil {
			panic(err)
		}
		fmt.Printf("i: %d, j: %d, k: %d\n", iInt, jInt, kInt)

		assert.ProverSucceeded(
			&loc2Index64Circuit{Lat: 0, Lng: 0, Resolution: 0},
			&loc2Index64Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k},
			test.WithCurves(ecc.BLS12_381),
			test.WithBackends(backend.GROTH16),
		)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
}

func TestLoc2Index64Solving(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, assignment := setupLoc2IndexWrapper()

	// Solve the circuit and assert.
	assert.SolvingSucceeded(&circuit, &assignment, test.WithBackends(backend.GROTH16))
}

func TestLoc2Index64Proving(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, assignment := setupLoc2IndexWrapper()

	// Proof successfully generated
	assert.ProverSucceeded(&circuit, &assignment, test.WithBackends(backend.GROTH16))
}
