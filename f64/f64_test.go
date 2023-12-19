package f64

import (
	"bufio"
	"fmt"
	"gnark-float/gadget"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

type Circuit struct {
	X frontend.Variable `gnark:",secret"`
	Y frontend.Variable `gnark:",secret"`
	Z frontend.Variable `gnark:",public"`
}

func (c *Circuit) Define(api frontend.API) error {
	gadget := gadget.New(api)
	ctx := Float{api, gadget}
	x := ctx.NewF64(c.X)
	y := ctx.NewF64(c.Y)
	z := ctx.NewF64(c.Z)
	ctx.AssertIsEqual(ctx.Add(x, y), z)
	return nil
}

func TestCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	path, _ := filepath.Abs("../data/f64/add")
	file, err := os.Open(path)
	assert.NoError(err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())
		a, _ := new(big.Int).SetString(data[0], 16)
		b, _ := new(big.Int).SetString(data[1], 16)
		c, _ := new(big.Int).SetString(data[2], 16)

		fmt.Printf(
			"%v %v %v \n",
			math.Float64frombits(a.Uint64()),
			math.Float64frombits(b.Uint64()),
			math.Float64frombits(c.Uint64()),
		)

		circuit := Circuit{
			X: a,
			Y: b,
			Z: c,
		}
		witness := Circuit{
			X: a,
			Y: b,
			Z: c,
		}

		err := test.IsSolved(&circuit, &witness, ecc.BN254.ScalarField(), test.SetAllVariablesAsConstants())
		assert.NoError(err)

		// TODO: fixme
		// assert.ProverSucceeded(&circuit, &witness, test.WithCurves(ecc.BN254))
	}
}
