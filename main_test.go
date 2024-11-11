package main

import (
	"bytes"
	"encoding/json"
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

// NewTestServer creates and returns a new test server
func NewTestServer() *TestServer {
	ts := &TestServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request path contains the expected endpoint
		if r.URL.Path != "/fip/api/live" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Get the webradio parameter
		webradio := r.URL.Query().Get("webradio")
		if webradio == "" {
			http.Error(w, "Missing webradio parameter", http.StatusBadRequest)
			return
		}

		// Return appropriate test response based on the webradio parameter
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"stationName": webradio,
			"data":        "test",
		}
		jsonResponse, _ := json.Marshal(response)
		if _, err := w.Write(jsonResponse); err != nil {
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
	// Setup test server
	ts := NewTestServer()
	defer ts.Close()

	// Store the original URL and restore it after the test
	originalFetchMetadata := fetchMetadata
	defer func() { fetchMetadata = originalFetchMetadata }()

	// Override fetchMetadata for testing
	fetchMetadata = func(param string) ([]byte, error) {
		return []byte(`{"stationName":"` + param + `","data":"test"}`), nil
	}

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

	// Reset cache for this test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()
}

func TestGetCachedData(t *testing.T) {
	// Reset cache before test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	param := "fip_rock"
	testData := []byte(`{"stationName":"fip_rock","data":"test"}`)

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

	if !bytes.Equal(data, testData) {
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

	// Store the original baseURL and restore it after the test
	originalBaseURL := baseURL
	baseURL = ts.server.URL + "/fip/api/live" // Fixed: Using server.URL directly
	defer func() { baseURL = originalBaseURL }()

	param := "test"
	data, err := fetchMetadata(param)
	if err != nil {
		t.Fatalf("fetchMetadata returned an error: %v", err)
	}

	expectedData := []byte(`{"stationName":"test","data":"test"}`)
	if !bytes.Equal(data, expectedData) {
		t.Errorf("fetchMetadata returned unexpected data: got %s want %s",
			string(data), string(expectedData))
	}

	// Parse the JSON response and check the stationName field
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(data, &jsonResponse); err != nil {
		t.Fatalf("error unmarshalling JSON response: %v", err)
	}

	if stationName, ok := jsonResponse["stationName"].(string); !ok || stationName != param {
		t.Errorf("stationName mismatch: expected %s, got %s", param, stationName)
	}
}
