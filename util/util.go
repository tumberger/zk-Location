package util

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/uber/h3-go/v4"
)

// ToDo - Add the logic
var (
	// Mathematical constants
	IEEE32ExponentBitwidth int               = 8
	IEEE32Precision        int               = 23
	BaseM                  int               = 8388608
	PiE                    int               = 1
	PiM                    int               = 13176795
	DoublePiE              int               = 2 // Correct?
	HalfPiE                int               = 0
	ThreeHalfPieE          int               = 2
	ThreeHalfPieM          int               = 9882596
	ThirdsM                int               = 11184811
	ResConstE              int               = -2
	ResConstM              int               = 12816653
	Ap7rotE                int               = -2
	Ap7rotM                int               = 11189503
	MaxResolution          frontend.Variable = 15
)

var Sqrt7scale = [30]int{
	// 0, 8388608,
	1, 11097086,
	2, 14680064,
	4, 9709950,
	5, 12845056,
	7, 8496206,
	8, 11239424,
	9, 14868360,
	11, 9834496,
	12, 13009816,
	14, 8605184,
	15, 11383588,
	16, 15059072,
	18, 9960640,
	19, 13176688,
	21, 8715560}

var faceCenterGeo = [120]int{
	0, -1, 13481880, 0, 0, 10472316, // face 0
	0, 0, 10970184, 0, 1, 10640718, // face 1
	0, 0, 8847894, 1, 0, 11303794, // face 2
	0, -1, 10069544, 1, -2, 15119758, // face 3
	0, -2, 16499232, 0, -2, 13488486, // face 4
	0, -3, 11592742, 0, 0, 14077316, // face 5
	0, -1, 10165808, 0, 1, 12389652, // face 6
	0, -2, 14340174, 1, 0, 15845042, // face 7
	1, -4, 10612074, 1, -1, 12304906, // face 8
	1, -3, 15499574, 0, -1, 8497586, // face 9
	0, -4, 10612074, 0, 1, 10100568, // face 10
	0, -3, 15499574, 1, 1, 11052398, // face 11
	1, -3, 11592742, 1, 0, 12276272, // face 12
	1, -1, 10165808, 1, -3, 12594276, // face 13
	1, -2, 14340174, 0, 0, 10508548, // face 14
	1, -1, 10069544, 1, 1, 11286824, // face 15
	1, -2, 16499232, 1, 1, 11490734, // face 16
	1, -1, 13481880, 1, 0, 15881272, // face 17
	1, 0, 10970184, 1, -1, 10144304, // face 18
	1, 0, 8847894, 0, 0, 15049794} // face 19

var faceCenterPoint = [180]int{
	0, -3, 14759304, 0, -1, 11045602, 0, -1, 12077038,
	1, -3, 14356162, 0, -3, 9919844, 0, -1, 16200110,
	0, -4, 14664968, 1, -2, 16146230, 0, -1, 14592446,
	0, -1, 12463068, 1, -2, 12059268, 0, -1, 9475782,
	0, -1, 13610574, 0, -2, 11572766, 0, -2, 15842348,
	1, -4, 14166656, 0, -1, 16432372, 0, -3, 11535172,
	1, -1, 13548286, 0, -3, 10291496, 0, -1, 9555064,
	1, -2, 9550088, 1, -1, 14502362, 0, -2, 13907616,
	0, -1, 12424572, 1, -1, 11195938, 1, -4, 10601022,
	0, -1, 14281276, 0, -2, 15845556, 1, -3, 15362140,
	1, -1, 12424572, 0, -1, 11195938, 0, -4, 10601022,
	1, -1, 14281276, 1, -2, 15845556, 0, -3, 15362140,
	0, -4, 14166656, 1, -1, 16432372, 1, -3, 11535172,
	0, -1, 13548286, 1, -3, 10291496, 1, -1, 9555064,
	0, -2, 9550088, 0, -1, 14502362, 1, -2, 13907616,
	1, -1, 12463068, 0, -2, 12059268, 1, -1, 9475782,
	1, -1, 13610574, 1, -2, 11572766, 1, -2, 15842348,
	1, -3, 14759304, 1, -1, 11045602, 1, -1, 12077038,
	0, -3, 14356162, 1, -3, 9919844, 1, -1, 16200110,
	1, -4, 14664968, 0, -2, 16146230, 1, -1, 14592446}

var cosFaceLat = [60]int{
	0, -1, 11645606,
	0, -2, 8725002,
	0, -2, 16557236,
	0, -1, 13845018,
	0, -1, 14789522,
	0, -1, 16527514,
	0, -1, 13790422,
	0, -1, 15268252,
	1, -1, 16724802,
	1, -1, 16331726,
	0, -1, 16724802,
	0, -1, 16331726,
	1, -1, 16527514,
	1, -1, 13790422,
	1, -1, 15268252,
	1, -1, 13845018,
	1, -1, 14789522,
	1, -1, 11645606,
	1, -2, 8725002,
	1, -2, 16557236}

var sinFaceLat = [60]int{
	0, -1, 12077038,
	0, -1, 16200110,
	0, -1, 14592446,
	0, -1, 9475782,
	0, -2, 15842348,
	0, -3, 11535172,
	0, -1, 9555064,
	0, -2, 13907616,
	1, -4, 10601022,
	1, -3, 15362140,
	0, -4, 10601022,
	0, -3, 15362140,
	1, -3, 11535172,
	1, -1, 9555064,
	1, -2, 13907616,
	1, -1, 9475782,
	1, -2, 15842348,
	1, -1, 12077038,
	1, -1, 16200110,
	1, -1, 14592446}

var azimuth = [60]int{
	0, -1, 10329110,
	0, -2, 16755342,
	0, -1, 11801618,
	0, -2, 14002176,
	0, -3, 10222084,
	0, -2, 14556182,
	0, -3, 10600866,
	0, -2, 12797940,
	0, -2, 11591192,
	0, -3, 9256814,
	0, -2, 11591192,
	0, -3, 9256814,
	0, -2, 14556182,
	0, -3, 10600866,
	0, -2, 12797940,
	0, -2, 14002176,
	0, -3, 10222084,
	0, -1, 10329110,
	0, -2, 16755342,
	0, -1, 11801618,
}

const ScaleFactor = 1e9

// Convert scaled integer to float64 assuming the scale is 1e9
func ScaledIntToFloat64(scaledInt int) float64 {
	return float64(scaledInt) / ScaleFactor
}

// radiansToDegrees converts radians to degrees.
func RadiansToDegrees(rad float64) float64 {
	return rad * (180.0 / math.Pi)
}

func ExecuteLatLngToIJK(resolution int, latitude float64, longitude float64) (int, int, int, error) {
	// Convert float64 latitude and longitude to string
	latStr := fmt.Sprintf("%f", latitude)
	lngStr := fmt.Sprintf("%f", longitude)
	resStr := strconv.Itoa(resolution)

	// Define the path to the executable
	executablePath := "../h3-master/build/bin/latLngToCell"

	// Define the command and arguments using the correct path
	cmd := exec.Command(executablePath, "--resolution", resStr, "--latitude", latStr, "--longitude", lngStr)

	// Run the command and capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, 0, err
	}

	// Define a regex pattern to find I, J, K values
	pattern := `I: (\d+), J: (\d+), K: (\d+)`
	re := regexp.MustCompile(pattern)

	// Find matches in the command output
	matches := re.FindStringSubmatch(string(output))
	if matches == nil || len(matches) != 4 {
		return 0, 0, 0, fmt.Errorf("failed to parse output")
	}

	// Convert matched strings to integers
	i, _ := strconv.Atoi(matches[1])
	j, _ := strconv.Atoi(matches[2])
	k, _ := strconv.Atoi(matches[3])

	return i, j, k, nil
}

// The following function translates to local IJ coordinates within the proximity of a given origin
// (not for gloabl use)
func LatLngToIJ(lat float64, lng float64, resolution int, origin h3.Cell) (I int, J int) {
	// Create a new LatLng struct
	latLng := h3.NewLatLng(lat, lng)

	// Convert LatLng to H3 cell
	cell := h3.LatLngToCell(latLng, resolution)

	// Convert H3 cell to local IJ coordinates
	coordIJ := h3.CellToLocalIJ(origin, cell)

	return coordIJ.I, coordIJ.J
}

func StrToIntSlice(inputData string, hexRepresentation bool) []int {

	// check if inputData in hex representation
	var byteSlice []byte
	if hexRepresentation {
		hexBytes, err := hex.DecodeString(inputData)
		if err != nil {
			log.Error().Msg("hex.DecodeString error.")
		}
		byteSlice = hexBytes
	} else {
		byteSlice = []byte(inputData)
	}

	// convert byte slice to int numbers which can be passed to gnark frontend.Variable
	var data []int
	for i := 0; i < len(byteSlice); i++ {
		data = append(data, int(byteSlice[i]))
	}

	return data
}

// compressThreshold --> if linear expressions are larger than this, the frontend will introduce
// intermediate constraints. The lower this number is, the faster compile time should be (to a point)
// but resulting circuit will have more constraints (slower proving time).
const compressThreshold = 100

func BenchProof(b *testing.B, circuit, assignment frontend.Circuit) {
	fmt.Println("compiling...")
	start := time.Now().UnixMicro()
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit, frontend.WithCompressThreshold(compressThreshold))
	require.NoError(b, err)
	// Print the number of constraints
	fmt.Println("Number of constraints:", cs.GetNbConstraints())
	fmt.Println("compiled in", time.Now().UnixMicro()-start, "μs")
	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	require.NoError(b, err)
	//publicWitness := fullWitness.Public()
	fmt.Println("setting up...")
	pk, _, err := groth16.Setup(cs)
	require.NoError(b, err)

	fmt.Println("solving and proving...")
	b.ResetTimer()

	b.N = 20

	for i := 0; i < b.N; i++ {
		id := rand.Uint32() % 256 //#nosec G404 -- This is a false positive
		start = time.Now().UnixMicro()
		fmt.Println("groth16 proving", id)
		_, err = groth16.Prove(cs, pk, fullWitness)
		require.NoError(b, err)
		fmt.Println("groth16 proved", id, "in", time.Now().UnixMicro()-start, "μs")

		// fmt.Println("mimc total calls: fr=", mimcFrTotalCalls, ", snark=", mimcSnarkTotalCalls)
	}
}
