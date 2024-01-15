package loc2index64

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
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"

	"github.com/consensys/gnark/test"
)

type loc2Index32Circuit struct {

	// SECRET INPUTS
	Lat frontend.Variable `gnark:",secret"`
	Lng frontend.Variable `gnark:",secret"`

	// PUBLIC INPUTS
	Resolution frontend.Variable `gnark:",public"`
	I          frontend.Variable `gnark:",public"`
	J          frontend.Variable `gnark:",public"`
	K          frontend.Variable `gnark:",public"`
}

func (c *loc2Index32Circuit) Define(api frontend.API) error {

	ctx := float.NewContext(api, util.IEEE32ExponentBitwidth, util.IEEE32Precision)
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

func TestLoc2Index32(t *testing.T) {
	assert := test.NewAssert(t)

	file, err := os.Open("../data/f32/loc2index32.txt")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.Fields(scanner.Text())

		lat, _ := new(big.Int).SetString(data[0], 16)
		lng, _ := new(big.Int).SetString(data[1], 16)
		res, _ := new(big.Int).SetString(data[2], 16)
		i, _ := new(big.Int).SetString(data[3], 16)
		j, _ := new(big.Int).SetString(data[4], 16)
		k, _ := new(big.Int).SetString(data[5], 16)

		fmt.Printf("lat: %f, lng: %f\n", lat, lng)
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Calculate I, J, K using the H3 library in C - just for comparison and debugging
		latFloat, _ := util.HexToFloat(data[0])
		lngFloat, _ := util.HexToFloat(data[1])
		fmt.Printf("latFloat: %f, lngFloat: %f\n", latFloat, lngFloat)
		iInt, jInt, kInt, err := util.ExecuteLatLngToIJK(15, latFloat, lngFloat)
		if err != nil {
			panic(err)
		}
		fmt.Printf("i: %d, j: %d, k: %d\n", iInt, jInt, kInt)

		assert.ProverSucceeded(
			&loc2Index32Circuit{Lat: 0, Lng: 0, Resolution: 0},
			&loc2Index32Circuit{Lat: lat, Lng: lng, Resolution: res, I: i, J: j, K: k},
			test.WithCurves(ecc.BN254),
			test.WithBackends(backend.GROTH16),
		)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
}
