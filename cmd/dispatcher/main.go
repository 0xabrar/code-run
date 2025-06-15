package main

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "log"
    "net/http"
    "sync"

    "github.com/acme/coderunner/internal/models"
    "github.com/acme/coderunner/internal/queue"
)

var (
    // queues keyed by language identifier
    queues = struct {
        sync.RWMutex
        m map[string]*queue.Queue
    }{m: make(map[string]*queue.Queue)}

    resultStore = struct {
        sync.RWMutex
        m map[string][]models.RunResult
    }{m: make(map[string][]models.RunResult)}
)

// getQueue returns the queue for a language, creating it lazily.
func getQueue(lang string) *queue.Queue {
    queues.RLock()
    q, found := queues.m[lang]
    queues.RUnlock()
    if found {
        return q
    }
    queues.Lock()
    defer queues.Unlock()
    if q, found = queues.m[lang]; found {
        return q
    }
    q = queue.New(100)
    queues.m[lang] = q
    return q
}

func main() {
    mux := http.NewServeMux()

    mux.HandleFunc("/run", handleRun)
    mux.HandleFunc("/status/", handleStatus)
    mux.HandleFunc("/queue/next", handleNext)    // runners provide ?lang=go
    mux.HandleFunc("/result/", handleResult)     // runners post back

    log.Println("dispatcher listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}

// handleRun accepts client code run requests.
func handleRun(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req models.RunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    jobID := generateID()
    job := models.Job{ID: jobID, Request: req}
    getQueue(req.Language).Enqueue(job)

    resp := map[string]string{
        "jobID":  jobID,
        "status": "/status/" + jobID,
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    _ = json.NewEncoder(w).Encode(resp)
}

// handleStatus returns results or pending message.
func handleStatus(w http.ResponseWriter, r *http.Request) {
    jobID := r.URL.Path[len("/status/"):]

    resultStore.RLock()
    res, found := resultStore.m[jobID]
    resultStore.RUnlock()

    if !found {
        http.Error(w, "job not found or not finished", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(res)
}

// handleNext is consumed by runner pods to fetch a job.
func handleNext(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    lang := r.URL.Query().Get("lang")
    if lang == "" {
        http.Error(w, "lang query parameter required", http.StatusBadRequest)
        return
    }
    q := getQueue(lang)

    select {
    case job := <-q.Chan():
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(job)
    default:
        w.WriteHeader(http.StatusNoContent)
    }
}

// handleResult accepts results from runner.
func handleResult(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    jobID := r.URL.Path[len("/result/"):]

    var results []models.RunResult
    if err := json.NewDecoder(r.Body).Decode(&results); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    resultStore.Lock()
    resultStore.m[jobID] = results
    resultStore.Unlock()

    w.WriteHeader(http.StatusNoContent)
}

// generateID creates a 16-byte random hex string.
func generateID() string {
    var b [16]byte
    if _, err := rand.Read(b[:]); err != nil {
        log.Printf("rand read error: %v", err)
    }
    return hex.EncodeToString(b[:])
} 