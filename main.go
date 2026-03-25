package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

// stationConfig holds the numeric ID and API format for a FIP channel
type stationConfig struct {
	ID     int
	Format string
}

var (
	cache      = make(map[string]CachedResponse)
	cacheMutex sync.Mutex
	cacheTTL   = 1 * time.Second // Cache Time-To-Live
	baseURL    = "https://api.radiofrance.fr/livemeta/live"

	// stationMap maps channel names to their Radio France station IDs and API formats.
	// The main FIP station uses "webrf_fip_player"; webradios use "webrf_webradio_player".
	stationMap = map[string]stationConfig{
		"fip":             {ID: 7, Format: "webrf_fip_player"},
		"fip_rock":        {ID: 64, Format: "webrf_webradio_player"},
		"fip_jazz":        {ID: 65, Format: "webrf_webradio_player"},
		"fip_groove":      {ID: 66, Format: "webrf_webradio_player"},
		"fip_world":       {ID: 69, Format: "webrf_webradio_player"},
		"fip_nouveautes":  {ID: 70, Format: "webrf_webradio_player"},
		"fip_reggae":      {ID: 71, Format: "webrf_webradio_player"},
		"fip_electro":     {ID: 74, Format: "webrf_webradio_player"},
		"fip_metal":       {ID: 77, Format: "webrf_webradio_player"},
		"fip_pop":         {ID: 78, Format: "webrf_webradio_player"},
		"fip_hiphop":      {ID: 95, Format: "webrf_webradio_player"},
		"fip_cultes":      {ID: 709, Format: "webrf_webradio_player"},
	}
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
		errorResponse := map[string]string{
			"error":   "API Error",
			"message": err.Error(),
		}
		jsonResp, _ := json.Marshal(errorResponse)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonResp)
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, User-Agent, Cache-Control, Pragma")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := w.Write(data); err != nil {
		log.Printf("Error writing response: %v", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		return
	}

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
	station, ok := stationMap[param]
	if !ok {
		return nil, fmt.Errorf("unknown station: %s", param)
	}

	url := fmt.Sprintf("%s/%d/%s", baseURL, station.ID, station.Format)
	log.Printf("Fetching data from: %s\n", url)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", param, err)
	}
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching data for %s: %v", param, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response code for %s: %d", param, resp.StatusCode)
	}

	// Handle gzip-compressed responses
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader for %s: %v", param, err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading response body for %s: %v", param, err)
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(data, &jsonResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON response for %s: %v", param, err)
	}

	if jsonResponse == nil {
		return nil, fmt.Errorf("received null response from FIP API for %s", param)
	}

	// Inject stationName for backward compatibility
	jsonResponse["stationName"] = param

	enriched, err := json.Marshal(jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("error marshalling enriched response for %s: %v", param, err)
	}

	return enriched, nil
}

func generateETag(data []byte) string {
	// Generate a SHA-256 hash of the JSON data
	hash := sha256.Sum256(data)
	etag := hex.EncodeToString(hash[:])
	return "\"" + etag + "\""
}
