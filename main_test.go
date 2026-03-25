// ABOUTME: Unit tests for the FIP metadata server.
// ABOUTME: Tests handler, caching, metadata fetching, and station name validation.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// TestServer represents a mock server for testing
type TestServer struct {
	server *httptest.Server
}

// NewTestServer creates and returns a test server that mimics the Radio France livemeta API
func NewTestServer() *TestServer {
	ts := &TestServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The new API path format is /livemeta/live/{stationId}/{format}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"now": map[string]interface{}{
				"firstLine":  "Test Song",
				"secondLine": "Test Artist",
				"cover":      "test-cover-uuid",
				"startTime":  1700000000,
				"endTime":    1700000300,
			},
			"next": []interface{}{},
			"prev": []interface{}{},
		}
		data, _ := json.Marshal(resp)
		if _, err := w.Write(data); err != nil {
			http.Error(w, "Error writing response", http.StatusInternalServerError)
			return
		}
	}))
	return ts
}

// Close shuts down the test server
func (ts *TestServer) Close() {
	ts.server.Close()
}

// URL returns the test server's URL
func (ts *TestServer) URL() string {
	return ts.server.URL
}

func TestHandler(t *testing.T) {
	// Store the original and restore it after the test
	originalFetchMetadata := fetchMetadata
	defer func() { fetchMetadata = originalFetchMetadata }()

	// Override fetchMetadata for testing
	fetchMetadata = func(param string) ([]byte, error) {
		resp := map[string]interface{}{
			"stationName": param,
			"now":         map[string]interface{}{"firstLine": "Test"},
		}
		data, _ := json.Marshal(resp)
		return data, nil
	}

	// Reset cache for this test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	req, err := http.NewRequest("GET", "/api/metadata/fip_rock", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/api/metadata/{param}", handler)
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestHandlerUnknownStation(t *testing.T) {
	// Reset cache
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	// Restore original fetchMetadata after test
	originalFetchMetadata := fetchMetadata
	defer func() { fetchMetadata = originalFetchMetadata }()

	// Use real fetchMetadata (it will fail on unknown station before making HTTP call)
	fetchMetadata = originalFetchMetadata

	req, err := http.NewRequest("GET", "/api/metadata/fip_nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/api/metadata/{param}", handler)
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler should return 500 for unknown station: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestGetCachedData(t *testing.T) {
	// Reset cache before test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	param := "fip_rock"
	testData := []byte(`{"stationName":"fip_rock","now":{"firstLine":"Test"}}`)

	// Pre-populate cache with test data
	cacheMutex.Lock()
	cache[param] = CachedResponse{
		Data:     testData,
		CachedAt: time.Now(),
	}
	cacheMutex.Unlock()

	data, etag, err := getCachedData(param)
	if err != nil {
		t.Fatalf("getCachedData returned an error: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("getCachedData returned unexpected data: got %v want %v",
			string(data), string(testData))
	}

	expectedETag := generateETag(testData)
	if etag != expectedETag {
		t.Errorf("getCachedData returned unexpected ETag: got %v want %v",
			etag, expectedETag)
	}
}

func TestFetchMetadata(t *testing.T) {
	// Setup test server
	ts := NewTestServer()
	defer ts.Close()

	// Override baseURL to point to test server
	originalBaseURL := baseURL
	baseURL = ts.server.URL + "/livemeta/live"
	defer func() { baseURL = originalBaseURL }()

	param := "fip_rock"
	data, err := fetchMetadata(param)
	if err != nil {
		t.Fatalf("fetchMetadata returned an error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify stationName is injected
	if resp["stationName"] != param {
		t.Errorf("expected stationName %s, got %v", param, resp["stationName"])
	}

	// Verify now block exists
	if resp["now"] == nil {
		t.Error("expected 'now' field in response")
	}
}

func TestStationNames(t *testing.T) {
	stations := []string{
		"fip_reggae", "fip_pop", "fip_metal", "fip_hiphop", "fip_rock",
		"fip_jazz", "fip_world", "fip_groove", "fip_nouveautes", "fip_electro", "fip_cultes", "fip",
	}

	for _, station := range stations {
		t.Run(station, func(t *testing.T) {
			// Setup test server
			ts := NewTestServer()
			defer ts.Close()

			// Override baseURL to point to test server
			originalBaseURL := baseURL
			baseURL = ts.server.URL + "/livemeta/live"
			defer func() { baseURL = originalBaseURL }()

			data, err := fetchMetadata(station)
			if err != nil {
				t.Fatalf("fetchMetadata returned an error for %s: %v", station, err)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Fatalf("failed to unmarshal response for %s: %v", station, err)
			}

			// Verify stationName is injected for backward compatibility
			if resp["stationName"] != station {
				t.Errorf("expected stationName %s, got %v", station, resp["stationName"])
			}
		})
	}
}

func TestStationMap(t *testing.T) {
	// Verify all expected stations exist in the mapping
	expectedStations := []string{
		"fip", "fip_rock", "fip_jazz", "fip_groove", "fip_world",
		"fip_nouveautes", "fip_reggae", "fip_electro", "fip_metal",
		"fip_pop", "fip_hiphop", "fip_cultes",
	}

	for _, name := range expectedStations {
		cfg, ok := stationMap[name]
		if !ok {
			t.Errorf("station %s not found in stationMap", name)
			continue
		}
		if cfg.ID <= 0 {
			t.Errorf("station %s has invalid ID: %d", name, cfg.ID)
		}
		if cfg.Format == "" {
			t.Errorf("station %s has empty format", name)
		}
	}

	// Verify main FIP uses fip_player format
	if stationMap["fip"].Format != "webrf_fip_player" {
		t.Errorf("main FIP station should use webrf_fip_player format, got %s", stationMap["fip"].Format)
	}
}

func TestBuildURL(t *testing.T) {
	// Verify URL construction for a known station
	station := stationMap["fip_rock"]
	url := fmt.Sprintf("%s/%d/%s", baseURL, station.ID, station.Format)
	expected := "https://api.radiofrance.fr/livemeta/live/64/webrf_webradio_player"
	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}

	station = stationMap["fip"]
	url = fmt.Sprintf("%s/%d/%s", baseURL, station.ID, station.Format)
	expected = "https://api.radiofrance.fr/livemeta/live/7/webrf_fip_player"
	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}
}
