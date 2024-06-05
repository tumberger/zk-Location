package util

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/uber/h3-go/v4"
)

var (
	// Mathematical constants
	IEEE32ExponentBitwidth uint    = 8
	IEEE32Precision        uint    = 23
	IEEE64ExponentBitwidth uint    = 11
	IEEE64Precision        uint    = 52
	Sin60_64               float64 = 0.8660254037844386467637231707529361834714
	Sqrt7_64               float64 = 2.6457513110645905905016157536392604257102
	ResConst_64            float64 = 0.38196601125010500003
	Ap7rot_64              float64 = 0.333473172251832115336090755351601070065900389
	Sin60_32               float32 = 0.8660254037844386467637231707529361834714
	Sqrt7_32               float32 = 2.6457513110645905905016157536392604257102
	ResConst_32            float32 = 0.38196601125010500003
	Ap7rot_32              float32 = 0.333473172251832115336090755351601070065900389

	MaxResolution frontend.Variable = 15
)

var FaceCenterGeoLng_64 = [20]float64{

	1.248397419617396099,  // face  0
	2.536945009877921159,  // face  1
	-1.347517358900396623, // face  2
	-0.450603909469755746, // face  3
	0.401988202911306943,  // face  4
	1.678146885280433686,  // face  5
	2.953923329812411617,  // face  6
	-1.888876200336285401, // face  7
	-0.733429513380867741, // face  8
	0.506495587332349035,  // face  9
	2.408163140208925497,  // face 10
	-2.635097066257444203, // face 11
	-1.463445768309359553, // face 12
	-0.187669323777381622, // face 13
	1.252716453253507838,  // face 14
	2.690988744120037492,  // face 15
	-2.739604450678486295, // face 16
	-1.893195233972397139, // face 17
	-0.604647643711872080, // face 18
	1.794075294689396615,  // face 19
}

var FaceCenterGeoLng_32 = [20]float32{

	1.248397419617396099,  // face  0
	2.536945009877921159,  // face  1
	-1.347517358900396623, // face  2
	-0.450603909469755746, // face  3
	0.401988202911306943,  // face  4
	1.678146885280433686,  // face  5
	2.953923329812411617,  // face  6
	-1.888876200336285401, // face  7
	-0.733429513380867741, // face  8
	0.506495587332349035,  // face  9
	2.408163140208925497,  // face 10
	-2.635097066257444203, // face 11
	-1.463445768309359553, // face 12
	-0.187669323777381622, // face 13
	1.252716453253507838,  // face 14
	2.690988744120037492,  // face 15
	-2.739604450678486295, // face 16
	-1.893195233972397139, // face 17
	-0.604647643711872080, // face 18
	1.794075294689396615,  // face 19
}

var FaceCenterPoint_64 = [60]float64{

	0.2199307791404606, 0.6583691780274996, 0.7198475378926182, // face  0
	-0.2139234834501421, 0.1478171829550703, 0.9656017935214205, // face  1
	0.1092625278784797, -0.4811951572873210, 0.8697775121287253, // face  2
	0.7428567301586791, -0.3593941678278028, 0.5648005936517033, // face  3
	0.8112534709140969, 0.3448953237639384, 0.4721387736413930, // face  4
	-0.1055498149613921, 0.9794457296411413, 0.1718874610009365, // face  5
	-0.8075407579970092, 0.1533552485898818, 0.5695261994882688, // face  6
	-0.2846148069787907, -0.8644080972654206, 0.4144792552473539, // face  7
	0.7405621473854482, -0.6673299564565524, -0.0789837646326737, // face  8
	0.8512303986474293, 0.4722343788582681, -0.2289137388687808, // face  9
	-0.7405621473854481, 0.6673299564565524, 0.0789837646326737, // face 10
	-0.8512303986474292, -0.4722343788582682, 0.2289137388687808, // face 11
	0.1055498149613919, -0.9794457296411413, -0.1718874610009365, // face 12
	0.8075407579970092, -0.1533552485898819, -0.5695261994882688, // face 13
	0.2846148069787908, 0.8644080972654204, -0.4144792552473539, // face 14
	-0.7428567301586791, 0.3593941678278027, -0.5648005936517033, // face 15
	-0.8112534709140971, -0.3448953237639382, -0.4721387736413930, // face 16
	-0.2199307791404607, -0.6583691780274996, -0.7198475378926182, // face 17
	0.2139234834501420, -0.1478171829550704, -0.9656017935214205, // face 18
	-0.1092625278784796, 0.4811951572873210, -0.8697775121287253, // face 19
}

var FaceCenterPoint_32 = [60]float32{

	0.2199307791404606, 0.6583691780274996, 0.7198475378926182, // face  0
	-0.2139234834501421, 0.1478171829550703, 0.9656017935214205, // face  1
	0.1092625278784797, -0.4811951572873210, 0.8697775121287253, // face  2
	0.7428567301586791, -0.3593941678278028, 0.5648005936517033, // face  3
	0.8112534709140969, 0.3448953237639384, 0.4721387736413930, // face  4
	-0.1055498149613921, 0.9794457296411413, 0.1718874610009365, // face  5
	-0.8075407579970092, 0.1533552485898818, 0.5695261994882688, // face  6
	-0.2846148069787907, -0.8644080972654206, 0.4144792552473539, // face  7
	0.7405621473854482, -0.6673299564565524, -0.0789837646326737, // face  8
	0.8512303986474293, 0.4722343788582681, -0.2289137388687808, // face  9
	-0.7405621473854481, 0.6673299564565524, 0.0789837646326737, // face 10
	-0.8512303986474292, -0.4722343788582682, 0.2289137388687808, // face 11
	0.1055498149613919, -0.9794457296411413, -0.1718874610009365, // face 12
	0.8075407579970092, -0.1533552485898819, -0.5695261994882688, // face 13
	0.2846148069787908, 0.8644080972654204, -0.4144792552473539, // face 14
	-0.7428567301586791, 0.3593941678278027, -0.5648005936517033, // face 15
	-0.8112534709140971, -0.3448953237639382, -0.4721387736413930, // face 16
	-0.2199307791404607, -0.6583691780274996, -0.7198475378926182, // face 17
	0.2139234834501420, -0.1478171829550704, -0.9656017935214205, // face 18
	-0.1092625278784796, 0.4811951572873210, -0.8697775121287253, // face 19
}

var CosFaceLat_64 = [20]float64{

	0.694132208005028062,
	0.260025337896551800,
	0.493444099564646799,
	0.825227416783206214,
	0.881524235868986983,
	0.985116592465405394,
	0.821973179669780118,
	0.910058760174088377,
	0.996875902469535280,
	0.973446711513843321,
	0.996875902469535280,
	0.973446711513843321,
	0.985116592465405394,
	0.821973179669780118,
	0.910058760174088377,
	0.825227416783206214,
	0.881524235868986983,
	0.694132208005028062,
	0.260025337896551800,
	0.493444099564646799,
}

var SinFaceLat = [20]float64{

	0.719847537892618239,
	0.965601793521420504,
	0.869777512128725339,
	0.564800593651703320,
	0.472138773641393006,
	0.171887461000936548,
	0.569526199488268769,
	0.414479255247353850,
	-0.078983764632673703,
	-0.228913738868780775,
	0.078983764632673703,
	0.228913738868780775,
	-0.171887461000936548,
	-0.569526199488268769,
	-0.414479255247353850,
	-0.564800593651703320,
	-0.472138773641393006,
	-0.719847537892618239,
	-0.965601793521420504,
	-0.869777512128725339,
}

var Azimuth = [20]float64{
	5.619958268523939882, // face  0
	5.760339081714187279, // face  1
	0.780213654393430055, // face  2
	0.430469363979999913, // face  3
	6.130269123335111400, // face  4
	2.692877706530642877, // face  5
	2.982963003477243874, // face  6
	3.532912002790141181, // face  7
	3.494305004259568154, // face  8
	3.003214169499538391, // face  9
	5.930472956509811562, // face 10
	0.138378484090254847, // face 11
	0.448714947059150361, // face 12
	0.158629650112549365, // face 13
	5.891865957979238535, // face 14
	2.711123289609793325, // face 15
	3.294508837434268316, // face 16
	3.804819692245439833, // face 17
	3.664438879055192436, // face 18
	2.361378999196363184, // face 19
}

var CosFaceLat_32 = [20]float32{

	0.694132208005028062,
	0.260025337896551800,
	0.493444099564646799,
	0.825227416783206214,
	0.881524235868986983,
	0.985116592465405394,
	0.821973179669780118,
	0.910058760174088377,
	0.996875902469535280,
	0.973446711513843321,
	0.996875902469535280,
	0.973446711513843321,
	0.985116592465405394,
	0.821973179669780118,
	0.910058760174088377,
	0.825227416783206214,
	0.881524235868986983,
	0.694132208005028062,
	0.260025337896551800,
	0.493444099564646799,
}

var SinFaceLat_32 = [20]float32{

	0.719847537892618239,
	0.965601793521420504,
	0.869777512128725339,
	0.564800593651703320,
	0.472138773641393006,
	0.171887461000936548,
	0.569526199488268769,
	0.414479255247353850,
	-0.078983764632673703,
	-0.228913738868780775,
	0.078983764632673703,
	0.228913738868780775,
	-0.171887461000936548,
	-0.569526199488268769,
	-0.414479255247353850,
	-0.564800593651703320,
	-0.472138773641393006,
	-0.719847537892618239,
	-0.965601793521420504,
	-0.869777512128725339,
}

var Azimuth_32 = [20]float32{
	5.619958268523939882, // face  0
	5.760339081714187279, // face  1
	0.780213654393430055, // face  2
	0.430469363979999913, // face  3
	6.130269123335111400, // face  4
	2.692877706530642877, // face  5
	2.982963003477243874, // face  6
	3.532912002790141181, // face  7
	3.494305004259568154, // face  8
	3.003214169499538391, // face  9
	5.930472956509811562, // face 10
	0.138378484090254847, // face 11
	0.448714947059150361, // face 12
	0.158629650112549365, // face 13
	5.891865957979238535, // face 14
	2.711123289609793325, // face 15
	3.294508837434268316, // face 16
	3.804819692245439833, // face 17
	3.664438879055192436, // face 18
	2.361378999196363184, // face 19
}

const ScaleFactor = 1e9

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
	e := new(big.Int).Add(components[1], big.NewInt(int64((1<<(E-1))-1+M))).Uint64()
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

// Convert scaled integer to float64 assuming the scale is 1e9
func ScaledIntToFloat64(scaledInt int) float64 {
	return float64(scaledInt) / ScaleFactor
}

// radiansToDegrees converts radians to degrees.
func RadiansToDegrees(rad float64) float64 {
	return rad * (180.0 / math.Pi)
}

func HexToFloat(hexStr string) (float64, error) {
	// Parse the hex string as uint32
	n, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, err
	}

	// Convert the uint32 to float32
	f := math.Float64frombits(uint64(n))
	return f, nil
}

func HexToFloat32(hexStr string) (float32, error) {
	// Parse the hex string as uint32
	n, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return 0, err
	}

	// Convert the uint32 to float32
	f := math.Float32frombits(uint32(n))
	return f, nil
}

func ExecuteLatLngToIJK(resolution int, latitude float64, longitude float64) (int, int, int, error) {
	// Convert float64 latitude and longitude to string
	latStr := fmt.Sprintf("%.30f", latitude)
	lngStr := fmt.Sprintf("%.30f", longitude)
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
const compressThreshold = 1000

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
	pk, vk, err := groth16.Setup(cs)
	require.NoError(b, err)

	fmt.Println("solving and proving...")
	b.ResetTimer()

	b.N = 20

	for i := 0; i < b.N; i++ {
		id := rand.Uint32() % 256 //#nosec G404 -- This is a false positive
		start = time.Now().UnixMicro()
		fmt.Println("groth16 proving", id)
		proof, err := groth16.Prove(cs, pk, fullWitness)
		require.NoError(b, err)
		fmt.Println("groth16 proved", id, "in", time.Now().UnixMicro()-start, "μs")

		publicWitness, _ := fullWitness.Public()

		groth16.Verify(proof, vk, publicWitness)
	}
}

func BenchProofToFileGroth16(b *testing.B, circuit, assignment frontend.Circuit, resolution int64, index int64, path string) error {
	// Open a file to save benchmark results
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		b.Errorf("Failed to open file: %v", err)
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		b.Errorf("Failed to get file stats: %v", err)
		return err
	}
	if info.Size() == 0 {
		_, err = fmt.Fprintln(file, "Resolution, Index, NbConstraints, CompilationTime, SetupTime, ProverTime, VerifierTime")
		if err != nil {
			b.Errorf("Failed to write headers to file: %v", err)
			return err
		}
	}

	// Variables for measuring times
	var compilationTime, setupTime, proverTime, verifierTime int64

	// Benchmarking loop
	b.ResetTimer()
	b.N = 1
	for i := 0; i < b.N; i++ {
		// Compilation step with time measurement
		start := time.Now().UnixMicro()
		cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit, frontend.WithCompressThreshold(compressThreshold))
		if err != nil {
			b.Errorf("Failed to compile: %v", err)
			return err
		}
		compilationTime = time.Now().UnixMicro() - start

		// Print the number of constraints
		fmt.Println("Number of constraints:", cs.GetNbConstraints())

		fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())

		// Setup step with time measurement
		start = time.Now().UnixMicro()
		pk, vk, err := groth16.Setup(cs)
		if err != nil {
			b.Errorf("Failed in setup: %v", err)
			return err
		}
		setupTime = time.Now().UnixMicro() - start

		// Proving step with time measurement
		id := rand.Uint32() % 256 // #nosec G404 -- This is a false positive
		start = time.Now().UnixMicro()
		fmt.Println("groth16 proving", id)
		proof, err := groth16.Prove(cs, pk, fullWitness)
		if err != nil {
			b.Errorf("Failed in proving: %v", err)
			return err
		}
		proverTime = time.Now().UnixMicro() - start

		// Verifier step with time measurement
		start = time.Now().UnixMicro()
		publicWitness, _ := fullWitness.Public()
		groth16.Verify(proof, vk, publicWitness)
		if err != nil {
			b.Errorf("Failed in verifying: %v", err)
			return err
		}
		verifierTime = time.Now().UnixMicro() - start

		// Writing the captured data to the file
		_, err = fmt.Fprintf(file, "%d, %d, %d, %d, %d, %d, %d\n",
			resolution, index, cs.GetNbConstraints(),
			compilationTime, setupTime, proverTime, verifierTime)
		if err != nil {
			b.Errorf("Failed to write data to file: %v", err)
			return err
		}
	}
	return nil
}

func BenchProofToFilePlonk(b *testing.B, circuit, assignment frontend.Circuit, resolution int64, index int64, path string) error {
	// Open a file to save benchmark results
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		b.Errorf("Failed to open file: %v", err)
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		b.Errorf("Failed to get file stats: %v", err)
		return err
	}
	if info.Size() == 0 {
		_, err = fmt.Fprintln(file, "Resolution, Index, NbConstraints, CompilationTime, SetupTime, ProverTime, VerifierTime")
		if err != nil {
			b.Errorf("Failed to write headers to file: %v", err)
			return err
		}
	}

	// Variables for measuring times
	var compilationTime, setupTime, proverTime, verifierTime int64

	// Benchmarking loop
	b.ResetTimer()
	b.N = 1
	for i := 0; i < b.N; i++ {
		// Compilation step with time measurement
		start := time.Now().UnixMicro()
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, circuit, frontend.WithCompressThreshold(compressThreshold))
		if err != nil {
			b.Errorf("Failed to compile: %v", err)
			return err
		}
		compilationTime = time.Now().UnixMicro() - start

		// Print the number of constraints
		fmt.Println("Number of constraints:", ccs.GetNbConstraints())

		fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())

		// create srs
		srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
		fmt.Println(reflect.TypeOf(srs))
		if err != nil {
			panic("Failed to create srs: " + err.Error())
		}

		// Setup step with time measurement
		start = time.Now().UnixMicro()
		pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
		if err != nil {
			b.Errorf("Failed in setup: %v", err)
			return err
		}
		setupTime = time.Now().UnixMicro() - start

		// Proving step with time measurement
		id := rand.Uint32() % 256 // #nosec G404 -- This is a false positive
		fmt.Println("plonk proving", id)
		start = time.Now().UnixMicro()
		proof, err := plonk.Prove(ccs, pk, fullWitness)
		if err != nil {
			b.Errorf("Failed in proving: %v", err)
			return err
		}
		proverTime = time.Now().UnixMicro() - start

		// Verifier step with time measurement
		start = time.Now().UnixMicro()
		publicWitness, _ := fullWitness.Public()
		err = plonk.Verify(proof, vk, publicWitness)
		verifierTime = time.Now().UnixMicro() - start
		if err != nil {
			b.Errorf("Failed in verifying: %v", err)
			return err
		}

		// Writing the captured data to the file
		_, err = fmt.Fprintf(file, "%d, %d, %d, %d, %d, %d, %d\n",
			resolution, index, ccs.GetNbConstraints(),
			compilationTime, setupTime, proverTime, verifierTime)
		if err != nil {
			b.Errorf("Failed to write data to file: %v", err)
			return err
		}
	}
	return nil
}

func BenchProofMemoryGroth16(b *testing.B, circuit, assignment frontend.Circuit) error {

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit, frontend.WithCompressThreshold(compressThreshold))
	if err != nil {
		b.Errorf("Failed to compile: %v", err)
		return err
	}

	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		b.Errorf("Failed Full Witness: %v", err)
		return err
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		b.Errorf("Failed in setup: %v", err)
		return err
	}

	// // Open the file in write mode for pk
	// var bufPK bytes.Buffer
	// _, _ = pk.WriteTo(&bufPK)
	// err = os.WriteFile("../benchmarks/pk.dat", bufPK.Bytes(), 0644)
	// if err != nil {
	// 	b.Errorf("Failed in writing prover key: %v", err)
	// 	return err
	// }

	// // Open the file in write mode for vk
	// var bufVK bytes.Buffer
	// _, _ = vk.WriteTo(&bufVK)
	// err = os.WriteFile("../benchmarks/vk.dat", bufVK.Bytes(), 0644)
	// if err != nil {
	// 	b.Errorf("Failed in writing verifier key key: %v", err)
	// 	return err
	// }

	proof, err := groth16.Prove(cs, pk, fullWitness)
	if err != nil {
		b.Errorf("Failed in proving: %v", err)
		return err
	}

	publicWitness, _ := fullWitness.Public()
	groth16.Verify(proof, vk, publicWitness)

	return nil
}

func BenchProofMemoryPlonk(b *testing.B, circuit, assignment frontend.Circuit) error {

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, circuit, frontend.WithCompressThreshold(compressThreshold))
	if err != nil {
		b.Errorf("Failed to compile: %v", err)
		return err
	}

	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		b.Errorf("Failed Full Witness: %v", err)
		return err
	}

	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)

	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		b.Errorf("Failed in setup: %v", err)
		return err
	}

	// // Open the file in write mode for pk
	// var bufSRS bytes.Buffer
	// _, _ = srs.WriteTo(&bufSRS)
	// err = os.WriteFile("../benchmarks/srs.dat", bufSRS.Bytes(), 0644)
	// if err != nil {
	// 	b.Errorf("Failed in writing SRS: %v", err)
	// 	return err
	// }

	// // Open the file in write mode for vk
	// var bufVK bytes.Buffer
	// _, _ = vk.WriteTo(&bufVK)
	// err = os.WriteFile("../benchmarks/vk.dat", bufVK.Bytes(), 0644)
	// if err != nil {
	// 	b.Errorf("Failed in writing verifier key key: %v", err)
	// 	return err
	// }

	proof, err := plonk.Prove(ccs, pk, fullWitness)
	if err != nil {
		b.Errorf("Failed in proving: %v", err)
		return err
	}

	publicWitness, _ := fullWitness.Public()
	err = plonk.Verify(proof, vk, publicWitness)
	if err != nil {
		b.Errorf("Failed in verifying: %v", err)
		return err
	}

	return nil
}
