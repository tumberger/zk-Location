package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecuteLatLngToIJKWithRadians(t *testing.T) {
	// Set up test cases with lat and long in radians
	tests := []struct {
		resolution int
		latitude   float64 // Latitude in radians
		longitude  float64 // Longitude in radians
		expectedI  int
		expectedJ  int
		expectedK  int
	}{
		// Convert the provided radian coordinates to degrees for the test case
		{0, RadiansToDegrees(-1.500000), RadiansToDegrees(-3.100000), 1, 0, 0},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("res=%d,latRad=%f,lngRad=%f", tc.resolution, tc.latitude, tc.longitude), func(t *testing.T) {
			// Call the function with converted degree values
			i, j, k, err := ExecuteLatLngToIJK(tc.resolution, tc.latitude, tc.longitude)
			require.NoError(t, err, "ExecuteLatLngToIJK should not return an error")
			require.Equal(t, tc.expectedI, i, "Expected I to be %d, got %d", tc.expectedI, i)
			require.Equal(t, tc.expectedJ, j, "Expected J to be %d, got %d", tc.expectedJ, j)
			require.Equal(t, tc.expectedK, k, "Expected K to be %d, got %d", tc.expectedK, k)
			// Print the coordinates and the error if any
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Printf("I coordinate: %d, J coordinate: %d, K coordinate: %d\n", i, j, k)
			}
		})
	}
}

func TestExecuteLatLngToIJK(t *testing.T) {
	// Set up test cases
	tests := []struct {
		resolution int
		latitude   float64
		longitude  float64
		expectedI  int
		expectedJ  int
		expectedK  int
	}{
		{10, 40.689167, -74.044444, 0, 2620, 17023},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("res=%d,lat=%f,lng=%f", tc.resolution, tc.latitude, tc.longitude), func(t *testing.T) {
			i, j, k, err := ExecuteLatLngToIJK(tc.resolution, tc.latitude, tc.longitude)
			require.NoError(t, err, "ExecuteLatLngToIJK should not return an error")
			require.Equal(t, tc.expectedI, i, "Expected I to be %d, got %d", tc.expectedI, i)
			require.Equal(t, tc.expectedJ, j, "Expected J to be %d, got %d", tc.expectedJ, j)
			require.Equal(t, tc.expectedK, k, "Expected K to be %d, got %d", tc.expectedK, k)
			// Print the coordinates and the error if any
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Printf("I coordinate: %d, J coordinate: %d, K coordinate: %d\n", i, j, k)
			}
		})
	}
}
