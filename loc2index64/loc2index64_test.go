package loc2index64

import (
	"encoding/json"
	"testing"
	"math"
	"fmt"
	//"time"
	//"math/rand"

	util "gnark-float/util"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	//"github.com/consensys/gnark/backend/groth16"
	//"github.com/consensys/gnark/backend/plonk"
	//"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/frontend/cs/r1cs"
	//"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

type loc2Index64Params struct {
	LatS       int `json:"LatS,string"`
	LatE       int `json:"LatE,string"`
	LatM       int `json:"LatM,string"`
	LatA       int `json:"LatA,string"`
	LngS       int `json:"LngS,string"`
	LngE       int `json:"LngE,string"`
	LngM       int `json:"LngM,string"`
	LngA       int `json:"LngA,string"`
	Resolution int `json:"Resolution,string"`
}

const loc2Index64 = `{
    "LatS": "1",
	"LatE": "0",
	"LatM": "4527589427376396",
	"LatA": "0",
    "LngS": "0",
	"LngE": "0",
	"LngM": "5370059476281135",
	"LngA": "0"
	"Resolution": "0"
}`

// Common setup function for both tests
func setupLoc2IndexWrapper() (loc2Index64Wrapper, loc2Index64Wrapper) {
	var data loc2Index64Params
	err := json.Unmarshal([]byte(loc2Index64), &data)
	if err != nil {
		panic(err)
	}

	// Convert the scaled integers to float64 for latitude and longitude
	// lat := util.ScaledIntToFloat64(data.Lat)
	// if data.LatIsNegative == 1 {
	// 	lat = -lat
	// }
	// lng := util.ScaledIntToFloat64(data.Lng)
	// if data.LngIsNegative == 1 {
	// 	lng = -lng
	// }

	// Latitude: 840,096,782 (in radians ×109×109)
	// Longitude: 202,144,034 (in radians ×109×109) ​
	//var lat float64 = 840096782
	//var lng float64 = 202144034
	
	lat := math.Pow(2, float64(data.LatE)) * (float64(data.LatM) / math.Pow(2, float64(52)))
	if data.LatS == 1 {
		lat = -lat
	}
	lng := math.Pow(2, float64(data.LngE)) * (float64(data.LngM) / math.Pow(2, float64(52)))
	if data.LngS == 1 {
		lng = -lng
	}
	
	fmt.Printf("lat, lng: %f, %f\n", lat, lng)

	// Calculate I, J, K using the H3 library in C
	i, j, k, err := util.ExecuteLatLngToIJK(data.Resolution, util.RadiansToDegrees(lat), util.RadiansToDegrees(lng))
	if err != nil {
		panic(err)
	}

	// Update witness values with calculated I, J, K
	assignment := loc2Index64Wrapper{
		LatS:       data.LatS,
		LatE:       data.LatE,
		LatM:       data.LatM,
		LatA:		data.LatA,
		LngS:       data.LngS,
		LngE:       data.LngE,
		LngM:       data.LngM,
		LngA:		data.LngA,
		I:          i,
		J:          j,
		K:          k,
		Resolution: data.Resolution,
	}

	circuit := loc2Index64Wrapper{
		// The circuit does not need actual values for I, J, K since these are
		// calculated within the circuit itself when running the proof or solving
		LatS:       data.LatS,
		LatE:       data.LatE,
		LatM:       data.LatM,
		LatA:		data.LatA,
		LngS:       data.LngS,
		LngE:       data.LngE,
		LngM:       data.LngM,
		LngA:		data.LngA,
		Resolution: data.Resolution,
	}

	// println("%d", assignment)

	return circuit, assignment
}

const maxResolution = 15

func FuzzLoc2Index(f *testing.F) {
	// Seed the random number generator
	//rand.Seed(time.Now().UnixNano())

	f.Add(0, -2, 8273112515479601, 0, 0, 0, 5961414826750325, 0, 12)
	f.Add(1, 0, 4610334938539177, 0, 0, 1, 6610882487532388, 0, 13)
	f.Add(0, -1, 6809442636584190, 0, 1, -1, 7359782511048864, 0, 14)
	f.Add(0, -2, 8273112515479601, 0, 0, 1, 5961414826750325, 0, 15)
	f.Add(1, 0, 4610334938539177, 0, 0, -2, 6610882487532388, 0, 13)
	f.Add(0, -2, 6809442636584190, 0, 1, -2, 7359782511048864, 0, 15)
	f.Add(0, -3, 8273112515479601, 0, 0, -1, 5961414826750325, 0, 15)
	f.Add(1, -1, 4610334938539177, 0, 0, 1, 6610882487532388, 0, 14)
	f.Add(0, 0, 6809442636584190, 0, 1, 0, 7359782511048864, 0, 15)

	f.Fuzz(func(t *testing.T, latS, latE, latM, latA, lngS, lngE, lngM, lngA, res int) {

		/// Skip the test if signs are neither 0 nor 1
		if latS != 0 && latS != 1 {
			t.Skip("Invalid latS value, skipping fuzz test")
		}
		if lngS != 0 && lngS != 1 {
			t.Skip("Invalid lngS value, skipping fuzz test")
		}
		if latA != 0 && latA != 1 {
			t.Skip("Invalid latA value, skipping fuzz test")
		}
		if lngA != 0 && lngA != 1 {
			t.Skip("Invalid lngA value, skipping fuzz test")
		}

		// Skip the test if the resolution is negative
		if res < 0 || res > maxResolution {
			t.Skip("Negative resolution, skipping fuzz test")
		}

		// If the values are valid, print all inputs
		// Print all inputs with fmt.Printf
		fmt.Printf("latS %d, latE %d, latM %d, latA %d, lngS %d, lngE %d, lngM %d, lngA %d, resolution: %d\n",
			latS, latE, latM, latA, lngS, lngE, lngM, lngA, res)

		// Convert to float64 for latitude and longitude
		latFloat := math.Pow(2, float64(latE)) * (float64(latM) / math.Pow(2, float64(52)))
		if latS == 1 {
			latFloat = -latFloat
		}
		lngFloat := math.Pow(2, float64(lngE)) * (float64(lngM) / math.Pow(2, float64(52)))
		if lngS == 1 {
			lngFloat = -lngFloat
		}
		
		fmt.Printf("latFloat: %f, lngFloat: %f, res: %d\n", latFloat, lngFloat, res)

		// Convert radians to degrees and check their ranges
		latDegrees := util.RadiansToDegrees(latFloat)
		lngDegrees := util.RadiansToDegrees(lngFloat)
		if latDegrees < -90 || latDegrees > 90 {
			t.Skip("Latitude out of valid degree range")
		}
		if lngDegrees < -180 || lngDegrees > 180 {
			t.Skip("Longitude out of valid degree range")
		}
		
		fmt.Printf("lat in degrees: %f, lng in degrees: %f\n", latDegrees, lngDegrees)

		// Calculate I, J, K using the H3 library in C
		i, j, k, _ := util.ExecuteLatLngToIJK(res, latDegrees, lngDegrees)
		// if err != nil {
		// 	t.Fatalf("Failed to calculate IJK: %v", err)
		// }

		// Print the calculated I, J, K values
		fmt.Printf("i: %d, j: %d, k: %d\n", i, j, k)

		// Update witness values with calculated I, J, K
		assignment := loc2Index64Wrapper{
			LatS:           latS,
			LatE:			latE,
			LatM:			latM,
			LatA:			latA,
			LngS:           lngS,
			LngE: 			lngE,
			LngM:			lngM,
			LngA:			lngA,
			I:             	i,
			J:             	j,
			K:             	k,
			Resolution:    	res,
		}

		circuit := loc2Index64Wrapper{
			// The circuit does not need actual values for I, J, K since these are
			// calculated within the circuit itself when running the proof or solving
			LatS:           latS,
			LatE:			latE,
			LatM:			latM,
			LngS:           lngS,
			LngE: 			lngE,
			LngM:			lngM,
			Resolution:    	res,
		}

		// Perform assertions using the assert object from the 'test' package
		assert := test.NewAssert(t)
		assert.SolvingSucceeded(&circuit, &assignment, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
		// You can add more assertions as necessary
	})
}

func TestLoc2IndexSolving(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, assignment := setupLoc2IndexWrapper()

	// Solve the circuit and assert.
	assert.SolvingSucceeded(&circuit, &assignment, test.WithBackends(backend.GROTH16))
}

func TestLoc2IndexProving(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, assignment := setupLoc2IndexWrapper()

	// Proof successfully generated
	assert.ProverSucceeded(&circuit, &assignment, test.WithBackends(backend.GROTH16))
}
