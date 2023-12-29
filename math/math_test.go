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
	//"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

type Circuit struct {
	X  frontend.Variable `gnark:",secret"`
	//Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *Circuit) Define(api frontend.API) error {
	// var f float32.Float
	ctx := float.NewContext(api, 11, 52)

	x := ctx.NewFloat(c.X)
	// y := ctx.NewF64(c.Y)
	z := ctx.NewFloat(c.Z)

	result := AtanRemez64(&ctx, x)

	// ToDo - Assertion would currently fail!
	api.AssertIsEqual(result.Exponent, z.Exponent)
	api.AssertIsEqual(result.Mantissa, z.Mantissa)

	return nil
}

func TestCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"Atan"}

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
				&Circuit{X: 0, Z: 0, op: "AtanRemez64"},
				&Circuit{X: a, Z: b, op: "AtanRemez64"},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16),
			)
			//break
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
