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

	// Verify "now" is transformed: firstLine should be an object with "title"
	now, ok := resp["now"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'now' to be an object")
	}

	fl, ok := now["firstLine"].(map[string]interface{})
	if !ok {
		t.Fatal("expected now.firstLine to be an object")
	}
	if fl["title"] != "Test Song" {
		t.Errorf("expected now.firstLine.title = 'Test Song', got %v", fl["title"])
	}

	sl, ok := now["secondLine"].(map[string]interface{})
	if !ok {
		t.Fatal("expected now.secondLine to be an object")
	}
	if sl["title"] != "Test Artist" {
		t.Errorf("expected now.secondLine.title = 'Test Artist', got %v", sl["title"])
	}

	// Verify visuals.card.src is constructed from cover UUID
	visuals, ok := now["visuals"].(map[string]interface{})
	if !ok {
		t.Fatal("expected now.visuals to be an object")
	}
	card, ok := visuals["card"].(map[string]interface{})
	if !ok {
		t.Fatal("expected now.visuals.card to be an object")
	}
	expectedSrc := visualBaseURL + "/test-cover-uuid"
	if card["src"] != expectedSrc {
		t.Errorf("expected visuals.card.src = %s, got %v", expectedSrc, card["src"])
	}
}

func TestTransformTrack(t *testing.T) {
	raw := map[string]interface{}{
		"firstLine":  "Song Title",
		"secondLine": "Artist Name",
		"cover":      "abc-123-uuid",
		"startTime":  float64(1700000000),
		"endTime":    float64(1700000300),
		"songUuid":   "song-uuid-456",
	}

	result := transformTrack(raw)

	// firstLine should be object with title
	fl := result["firstLine"].(map[string]interface{})
	if fl["title"] != "Song Title" {
		t.Errorf("expected firstLine.title = 'Song Title', got %v", fl["title"])
	}

	// secondLine should be object with title
	sl := result["secondLine"].(map[string]interface{})
	if sl["title"] != "Artist Name" {
		t.Errorf("expected secondLine.title = 'Artist Name', got %v", sl["title"])
	}

	// visuals.card.src should be constructed from cover
	visuals := result["visuals"].(map[string]interface{})
	card := visuals["card"].(map[string]interface{})
	if card["src"] != visualBaseURL+"/abc-123-uuid" {
		t.Errorf("unexpected visuals.card.src: %v", card["src"])
	}

	// Timing fields preserved
	if result["startTime"] != float64(1700000000) {
		t.Errorf("startTime not preserved: %v", result["startTime"])
	}
	if result["songUuid"] != "song-uuid-456" {
		t.Errorf("songUuid not preserved: %v", result["songUuid"])
	}
}

func TestTransformResponse(t *testing.T) {
	raw := map[string]interface{}{
		"now": map[string]interface{}{
			"firstLine":  "Current Song",
			"secondLine": "Current Artist",
			"cover":      "now-uuid",
		},
		"next": []interface{}{
			map[string]interface{}{
				"firstLine":  "Next Song",
				"secondLine": "Next Artist",
				"cover":      "next-uuid",
			},
		},
		"prev": []interface{}{
			map[string]interface{}{
				"firstLine":  "Prev Song",
				"secondLine": "Prev Artist",
				"cover":      "prev-uuid",
			},
		},
		"delayToRefresh": float64(60000),
	}

	result := transformResponse(raw, "fip_rock")

	if result["stationName"] != "fip_rock" {
		t.Errorf("expected stationName fip_rock, got %v", result["stationName"])
	}

	// now should be a transformed single object
	now := result["now"].(map[string]interface{})
	nowFL := now["firstLine"].(map[string]interface{})
	if nowFL["title"] != "Current Song" {
		t.Errorf("expected now.firstLine.title = 'Current Song', got %v", nowFL["title"])
	}

	// next should be a transformed single object (first element of array)
	next := result["next"].(map[string]interface{})
	nextFL := next["firstLine"].(map[string]interface{})
	if nextFL["title"] != "Next Song" {
		t.Errorf("expected next.firstLine.title = 'Next Song', got %v", nextFL["title"])
	}

	// prev should be a transformed single object
	prev := result["prev"].(map[string]interface{})
	prevFL := prev["firstLine"].(map[string]interface{})
	if prevFL["title"] != "Prev Song" {
		t.Errorf("expected prev.firstLine.title = 'Prev Song', got %v", prevFL["title"])
	}

	if result["delayToRefresh"] != float64(60000) {
		t.Errorf("expected delayToRefresh 60000, got %v", result["delayToRefresh"])
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
