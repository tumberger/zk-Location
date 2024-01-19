package float

import (
	"math"
	"math/big"
)

func ComponentsOf(v uint64, E, M uint64) []*big.Int {
	s := v >> (E + M)
	e := (v >> M) - (s << E)
	m := v - (s << (E + M)) - (e << M)

	sign := big.NewInt(int64(s))

	exponent_max := big.NewInt(1 << (E - 1))
	exponent_min := new(big.Int).Sub(big.NewInt(1), exponent_max)

	exponent := new(big.Int).Add(big.NewInt(int64(e)), exponent_min)

	mantissa_is_not_zero := m != 0
	exponent_is_min := exponent.Cmp(exponent_min) == 0
	exponent_is_max := exponent.Cmp(exponent_max) == 0

	mantissa := big.NewInt(int64(m))
	shift := uint(0)
	for i := int(M - 1); i >= 0; i-- {
		if mantissa.Bit(i) != 0 {
			break
		}
		shift++
	}

	shifted_mantissa := new(big.Int).Lsh(new(big.Int).Set(mantissa), shift)

	if exponent_is_min {
		exponent = new(big.Int).Sub(exponent, big.NewInt(int64(shift)))
		mantissa = new(big.Int).Lsh(shifted_mantissa, 1)
	} else {
		if exponent_is_max && mantissa_is_not_zero {
			mantissa.SetUint64(0)
		} else {
			mantissa = new(big.Int).Add(mantissa, big.NewInt(1<<M))
		}
	}

	is_abnormal := big.NewInt(0)
	if exponent_is_max {
		is_abnormal.SetUint64(1)
	}

	return []*big.Int{sign, exponent, mantissa, is_abnormal}
}

func ValueOf(components []*big.Int, E, M uint64) uint64 {
	s := components[0].Uint64()
	e := new(big.Int).Add(components[1], big.NewInt(int64((1 << (E - 1)) - 1 + M))).Uint64()
	m := components[2].Uint64()
	is_abnormal := components[3].Uint64() == 1

	if e <= M {
		if is_abnormal || (e == 0) != (m == 0) {
			panic("")
		}
		delta := M + 1 - e
		if (m>>delta)<<delta != m {
			panic("")
		}
		return (s << (M + E)) + (m >> delta)
	} else {
		e = e - M
		if (e == (1<<E)-1) != is_abnormal {
			panic("")
		}
		if is_abnormal && m == 0 {
			m = 1
		} else {
			m = m - (1 << M)
		}
		return (s << (M + E)) + (e << M) + m
	}
}

func F32ToComponents(v float32) []*big.Int {
	return ComponentsOf(uint64(math.Float32bits(v)), 8, 23)
}

func F64ToComponents(v float64) []*big.Int {
	return ComponentsOf(math.Float64bits(v), 11, 52)
}

func ComponentsToF32(components []*big.Int) float32 {
	return math.Float32frombits(uint32(ValueOf(components, 8, 23)))
}

func ComponentsToF64(components []*big.Int) float64 {
	return math.Float64frombits(ValueOf(components, 11, 52))
}
