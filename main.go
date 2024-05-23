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
    router.HandleFunc("/api/metadata/{param}", handler).Methods("GET")
    log.Fatal(http.ListenAndServe(":8080", router))
}

func handler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    fipParam, ok := vars["param"]
    if !ok {
        http.Error(w, "Missing 'param' parameter", http.StatusBadRequest)
        return
    }

    data, err := getCachedData(fipParam)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}

func getCachedData(param string) ([]byte, error) {
    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    // Check if data is cached and still valid
    if cachedResponse, found := cache[param]; found {
        if time.Since(cachedResponse.CachedAt) < cacheTTL {
            return cachedResponse.Data, nil
        }
        // Remove stale cache
        delete(cache, param)
    }

    // Fetch new data
    url := fmt.Sprintf("https://www.radiofrance.fr/fip/api/live/webradios/%s", param)
    resp, err := http.Get(url)
    if err != nil {
        return nil, fmt.Errorf("error fetching data: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
    }

    data, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %v", err)
    }

    // Cache the new data
    cache[param] = CachedResponse{Data: data, CachedAt: time.Now()}

    return data, nil
}
