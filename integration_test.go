package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

const (
	integrationTestEnv = "INTEGRATION_TESTS"
)

// TestIntegrationConfig holds configuration for integration tests
type TestIntegrationConfig struct {
	timeout time.Duration
}

// setupIntegrationTest prepares the test environment
func setupIntegrationTest(t *testing.T) *TestIntegrationConfig {
	t.Helper()

	// Check if integration tests should run
	if os.Getenv(integrationTestEnv) == "" {
		t.Skip("Skipping integration tests. Set INTEGRATION_TESTS=1 to run them.")
	}

	return &TestIntegrationConfig{
		timeout: 10 * time.Second,
	}
}

// validateJSONResponse helper function to validate JSON responses
func validateJSONResponse(data []byte) error {
	var result map[string]interface{}
	return json.Unmarshal(data, &result)
}

// TestIntegrationSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	config := setupIntegrationTest(t)

	// Define test cases
	tests := []struct {
		name string
		fn   func(t *testing.T, cfg *TestIntegrationConfig)
	}{
		{"FetchMetadata", testFetchMetadata},
		{"AllStations", testAllStations},
		{"Caching", testCaching},
		{"ConcurrentRequests", testConcurrentRequests},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t, config)
		})
	}
}

func testFetchMetadata(t *testing.T, cfg *TestIntegrationConfig) {
	data, err := fetchMetadata("fip")
	if err != nil {
		t.Fatalf("Failed to fetch metadata: %v", err)
	}

	if err := validateJSONResponse(data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
}

func testAllStations(t *testing.T, cfg *TestIntegrationConfig) {
	stations := []string{
		"fip_reggae",
		"fip_pop",
		"fip_metal",
		"fip_hiphop",
		"fip_rock",
		"fip_jazz",
		"fip_world",
		"fip_groove",
		"fip_nouveautes",
		"fip_electro",
		"fip",
	}

	for _, station := range stations {
		t.Run(station, func(t *testing.T) {
			data, err := fetchMetadata(station)
			if err != nil {
				t.Fatalf("Failed to fetch metadata for %s: %v", station, err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("Invalid JSON response for %s: %v", station, err)
			}

			expectedFields := []string{"levels", "slots"}
			for _, field := range expectedFields {
				if _, ok := result[field]; !ok {
					t.Errorf("Expected field '%s' missing from response for station %s", field, station)
				}
			}
		})
	}
}

func testCaching(t *testing.T, cfg *TestIntegrationConfig) {
	station := "fip"

	// First request
	data1, etag1, err := getCachedData(station)
	if err != nil {
		t.Fatalf("Failed to get initial data: %v", err)
	}

	// Immediate second request
	data2, etag2, err := getCachedData(station)
	if err != nil {
		t.Fatalf("Failed to get cached data: %v", err)
	}

	if etag1 != etag2 {
		t.Errorf("Cache inconsistency: ETags don't match. Expected %s, got %s", etag1, etag2)
	}

	// Wait for cache to expire
	time.Sleep(cacheTTL + 100*time.Millisecond)

	// Third request
	data3, _, err := getCachedData(station)
	if err != nil {
		t.Fatalf("Failed to get fresh data after cache expiry: %v", err)
	}

	// Validate all responses
	for i, data := range [][]byte{data1, data2, data3} {
		if err := validateJSONResponse(data); err != nil {
			t.Errorf("Invalid JSON in response %d: %v", i+1, err)
		}
	}
}

func testConcurrentRequests(t *testing.T, cfg *TestIntegrationConfig) {
	concurrentRequests := 5
	station := "fip"

	errChan := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			data, _, err := getCachedData(station)
			if err != nil {
				errChan <- err
				return
			}

			if err := validateJSONResponse(data); err != nil {
				errChan <- err
				return
			}

			errChan <- nil
		}()
	}

	for i := 0; i < concurrentRequests; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}
