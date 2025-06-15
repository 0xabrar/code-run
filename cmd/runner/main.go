package main

import (
    "bytes"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/acme/coderunner/internal/models"
    "github.com/acme/coderunner/internal/sandbox"
)

var (
    dispatcherURL = getenv("DISPATCHER_URL", "http://coderunner-dispatcher")
    nsjailPath    = getenv("NSJAIL_BIN", "/usr/sbin/nsjail")
    supportedLang = getenv("LANGUAGE", "go")
)

func getenv(k, def string) string {
    v := os.Getenv(k)
    if v == "" {
        return def
    }
    return v
}

func main() {
    log.Printf("runner connecting to dispatcher at %s", dispatcherURL)
    for {
        job, ok := fetchJob()
        if !ok {
            time.Sleep(1 * time.Second)
            continue
        }
        processJob(job)
    }
}

func fetchJob() (models.Job, bool) {
    url := dispatcherURL + "/queue/next?lang=" + supportedLang
    resp, err := http.Get(url)
    if err != nil {
        log.Printf("error fetching job: %v", err)
        return models.Job{}, false
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusNoContent {
        return models.Job{}, false
    }
    var job models.Job
    if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
        log.Printf("decode job error: %v", err)
        return models.Job{}, false
    }
    return job, true
}

func processJob(job models.Job) {
    log.Printf("processing job %s with %d tests", job.ID, len(job.Request.Tests))

    results := make([]models.RunResult, 0, len(job.Request.Tests))

    for _, tc := range job.Request.Tests {
        stdout, stderr, exitCode, timedOut, oom, err := sandbox.Run(job.Request.Language, job.Request.Code, tc.Stdin, nsjailPath)
        if err != nil {
            log.Printf("job %s test failed: %v", job.ID, err)
        }
        results = append(results, models.RunResult{
            Stdin:     tc.Stdin,
            Stdout:    stdout,
            Stderr:    stderr,
            ExitCode:  exitCode,
            Expected:  tc.Expected,
            TimedOut:  timedOut,
            OOMKilled: oom,
        })
    }

    // send results back
    payload, _ := json.Marshal(results)
    url := dispatcherURL + "/result/" + job.ID
    resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
    if err != nil {
        log.Printf("error posting results: %v", err)
        return
    }
    resp.Body.Close()
    log.Printf("job %s completed", job.ID)
} 