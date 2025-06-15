package models

// TestCase defines a single test input and expected output pair.
type TestCase struct {
    Stdin    string `json:"stdin"`
    Expected string `json:"expected"`
}

// RunRequest is sent by clients to initiate a code run.
type RunRequest struct {
    Language string     `json:"language"`
    Code     string     `json:"code"`
    Tests    []TestCase `json:"tests"`
}

// RunResult is produced per test case.
type RunResult struct {
    Stdin     string `json:"stdin"`
    Stdout    string `json:"stdout"`
    Stderr    string `json:"stderr"`
    ExitCode  int    `json:"exitCode"`
    Expected  string `json:"expected"`
    TimedOut  bool   `json:"timedOut"`
    OOMKilled bool   `json:"oomKilled"`
}

// Job wraps a RunRequest with an ID.
type Job struct {
    ID      string      `json:"id"`
    Request RunRequest  `json:"request"`
} 