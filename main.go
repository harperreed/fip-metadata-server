package main

import (
    // "encoding/json"
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
    Data      []byte
    CachedAt  time.Time
}

var (
    cache      = make(map[string]CachedResponse)
    cacheMutex sync.Mutex
    cacheTTL   = 5 * time.Minute // Cache Time-To-Live
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
    data, err := getCachedData(fipParam)
    if err != nil {
        log.Printf("Error fetching data for param: %s, error: %v\n", fipParam, err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}

func getCachedData(param string) ([]byte, error) {
    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    log.Printf("Checking cache for param: %s\n", param)

    // Check if data is cached and still valid
    if cachedResponse, found := cache[param]; found {
        if time.Since(cachedResponse.CachedAt) < cacheTTL {
            log.Printf("Cache hit for param: %s\n", param)
            return cachedResponse.Data, nil
        }
        // Remove stale cache
        log.Printf("Cache expired for param: %s, fetching new data\n", param)
        delete(cache, param)
    } else {
        log.Printf("Cache miss for param: %s\n", param)
    }

    // Fetch new data
    url := fmt.Sprintf("https://www.radiofrance.fr/fip/api/live/webradios/%s", param)
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

    // Cache the new data
    cache[param] = CachedResponse{Data: data, CachedAt: time.Now()}
    log.Printf("New data cached for param: %s\n", param)

    return data, nil
}
