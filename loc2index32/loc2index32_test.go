package main

import (
	"bufio"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"

	float "gnark-float/float"
	maths "gnark-float/math"
	util "gnark-float/util"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestLoc2Index32(t *testing.T) {
	ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &Loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0})

	for i := 0; i <= 15; i++ {
		file, err := os.Open(fmt.Sprintf("../data/f32/loc2index/%v_100.txt", i))

		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var failures [16]int
		line := 0
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())

			lat, _ := new(big.Int).SetString(data[0], 16)
			lng, _ := new(big.Int).SetString(data[1], 16)
			res, _ := new(big.Int).SetString(data[2], 16)
			i, _ := new(big.Int).SetString(data[3], 16)
			j, _ := new(big.Int).SetString(data[4], 16)
			k, _ := new(big.Int).SetString(data[5], 16)

			full, err := frontend.NewWitness(&Loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}, ecc.BN254.ScalarField())

			_, err = ccs.Solve(full)
			if err != nil {
				failures[line%16]++
			}
			line += 1
		}

		t.Logf("Failures at distance %d: %v", i, failures)
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

	var circuits, assignments []Loc2Index32Circuit
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

		circuit := Loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := Loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

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

	var circuits, assignments []Loc2Index32Circuit

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

		circuit := Loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := Loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

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
		if err := util.BenchProofMemoryPlonk(b, &circuits[i], &assignments[i]); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}
}
