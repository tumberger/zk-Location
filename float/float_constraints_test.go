package float

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

type Param struct {
	name string
	E    uint
	M    uint
}

var params = []Param{
	{E: 8, M: 23, name: "F32"},
	{E: 11, M: 52, name: "F64"},
}

type Constraints struct {
	native        uint
	lookup_query  uint
	lookup_global uint // A one-time cost that can be amortized over many queries
}

type F32Circuit struct {
	X      frontend.Variable `gnark:",secret"`
	Y      frontend.Variable `gnark:",secret"`
	E      uint
	M      uint
	size   uint
	result []Constraints
}

func count_constraints(ctx Context, f func()) Constraints {
	native_constraints := ctx.Api.GetNbConstraints()
	lookup_query_constraints := ctx.Gadget.LookupQueryConstraints()
	f()
	native_constraints = ctx.Api.GetNbConstraints() - native_constraints
	lookup_query_constraints = ctx.Gadget.LookupQueryConstraints() - lookup_query_constraints
	return Constraints{
		uint(native_constraints),
		lookup_query_constraints,
		ctx.Gadget.LookupEntryConstraints() + ctx.Gadget.LookupFinalizeConstraints(),
	}
}

func (c *F32Circuit) Define(api frontend.API) error {
	ctx := NewContext(api, c.size, c.E, c.M)

	x := ctx.NewFloat(c.X)
	y := ctx.NewFloat(c.Y)

	for _, op := range []func(){
		func() { ctx.NewFloat(c.X) },
		func() { ctx.Add(x, y) },
		func() { ctx.Sub(x, y) },
		func() { ctx.Mul(x, y) },
		func() { ctx.Div(x, y) },
		func() { ctx.Sqrt(x) },
		func() { ctx.IsLt(x, y) },
	} {
		c.result = append(c.result, count_constraints(ctx, op))
	}

	return nil
}

var (
    _, b, _, _ = runtime.Caller(0)
    basepath   = filepath.Dir(b)
)


// Note that although we can measure the number of constraints for multiple operations,
// the result returned by gnark might be greater than the actual number when `CompressThreshold`
// is not large enough.

func TestF32CircuitConstraints(t *testing.T) {
	ops := []string{"Init", "Add", "Sub", "Mul", "Div", "Sqrt", "Cmp"}

	var result_all strings.Builder
	result_all.WriteString("Type, T_RC")
	for _, op := range ops {
		result_all.WriteString(", " + op)
	}
	result_all.WriteString("\n")

	for _, param := range params {
		for _, size := range []uint{8, 12, 16} {

			result_all.WriteString(param.name + ", ")
			result_all.WriteString(fmt.Sprint(size) + ", ")

			circuit := &F32Circuit{E: param.E, M: param.M, size: size}
			_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
			if err != nil {
				t.Fatal(err)
			}

			for i, op := range ops {
				c := circuit.result[i]
				result_all.WriteString(fmt.Sprint(c.native) + " + " + fmt.Sprint(c.lookup_query) + " + " + fmt.Sprint(c.lookup_global) + "/n")
				if i != len(circuit.result)-1 {
					result_all.WriteString(", ")
				} else {
					result_all.WriteString("\n")
				}
				var result_op strings.Builder
				result_op.WriteString("#Operations, #Constraints (Total), #Constraints (Amortized)\n")
				for j := 0; j < 16; j++ {
					n := uint(1 << j)
					result_op.WriteString(fmt.Sprint(n) + ", ")
					result_op.WriteString(fmt.Sprint(n*(c.native + c.lookup_query) + c.lookup_global) + ", ")
					result_op.WriteString(fmt.Sprint((n*(c.native + c.lookup_query) + c.lookup_global) / n) + "\n")
				}
				err := os.WriteFile(filepath.Join(basepath, fmt.Sprintf("../benchmarks/float/%s/constraints_%s (T_RC = %d).csv", strings.ToLower(param.name), strings.ToLower(op), size)), []byte(result_op.String()), 0644)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	err := os.WriteFile(filepath.Join(basepath, "../benchmarks/float/constraints.csv"), []byte(result_all.String()), 0644)
	if err != nil {
        t.Fatal(err)
    }
}
