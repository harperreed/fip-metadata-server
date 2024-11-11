package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/metadata/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"data":"test"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestGetCachedData(t *testing.T) {
	param := "test"
	data, etag, err := getCachedData(param)
	if err != nil {
		t.Fatalf("getCachedData returned an error: %v", err)
	}

	expectedData := []byte(`{"data":"test"}`)
	if !bytes.Equal(data, expectedData) {
		t.Errorf("getCachedData returned unexpected data: got %v want %v",
			data, expectedData)
	}

	expectedETag := generateETag(expectedData)
	if etag != expectedETag {
		t.Errorf("getCachedData returned unexpected ETag: got %v want %v",
			etag, expectedETag)
	}
}

func TestCachingMechanism(t *testing.T) {
	param := "test"
	data := []byte(`{"data":"test"}`)
	etag := generateETag(data)

	cacheMutex.Lock()
	cache[param] = CachedResponse{Data: data, CachedAt: time.Now()}
	cacheMutex.Unlock()

	cachedData, cachedETag, err := getCachedData(param)
	if err != nil {
		t.Fatalf("getCachedData returned an error: %v", err)
	}

	if !bytes.Equal(cachedData, data) {
		t.Errorf("getCachedData returned unexpected data: got %v want %v",
			cachedData, data)
	}

	if cachedETag != etag {
		t.Errorf("getCachedData returned unexpected ETag: got %v want %v",
			cachedETag, etag)
	}
}

func TestFetchMetadata(t *testing.T) {
	param := "test"
	data, err := fetchMetadata(param)
	if err != nil {
		t.Fatalf("fetchMetadata returned an error: %v", err)
	}

	expectedData := []byte(`{"data":"test"}`)
	if !bytes.Equal(data, expectedData) {
		t.Errorf("fetchMetadata returned unexpected data: got %v want %v",
			data, expectedData)
	}
}

func generateETag(data []byte) string {
	hash := sha256.Sum256(data)
	return "\"" + hex.EncodeToString(hash[:]) + "\""
}
