package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LucaTheHacker/go-haversine"
)

const (
	RadiusOfEarth = 6371000.0 // meters
	Pi            = math.Pi
)

// Haversine calculates the distance between two points on Earth.
func OldHaversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)
	a := math.Pow(math.Sin(dLat/2), 2) + math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*math.Pow(math.Sin(dLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return RadiusOfEarth * c
}

// degreesToRadians converts degrees to radians.
func degreesToRadians(degrees float64) float64 {
	return degrees * (Pi / 180)
}

// HexCell represents a hexagonal cell with its center and boundary coordinates.
type HexCell struct {
	CenterLat, CenterLon float64
	Boundary             [][2]float64
}

// floatToHex converts a float64 to a hexadecimal string.
func floatToHex(f float64) string {
	bits := math.Float64bits(f)
	return strconv.FormatUint(bits, 16)
}

// WriteHexCellsToFile writes hex cell data to a file.
func WriteHexCellsToFile(cells []HexCell, filename string, useFloat32 bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, cell := range cells {
		var lat, lon string
		if useFloat32 {
			lat = floatToHex(float64(float32(cell.CenterLat)))
			lon = floatToHex(float64(float32(cell.CenterLon)))
		} else {
			lat = floatToHex(cell.CenterLat)
			lon = floatToHex(cell.CenterLon)
		}

		// Example: Write latitude and longitude as hex strings.
		_, err := file.WriteString(fmt.Sprintf("%s %s\n", lat, lon))
		if err != nil {
			return err
		}
	}
	return nil
}

// executeCommand runs a given command with arguments and returns the output as a string.
func executeCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	outputBytes, err := cmd.CombinedOutput()
	return string(outputBytes), err
}

// executeLatLngToIJK executes the latLngToCell command and parses its output.
func executeLatLngToIJK(executablePath string, resolution int, latitude, longitude float64) (int, int, int, error) {
	output, err := executeCommand(executablePath, "--resolution", fmt.Sprint(resolution), "--latitude", fmt.Sprintf("%.30f", latitude), "--longitude", fmt.Sprintf("%.30f", longitude))
	if err != nil {
		return 0, 0, 0, err
	}

	pattern := regexp.MustCompile(`I: (\d+), J: (\d+), K: (\d+)`)
	matches := pattern.FindStringSubmatch(output)
	if matches == nil || len(matches) < 4 {
		return 0, 0, 0, fmt.Errorf("failed to parse output")
	}

	return extractInt(matches[1]), extractInt(matches[2]), extractInt(matches[3]), nil
}

// extractInt is a helper function to convert a string to an int.
func extractInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func latLngToCell(resolution int, latitude, longitude float64) (string, error) {
	executablePath := "../h3-master/build/bin/latLngToCell"
	output, err := executeCommand(executablePath, "--resolution", strconv.Itoa(resolution), "--latitude", fmt.Sprintf("%.30f", latitude), "--longitude", fmt.Sprintf("%.30f", longitude))
	if err != nil {
		return "", err
	}

	pattern := regexp.MustCompile(`--- (\w+)`)
	matches := pattern.FindStringSubmatch(output)
	if matches == nil || len(matches) < 2 {
		return "", fmt.Errorf("failed to parse H3 index")
	}

	return matches[1], nil
}

func cellToLatLng(h3Index string) (float64, float64, error) {
	cmd := exec.Command("../h3-master/build/bin/cellToLatLng", "--index", h3Index)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(string(output))
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid output format")
	}

	lat, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, err
	}

	lng, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, err
	}

	return lat, lng, nil
}

func cellToBoundary(h3Index string) ([][2]float64, error) {
	cmd := exec.Command("../h3-master/build/bin/cellToBoundary", "--index", h3Index)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`(\d+\.\d+)\s+(\d+\.\d+)`)
	matches := pattern.FindAllStringSubmatch(string(output), -1)

	var boundary [][2]float64
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		lat, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return nil, err
		}

		lng, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, err
		}

		boundary = append(boundary, [2]float64{lat, lng})
	}

	return boundary, nil
}

func main() {
	// User Lat/Lon
	verifierLat := 40.689167
	verifierLon := 72.044444

	// Example latitude and longitude
	proverLat := 40.689167
	proverLon := 74.044444
	resolution := 4

	// Step 1: Generate H3 index for the given latitude and longitude
	h3Index, err := latLngToCell(resolution, proverLat, proverLon)
	if err != nil {
		fmt.Printf("Error generating H3 index: %s\n", err)
		return
	}
	fmt.Printf("Generated H3 index: %s\n", h3Index)

	numberOfExecutions := 20
	var totalDuration time.Duration
	closestDistance := math.MaxFloat64

	for i := 0; i < numberOfExecutions; i++ {
		startTime := time.Now()

		// Step 2: Obtain all boundary points for the generated H3 index
		boundaryPoints, err := cellToBoundary(h3Index)
		if err != nil {
			fmt.Printf("Error obtaining boundary points: %s\n", err)
			return
		}
		fmt.Println("Boundary Points:", boundaryPoints)

		// Create a coordinate object for the example location
		exampleLocation := haversine.Coordinates{Latitude: verifierLat, Longitude: verifierLon}

		// Step 3: Calculate the closest distance from exampleLat, exampleLon to the hexagon's boundary

		for _, point := range boundaryPoints {
			boundaryLocation := haversine.Coordinates{Latitude: point[0], Longitude: point[1]}
			distance := haversine.Distance(exampleLocation, boundaryLocation).Kilometers()

			if distance < closestDistance {
				closestDistance = distance
			}
		}

		// Calculate elapsed time for this execution
		elapsedTime := time.Since(startTime)
		totalDuration += elapsedTime

		// Optionally, print result of each iteration
		// fmt.Printf("Iteration %d - Closest distance: %.2f Kilometers, Time taken: %s\n", i+1, closestDistance, elapsedTime)
	}

	// Calculate average duration
	averageDuration := totalDuration / time.Duration(numberOfExecutions)
	fmt.Printf("Average time taken over %d executions: %s\n", numberOfExecutions, averageDuration)

	fmt.Printf("Closest distance to hexagon's boundary: %.2f Kilometers\n", closestDistance)
}
