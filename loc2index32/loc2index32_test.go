package loc2index64

import (
	"bufio"
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

type loc2Index32Circuit struct {

	// SECRET INPUTS
	Lat frontend.Variable `gnark:",secret"`
	Lng frontend.Variable `gnark:",secret"`

	// PUBLIC INPUTS
	Resolution frontend.Variable `gnark:",public"`
	I          frontend.Variable `gnark:",public"`
	J          frontend.Variable `gnark:",public"`
	K          frontend.Variable `gnark:",public"`
}

func (c *loc2Index32Circuit) Define(api frontend.API) error {

	ctx := float.NewContext(api, 0, util.IEEE32ExponentBitwidth, util.IEEE32Precision)
	lat := ctx.NewFloat(c.Lat)
	lng := ctx.NewFloat(c.Lng)

	resolution := c.Resolution

	pi := ctx.NewF32Constant(math.Pi)
	halfPi := ctx.NewF32Constant(math.Pi / 2.0)

	// Lat can't be more than pi/2, Lng can't be more than pi and max resolution is 15
	api.AssertIsEqual(ctx.IsGt(lat, halfPi), 0)
	api.AssertIsEqual(ctx.IsGt(lng, pi), 0)
	api.AssertIsLessOrEqual(resolution, util.MaxResolution)

	// Compute alpha, beta, gamma, delta for Latitude
	alphaLat := ctx.NewFloat(0)
	betaLat := ctx.NewFloat(0)
	gammaLat := ctx.NewFloat(0)
	deltaLat := ctx.NewFloat(0)
	precomputeLat, err := ctx.Api.Compiler().NewHint(hint.PrecomputeHint32, 16, c.Lat, ctx.E, ctx.M)
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
	precomputeLng, err := ctx.Api.Compiler().NewHint(hint.PrecomputeHint32, 16, c.Lng, ctx.E, ctx.M)
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
	ctx.AssertIsEqualOrCustomULP32(identityLat, ctx.NewF32Constant(1), 1.0)
	ctx.AssertIsEqualOrCustomULP32(identityLng, ctx.NewF32Constant(1), 1.0)

	// Check 2 for Latitude and Longitude
	ctx.AssertIsEqualOrCustomULP32(ctx.Mul(alphaLat, deltaLat), gammaLat, 1.0)
	ctx.AssertIsEqualOrCustomULP32(ctx.Mul(alphaLng, deltaLng), gammaLng, 1.0)

	// Check 3 for Latitude and Longitude
	ctx.AssertIsEqualOrCustomULP32(ctx.Mul(ctx.NewF32Constant(2), ctx.Mul(gammaLat, deltaLat)), betaLat, 1.0)
	ctx.AssertIsEqualOrCustomULP32(ctx.Mul(ctx.NewF32Constant(2), ctx.Mul(gammaLng, deltaLng)), betaLng, 1.0)

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

func TestLoc2Index32(t *testing.T) {
	assert := test.NewAssert(t)

	file, err := os.Open("../data/f32/loc2index32.txt")
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

		fmt.Printf("lat: %f, lng: %f\n", math.Float32frombits(uint32(lat.Uint64())), math.Float32frombits(uint32(lng.Uint64())))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Calculate I, J, K using the H3 library in C - just for comparison and debugging
		latFloat, _ := util.HexToFloat32(data[0])
		lngFloat, _ := util.HexToFloat32(data[1])
		fmt.Printf("latFloat: %f, lngFloat: %f\n", latFloat, lngFloat)
		// iInt, jInt, kInt, err := util.ExecuteLatLngToIJK(15, latFloat, lngFloat)
		// if err != nil {
		// 	panic(err)
		// }
		// fmt.Printf("i: %d, j: %d, k: %d\n", iInt, jInt, kInt)

		assert.ProverSucceeded(
			&loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0},
			&loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k},
			test.WithCurves(ecc.BN254),
			test.WithBackends(backend.GROTH16),
		)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
}

type loc2Index32CircuitWrapper struct {
	// SECRET INPUTS
	Lat frontend.Variable `gnark:",secret"`
	Lng frontend.Variable `gnark:",secret"`

	// PUBLIC INPUTS
	Resolution frontend.Variable `gnark:",public"`
	I          frontend.Variable `gnark:",public"`
	J          frontend.Variable `gnark:",public"`
	K          frontend.Variable `gnark:",public"`
}

func (c *loc2Index32CircuitWrapper) Define(api frontend.API) error {

	ctx := float.NewContext(api, 0, util.IEEE32ExponentBitwidth, util.IEEE32Precision)
	lat := ctx.NewFloat(c.Lat)
	lng := ctx.NewFloat(c.Lng)

	resolution := c.Resolution

	pi := ctx.NewF32Constant(math.Pi)
	halfPi := ctx.NewF32Constant(math.Pi / 2.0)
	doublePi := ctx.NewF32Constant(math.Pi * 2.0)

	// Lat can't be more than pi/2, Lng can't be more than pi and max resolution is 15
	api.AssertIsEqual(ctx.IsGt(lat, halfPi), 0)
	api.AssertIsEqual(ctx.IsGt(lng, pi), 0)
	api.AssertIsLessOrEqual(resolution, util.MaxResolution)

	// Adding half pi to latitude to apply cos() -- lat always in range [-pi/2, pi/2]
	term := ctx.Add(lat, halfPi)
	cosLat := maths.SinTaylor32(&ctx, term)

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

	cosLng := maths.SinTaylor32(&ctx, term)
	x := ctx.Mul(cosLat, cosLng)

	sinLng := maths.SinTaylor32(&ctx, lng)
	y := ctx.Mul(cosLat, sinLng)

	z := maths.SinTaylor32(&ctx, lat)

	calc := closestFaceCalculations(&ctx, x, y, z, lng)

	r := calculateR(&ctx, calc[0], resolution)
	hex2d := calculateHex2d(&ctx, z, cosLat, sinLng, cosLng, calc[1], calc[2], calc[3], calc[4], calc[5], calc[6], calc[7], calc[8], r, resolution)

	ijk := hex2dToCoordIJK(&ctx, hex2d[0], hex2d[1])

	api.AssertIsEqual(c.I, ijk[0])
	api.AssertIsEqual(c.J, ijk[1])
	api.AssertIsEqual(c.K, ijk[2])

	return nil
}

func setupLoc2IndexWrapper() ([]loc2Index32CircuitWrapper, []loc2Index32CircuitWrapper, []int64, []int64) {
	file, _ := os.Open("../data/f32/loc2index32.txt")
	defer file.Close()

	var circuits, assignments []loc2Index32CircuitWrapper
	var resolutions, indices []int64
	resolutionCounts := make(map[int64]int64)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		// Update the count for this resolution
		resolutionCounts[res.Int64()]++

		circuit := loc2Index32CircuitWrapper{Lat: 0, Lng: 0, Resolution: 0}
		assignment := loc2Index32CircuitWrapper{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

		// Append the created structs to the slices
		circuits = append(circuits, circuit)
		assignments = append(assignments, assignment)
		resolutions = append(resolutions, res.Int64())
		indices = append(indices, resolutionCounts[res.Int64()])
	}

	return circuits, assignments, resolutions, indices
}

func BenchmarkLoc2IndexProof(b *testing.B) {

	file, _ := os.Open("../data/f32/loc2index32.txt")
	defer file.Close()

	var circuits, assignments []loc2Index32Circuit
	var resolutions, indices []int64
	resolutionCounts := make(map[int64]int64)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		fmt.Printf("lat: %f, lng: %f\n", math.Float32frombits(uint32(lat.Uint64())), math.Float32frombits(uint32(lng.Uint64())))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Update the count for this resolution
		resolutionCounts[res.Int64()]++

		circuit := loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

		// Append the created structs to the slices
		circuits = append(circuits, circuit)
		assignments = append(assignments, assignment)
		resolutions = append(resolutions, res.Int64())
		indices = append(indices, resolutionCounts[res.Int64()])
	}

	if err := scanner.Err(); err != nil {
		b.Fatalf("Error reading file: %v", err)
	}

	// Ensure that the number of circuits and assignments is the same
	if len(circuits) != len(assignments) {
		b.Fatalf("Mismatch in number of circuits and assignments")
	}

	for i := range circuits {
		if err := util.BenchProofToFileGroth16(b, &circuits[i], &assignments[i], resolutions[i], indices[i], "../benchmarks/raw/bench_ZKLP32_G16_BN254.txt"); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}

	for i := range circuits {
		if err := util.BenchProofToFilePlonk(b, &circuits[i], &assignments[i], resolutions[i], indices[i], "../benchmarks/raw/bench_ZKLP32_Plonk_BN254.txt"); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}
}

func BenchmarkLoc2IndexProofMemory(b *testing.B) {

	file, _ := os.Open("../data/f32/loc2index32.txt")
	defer file.Close()

	var circuits, assignments []loc2Index32Circuit

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		fmt.Printf("lat: %f, lng: %f\n", math.Float32frombits(uint32(lat.Uint64())), math.Float32frombits(uint32(lng.Uint64())))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		circuit := loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

		// Append the created structs to the slices
		circuits = append(circuits, circuit)
		assignments = append(assignments, assignment)
		break
	}

	if err := scanner.Err(); err != nil {
		b.Fatalf("Error reading file: %v", err)
	}

	// Ensure that the number of circuits and assignments is the same
	if len(circuits) != len(assignments) {
		b.Fatalf("Mismatch in number of circuits and assignments")
	}

	for i := range circuits {
		if err := util.BenchProofMemoryGroth16(b, &circuits[i], &assignments[i]); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}
}
