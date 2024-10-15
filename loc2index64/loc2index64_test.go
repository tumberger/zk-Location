package main

import (
	"bufio"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"

	util "gnark-float/util"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestLoc2Index64(t *testing.T) {
	ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &Loc2Index64Circuit{Lat: 0, Lng: 0, Resolution: 0})

	for i := 0; i <= 15; i++ {
		file, err := os.Open(fmt.Sprintf("../data/f64/loc2index/%v_100.txt", i))

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

			full, err := frontend.NewWitness(&Loc2Index64Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}, ecc.BN254.ScalarField())

			_, err = ccs.Solve(full)
			if err != nil {
				failures[line%16]++
			}
			line += 1
		}

		t.Logf("Failures at distance %d: %v", i, failures)
	}
}

func BenchmarkLoc2IndexProof(b *testing.B) {

	file, _ := os.Open("../data/f64/loc2index64.txt")
	defer file.Close()

	var circuits, assignments []Loc2Index64Circuit
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

		fmt.Printf("lat: %f, lng: %f\n", math.Float64frombits(uint64(lat.Uint64())), math.Float64frombits(uint64(lng.Uint64())))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Update the count for this resolution
		resolutionCounts[res.Int64()]++

		circuit := Loc2Index64Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := Loc2Index64Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

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
		if err := util.BenchProofToFileGroth16(b, &circuits[i], &assignments[i], resolutions[i], indices[i], "../benchmarks/raw/bench_ZKLP64_G16_BN254.txt"); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}

	for i := range circuits {
		if err := util.BenchProofToFilePlonk(b, &circuits[i], &assignments[i], resolutions[i], indices[i], "../benchmarks/raw/bench_ZKLP64_Plonk_BN254.txt"); err != nil {
			b.Logf("Error on benchmarking proof for entry %d: %v", i, err)
			continue
		}
	}
}

func BenchmarkLoc2IndexProofMemory(b *testing.B) {

	file, _ := os.Open("../data/f64/loc2index64.txt")
	defer file.Close()

	var circuits, assignments []Loc2Index64Circuit

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		fmt.Printf("lat: %f, lng: %f\n", math.Float64frombits(uint64(lat.Uint64())), math.Float64frombits(uint64(lng.Uint64())))
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		circuit := Loc2Index64Circuit{Lat: 0, Lng: 0, Resolution: 0}
		assignment := Loc2Index64Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k}

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
