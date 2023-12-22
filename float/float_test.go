package float

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

type F32UnaryCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",public"`
	op string
}

func (c *F32UnaryCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 8, 23)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	ctx.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x)})[0].Interface().(FloatVar), y)
	return nil
}

type F32BinaryCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *F32BinaryCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 8, 23)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	z := ctx.NewFloat(c.Z)
	ctx.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x), reflect.ValueOf(y)})[0].Interface().(FloatVar), z)
	return nil
}

type F32ComparisonCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *F32ComparisonCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 8, 23)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	z := c.Z
	api.AssertIsBoolean(z)
	api.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x), reflect.ValueOf(y)})[0].Interface(), z)
	return nil
}

type F64UnaryCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",public"`
	op string
}

func (c *F64UnaryCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 11, 52)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	ctx.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x)})[0].Interface().(FloatVar), y)
	return nil
}

type F64BinaryCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *F64BinaryCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 11, 52)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	z := ctx.NewFloat(c.Z)
	ctx.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x), reflect.ValueOf(y)})[0].Interface().(FloatVar), z)
	return nil
}

type F64ComparisonCircuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *F64ComparisonCircuit) Define(api frontend.API) error {
	ctx := NewContext(api, 11, 52)
	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)
	z := c.Z
	api.AssertIsBoolean(z)
	api.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x), reflect.ValueOf(y)})[0].Interface(), z)
	return nil
}

func TestF32UnaryCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"Sqrt"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f32/%s", strings.ToLower(op)))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)

			assert.ProverSucceeded(
				&F32UnaryCircuit{X: 0, Y: 0, op: op},
				&F32UnaryCircuit{X: a, Y: b, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}

func TestF32BinaryCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"Add", "Sub", "Mul", "Div"}

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

			assert.ProverSucceeded(
				&F32BinaryCircuit{X: 0, Y: 0, Z: 0, op: op},
				&F32BinaryCircuit{X: a, Y: b, Z: c, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}

func TestF32ComparisonCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"IsLt", "IsLe", "IsGt", "IsGe"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f32/%s", strings.ToLower(op[2:])))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 2)

			assert.ProverSucceeded(
				&F32ComparisonCircuit{X: 0, Y: 0, Z: 0, op: op},
				&F32ComparisonCircuit{X: a, Y: b, Z: c, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}

func TestF64UnaryCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"Sqrt"}

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
				&F64UnaryCircuit{X: 0, Y: 0, op: op},
				&F64UnaryCircuit{X: a, Y: b, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}

func TestF64BinaryCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"Add", "Sub", "Mul", "Div"}

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

			assert.ProverSucceeded(
				&F64BinaryCircuit{X: 0, Y: 0, Z: 0, op: op},
				&F64BinaryCircuit{X: a, Y: b, Z: c, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}

func TestF64ComparisonCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	ops := []string{"IsLt", "IsLe", "IsGt", "IsGe"}

	for _, op := range ops {
		path, _ := filepath.Abs(fmt.Sprintf("../data/f64/%s", strings.ToLower(op[2:])))
		file, _ := os.Open(path)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			data := strings.Fields(scanner.Text())
			a, _ := new(big.Int).SetString(data[0], 16)
			b, _ := new(big.Int).SetString(data[1], 16)
			c, _ := new(big.Int).SetString(data[2], 2)

			assert.ProverSucceeded(
				&F64ComparisonCircuit{X: 0, Y: 0, Z: 0, op: op},
				&F64ComparisonCircuit{X: a, Y: b, Z: c, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}