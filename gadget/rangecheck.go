package gadget

import (
	"fmt"
	"math"
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/logderivarg"
	"github.com/consensys/gnark/frontend/frontendtype"
)

type Mode int

const (
	Loose Mode = iota
    TightForSmallAbs Mode = iota
    TightForUnknownRange Mode = iota
)

func init() {
	solver.RegisterHint(DecomposeHint)
}

type checkedVariable struct {
	v    frontend.Variable
	bits int
	mode Mode
}

type CommitChecker struct {
	collected []checkedVariable
	closed    bool
	size      int
}

func NewCommitRangechecker(api frontend.API, size int) *CommitChecker {
	cht := &CommitChecker{size: size}
	api.Compiler().Defer(cht.commit)
	return cht
}

func (c *CommitChecker) Check(in frontend.Variable, bits int, mode Mode) {
	if c.closed {
		panic("checker already closed")
	}
	c.collected = append(c.collected, checkedVariable{v: in, bits: bits, mode: mode})
}

func (c *CommitChecker) buildTable(nbTable int) []frontend.Variable {
	tbl := make([]frontend.Variable, nbTable)
	for i := 0; i < nbTable; i++ {
		tbl[i] = i
	}
	return tbl
}

func (c *CommitChecker) commit(api frontend.API) error {
	if c.closed {
		return nil
	}
	defer func() { c.closed = true }()
	if len(c.collected) == 0 {
		return nil
	}
	baseLength := c.size
	if baseLength <= 0 {
		baseLength = c.getOptimalBasewidth(api)
	}
	// decompose into smaller limbs
	decomposed := make([]frontend.Variable, 0, len(c.collected))
	coef := new(big.Int)
	one := big.NewInt(1)
	for i := range c.collected {
		// decompose value into limbs
		nbLimbs := decompSize(c.collected[i].bits, baseLength)
		diff := nbLimbs * baseLength - c.collected[i].bits

		if nbLimbs == 1 {
			if c.collected[i].mode == TightForSmallAbs {
				decomposed = append(decomposed, api.Mul(c.collected[i].v, coef.Lsh(one, uint(diff))))
			} else {
				decomposed = append(decomposed, c.collected[i].v)
			}
	
			if c.collected[i].mode == TightForUnknownRange && diff != 0 {
				decomposed = append(decomposed, api.Mul(c.collected[i].v, coef.Lsh(one, uint(diff))))
			}
		} else {
			scale := big.NewInt(1)
			if c.collected[i].mode == TightForSmallAbs {
				scale.Lsh(one, uint(diff))
			}
			limbs, err := api.Compiler().NewHint(DecomposeHint, int(nbLimbs), c.collected[i].bits, baseLength, c.collected[i].v, scale)
			if err != nil {
				panic(fmt.Sprintf("decompose %v", err))
			}
			// store all limbs for counting
			decomposed = append(decomposed, limbs...)
			// check that limbs are correct. We check the sizes of the limbs later
			var composed frontend.Variable = 0
			for j := range limbs {
				composed = api.Add(composed, api.Mul(limbs[j], coef.Lsh(one, uint(baseLength*j))))
			}
			if c.collected[i].mode == TightForSmallAbs {
				api.AssertIsEqual(api.Mul(c.collected[i].v, scale), composed)
			} else {
				api.AssertIsEqual(c.collected[i].v, composed)
			}

			if c.collected[i].mode == TightForUnknownRange && diff != 0 {
				msLimbShifted := api.Mul(limbs[nbLimbs-1], coef.Lsh(one, uint(diff)))
				decomposed = append(decomposed, msLimbShifted)
			}
		}
	}
	nbTable := 1 << baseLength
	return logderivarg.Build(api, logderivarg.AsTable(c.buildTable(nbTable)), logderivarg.AsTable(decomposed))
}

func decompSize(varSize int, limbSize int) int {
	return (varSize + limbSize - 1) / limbSize
}

// DecomposeHint is a hint used for range checking with commitment. It
// decomposes large variables into chunks which can be individually range-check
// in the native range.
func DecomposeHint(m *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	if len(inputs) != 4 {
		return fmt.Errorf("input must be 4 elements")
	}
	if !inputs[0].IsUint64() || !inputs[1].IsUint64() {
		return fmt.Errorf("first two inputs have to be uint64")
	}
	varSize := int(inputs[0].Int64())
	limbSize := int(inputs[1].Int64())
	val := inputs[2]
	scale := inputs[3]
	nbLimbs := decompSize(varSize, limbSize)
	if len(outputs) != nbLimbs {
		return fmt.Errorf("need %d outputs instead to decompose", nbLimbs)
	}
	base := new(big.Int).Lsh(big.NewInt(1), uint(limbSize))
	tmp := new(big.Int).Mul(val, scale)
	for i := 0; i < len(outputs); i++ {
		outputs[i].Mod(tmp, base)
		tmp.Rsh(tmp, uint(limbSize))
	}
	return nil
}

func (c *CommitChecker) getOptimalBasewidth(api frontend.API) int {
	if ft, ok := api.(frontendtype.FrontendTyper); ok {
		switch ft.FrontendType() {
		case frontendtype.R1CS:
			return optimalWidth(nbR1CSConstraints, c.collected)
		case frontendtype.SCS:
			return optimalWidth(nbPLONKConstraints, c.collected)
		}
	}
	return optimalWidth(nbR1CSConstraints, c.collected)
}

func optimalWidth(countFn func(baseLength int, collected []checkedVariable) int, collected []checkedVariable) int {
	min := math.MaxInt64
	minVal := 0
	for j := 2; j < 18; j++ {
		current := countFn(j, collected)
		if current < min {
			min = current
			minVal = j
		}
	}
	return minVal
}

func nbR1CSConstraints(baseLength int, collected []checkedVariable) int {
	nbDecomposed := 0
	for i := range collected {
		nbVarLimbs := int(decompSize(collected[i].bits, baseLength))
		if nbVarLimbs*baseLength > collected[i].bits {
			nbVarLimbs += 1
		}
		nbDecomposed += int(nbVarLimbs)
	}
	eqs := len(collected)       // correctness of decomposition
	nbRight := nbDecomposed     // inverse per decomposed
	nbleft := (1 << baseLength) // div per table
	return nbleft + nbRight + eqs + 1
}

func nbPLONKConstraints(baseLength int, collected []checkedVariable) int {
	nbDecomposed := 0
	for i := range collected {
		nbVarLimbs := int(decompSize(collected[i].bits, baseLength))
		if nbVarLimbs*baseLength > collected[i].bits {
			nbVarLimbs += 1
		}
		nbDecomposed += int(nbVarLimbs)
	}
	eqs := nbDecomposed               // check correctness of every decomposition. this is nbDecomp adds + eq cost per collected
	nbRight := 3 * nbDecomposed       // denominator sub, inv and large sum per table entry
	nbleft := 3 * (1 << baseLength)   // denominator sub, div and large sum per table entry
	return nbleft + nbRight + eqs + 1 // and the final assert
}
