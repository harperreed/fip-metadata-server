package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// CachedResponse stores the response data and the time it was cached
type CachedResponse struct {
	Data     []byte
	CachedAt time.Time
}

var (
	cache      = make(map[string]CachedResponse)
	cacheMutex sync.Mutex
	cacheTTL   = 1 * time.Second // Cache Time-To-Live
	baseURL    = "https://www.radiofrance.fr/fip/api/live"
)

func main() {
	router := mux.NewRouter()

	// API route
	router.HandleFunc("/api/metadata/{param}", handler).Methods("GET")

	// Serve the index.html file for documentation
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func handler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fipParam, ok := vars["param"]
	if !ok {
		log.Println("Missing 'param' parameter in request")
		http.Error(w, "Missing 'param' parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Fetching data for param: %s\n", fipParam)
	data, etag, err := getCachedData(fipParam)
	if err != nil {
		log.Printf("Error fetching data for param: %s, error: %v\n", fipParam, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if the client has a cached version
	clientETag := r.Header.Get("If-None-Match")
	if clientETag == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Write(data)
}

func getCachedData(param string) ([]byte, string, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	log.Printf("Checking cache for param: %s\n", param)

	// Check if data is cached and still valid
	if cachedResponse, found := cache[param]; found {
		if time.Since(cachedResponse.CachedAt) < cacheTTL {
			log.Printf("Cache hit for param: %s\n", param)
			return cachedResponse.Data, generateETag(cachedResponse.Data), nil
		}
		// Remove stale cache
		log.Printf("Cache expired for param: %s, fetching new data\n", param)
		delete(cache, param)
	} else {
		log.Printf("Cache miss for param: %s\n", param)
	}

	// Fetch new data
	data, err := fetchMetadata(param)
	if err != nil {
		return nil, "", err
	}

	// Cache the new data
	cache[param] = CachedResponse{Data: data, CachedAt: time.Now()}
	log.Printf("New data cached for param: %s\n", param)

	return data, generateETag(data), nil
}

// Make fetchMetadata a variable so it can be replaced in tests
var fetchMetadata = func(param string) ([]byte, error) {
	url := fmt.Sprintf("%s?webradio=%s", baseURL, param)
	log.Printf("Fetching data from: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching data for %s: %v", param, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response code for %s: %d", param, resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body for %s: %v", param, err)
	}

	return data, nil
}

func generateETag(data []byte) string {
	// Generate a SHA-256 hash of the JSON data
	hash := sha256.Sum256(data)
	etag := hex.EncodeToString(hash[:])
	return "\"" + etag + "\""
}
