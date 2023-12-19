package f64

import (
	"bufio"
	"fmt"
	"gnark-float/gadget"
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

type Circuit struct {
	X  frontend.Variable `gnark:",secret"`
	Y  frontend.Variable `gnark:",secret"`
	Z  frontend.Variable `gnark:",public"`
	op string
}

func (c *Circuit) Define(api frontend.API) error {
	gadget := gadget.New(api)
	ctx := Float{api, gadget}
	x := ctx.NewF64(c.X)
	y := ctx.NewF64(c.Y)
	z := ctx.NewF64(c.Z)
	ctx.AssertIsEqual(reflect.ValueOf(&ctx).MethodByName(c.op).Call([]reflect.Value{reflect.ValueOf(x), reflect.ValueOf(y)})[0].Interface().(F64), z)
	return nil
}

func TestCircuit(t *testing.T) {
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
				&Circuit{X: 0, Y: 0, Z: 0, op: op},
				&Circuit{X: a, Y: b, Z: c, op: op},
				test.WithCurves(ecc.BN254),
				test.WithBackends(backend.GROTH16, backend.PLONK),
			)
		}
	}
}
