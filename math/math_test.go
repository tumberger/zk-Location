package math

import (
	"bufio"
	"fmt"
	"gnark-float/float"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

type CircuitATanRemez64 struct {
	X  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *CircuitATanRemez64) Define(api frontend.API) error {

	ctx := float.NewContext(api, 11, 52)

	x := ctx.NewFloat(c.X)
	z := ctx.NewFloat(c.Z)

	result := AtanRemez64(&ctx, x)

	// Assertion of Mantissa fails, ULP test checks that ULP error <1
	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsEqual(result.Mantissa, z.Mantissa)
	return nil
}

type CircuitATanRemez32ULP struct {
	X       frontend.Variable `gnark:",secret"`
	Z       frontend.Variable `gnark:",public"`
	Z_lower frontend.Variable `gnark:",public"`
	Z_upper frontend.Variable `gnark:",public"`
	op      string
}

func (c *CircuitATanRemez32ULP) Define(api frontend.API) error {

	ctx := float.NewContext(api, 8, 23)

	x := ctx.NewFloat(c.X)
	z := ctx.NewFloat(c.Z)
	z_lower := ctx.NewFloat(c.Z_lower)
	z_upper := ctx.NewFloat(c.Z_upper)

	result := AtanRemez32(&ctx, x)

	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsLessOrEqual(z_lower.Mantissa, result.Mantissa)
	api.AssertIsLessOrEqual(result.Mantissa, z_upper.Mantissa)

	return nil
}

type CircuitATanRemez64ULP struct {
	X       frontend.Variable `gnark:",secret"`
	Z       frontend.Variable `gnark:",public"`
	Z_lower frontend.Variable `gnark:",public"`
	Z_upper frontend.Variable `gnark:",public"`
	op      string
}

func (c *CircuitATanRemez64ULP) Define(api frontend.API) error {

	ctx := float.NewContext(api, 11, 52)

	x := ctx.NewFloat(c.X)
	z := ctx.NewFloat(c.Z)
	z_lower := ctx.NewFloat(c.Z_lower)
	z_upper := ctx.NewFloat(c.Z_upper)

	result := AtanRemez64(&ctx, x)

	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsLessOrEqual(z_lower.Mantissa, result.Mantissa)
	api.AssertIsLessOrEqual(result.Mantissa, z_upper.Mantissa)

	return nil
}

type SinCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *SinCircuit) Define(api frontend.API) error {
	ctx := float.NewContext(api, 11, 52)

	x := ctx.NewFloat(c.X)
	z := ctx.NewFloat(c.Z)

	result := SinTaylor64(&ctx, x)

	// Assertion of Mantissa fails, ULP test checks that ULP error <1
	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsEqual(result.Mantissa, z.Mantissa)
	return nil
}

type CircuitSin64ULP struct {
	X       frontend.Variable `gnark:",secret"`
	Z       frontend.Variable `gnark:",public"`
	Z_lower frontend.Variable `gnark:",public"`
	Z_upper frontend.Variable `gnark:",public"`
	op      string
}

func (c *CircuitSin64ULP) Define(api frontend.API) error {

	ctx := float.NewContext(api, 11, 52)

	x := ctx.NewFloat(c.X)
	z := ctx.NewFloat(c.Z)
	z_lower := ctx.NewFloat(c.Z_lower)
	z_upper := ctx.NewFloat(c.Z_upper)

	result := SinTaylor64(&ctx, x)

	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsLessOrEqual(z_lower.Mantissa, result.Mantissa)
	api.AssertIsLessOrEqual(result.Mantissa, z_upper.Mantissa)

	return nil
}

func TestATanCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"atan"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)

			assert.ProverSucceeded(
				&CircuitATanRemez64{X: 0, Z: 0, op: "AtanRemez64"},
				&CircuitATanRemez64{X: a, Z: b, op: "AtanRemez64"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
		}
	}
}

func TestCircuitATanRemez32ULP(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"atan_ulp"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f32/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 16)
			d, _ := new(big.Int).SetString(data[3], 16)

			assert.ProverSucceeded(
				&CircuitATanRemez32ULP{X: 0, Z: 0, Z_lower: 0, Z_upper: 0, op: "AtanRemez32"},
				&CircuitATanRemez32ULP{X: a, Z: b, Z_lower: c, Z_upper: d, op: "AtanRemez32"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
		}
	}
}

func TestCircuitATanRemez64ULP(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"atan_ulp"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 16)
			d, _ := new(big.Int).SetString(data[3], 16)

			assert.ProverSucceeded(
				&CircuitATanRemez64ULP{X: 0, Z: 0, Z_lower: 0, Z_upper: 0, op: "AtanRemez64"},
				&CircuitATanRemez64ULP{X: a, Z: b, Z_lower: c, Z_upper: d, op: "AtanRemez64"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
		}
	}
}

func TestCircuitSin(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"sin"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)

			assert.ProverSucceeded(
				&SinCircuit{X: 0, Z: 0, op: "SinTaylor64"},
				&SinCircuit{X: a, Z: b, op: "SinTaylor64"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
			break
		}
	}
}

func TestCircuitSin64ULP(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"sin_ulp"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 16)
			d, _ := new(big.Int).SetString(data[3], 16)

			assert.ProverSucceeded(
				&CircuitSin64ULP{X: 0, Z: 0, Z_lower: 0, Z_upper: 0, op: "SinTaylor64"},
				&CircuitSin64ULP{X: a, Z: b, Z_lower: c, Z_upper: d, op: "SinTaylor64"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
		}
	}
}

/*
func TestRealProofComputation(t *testing.T) {

	ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &Circuit{})

	fmt.Printf("Number of constraints %d", ccs.GetNbConstraints())

	pk, _, _ := groth16.Setup(ccs)

	// ToDo - This currently uses the Floats as defined for floating point tests
	// Change - generate raw data for atan etc.
	ops := []string{"Add"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())

			// ToDo - REMOVE HARD CODE once test values generated
			// 48.134 and 11.582
			data[0] = "42408937"
			data[1] = "41394fdf"
			// sin(48.134) = 0.74470771
			// data[2] = "3f3ea52a"

			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 16)

			assignment := &Circuit{
				X:  a,
				Y:  b,
				Z:  c,
				op: op,
			}

			witness, _ := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
			// publicWitness, _ := witness.Public()

			_, err := groth16.Prove(ccs, pk, witness)
			// err = plonk.Verify(proof, vk, publicWitness)
			if err != nil {
				panic(err)
			}
			// ToDo - Add assertion that proof verifies (not done due to missing sanity check data)
			break
		}
	}

}*/
