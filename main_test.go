package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// Create a test server to mock the FIP API
func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"test"}`))
	}))
}

func TestHandler(t *testing.T) {
	// Setup test server
	testServer := setupTestServer()
	defer testServer.Close()

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
	// Setup test server
	testServer := setupTestServer()
	defer testServer.Close()

	// Reset cache before test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	param := "fip_rock"
	testData := []byte(`{"data":"test"}`)

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

func TestCachingMechanism(t *testing.T) {
	// Setup test server
	testServer := setupTestServer()
	defer testServer.Close()

	// Reset cache before test
	cacheMutex.Lock()
	cache = make(map[string]CachedResponse)
	cacheMutex.Unlock()

	param := "fip_rock"
	testData := []byte(`{"data":"test"}`)
	
	// Pre-populate cache
	cacheMutex.Lock()
	cache[param] = CachedResponse{
		Data:     testData,
		CachedAt: time.Now(),
	}
	cacheMutex.Unlock()

	cachedData, cachedETag, err := getCachedData(param)
	if err != nil {
		t.Fatalf("getCachedData returned an error: %v", err)
	}

	if !bytes.Equal(cachedData, testData) {
		t.Errorf("getCachedData returned unexpected data: got %s want %s",
			string(cachedData), string(testData))
	}

	expectedETag := generateETag(testData)
	if cachedETag != expectedETag {
		t.Errorf("getCachedData returned unexpected ETag: got %v want %v",
			cachedETag, expectedETag)
	}
}
