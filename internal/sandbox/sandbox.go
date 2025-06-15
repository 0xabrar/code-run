package sandbox

import (
    "bytes"
    "context"
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "time"
)

const (
    wallTimeout = 5 * time.Second      // 5s max execution per test
    memoryLimit = 256 * 1024 * 1024    // 256 MiB
)

// Run executes the supplied code in the specified language.
// Supported languages: "go", "python", "javascript" (node).
func Run(language, code, stdin, nsjailPath string) (stdout string, stderr string, exitCode int, timedOut bool, oom bool, err error) {
    workDir, err := ioutil.TempDir("", "coderunner-")
    if err != nil {
        return
    }
    defer os.RemoveAll(workDir)

    var execPath string
    var buildCmd *exec.Cmd

    switch language {
    case "go":
        sourcePath := filepath.Join(workDir, "main.go")
        if err = os.WriteFile(sourcePath, []byte(code), 0644); err != nil {
            return
        }
        execPath = filepath.Join(workDir, "userprog")
        buildCmd = exec.Command("go", "build", "-o", execPath, sourcePath)
    case "python":
        execPath = filepath.Join(workDir, "script.py")
        if err = os.WriteFile(execPath, []byte(code), 0644); err != nil {
            return
        }
        // no compile step
    case "javascript", "js":
        execPath = filepath.Join(workDir, "script.js")
        if err = os.WriteFile(execPath, []byte(code), 0644); err != nil {
            return
        }
        // no compile
    default:
        err = fmt.Errorf("unsupported language: %s", language)
        return
    }

    // If build needed (Go)
    if buildCmd != nil {
        var compileBuf bytes.Buffer
        buildCmd.Stdout = &compileBuf
        buildCmd.Stderr = &compileBuf
        if err = buildCmd.Run(); err != nil {
            stderr = compileBuf.String()
            return
        }
    }

    // Build execution command
    var innerCmd []string
    switch language {
    case "go":
        innerCmd = []string{execPath}
    case "python":
        innerCmd = []string{"/usr/local/bin/python3", execPath}
    case "javascript", "js":
        innerCmd = []string{"/usr/local/bin/node", execPath}
    }

    ctx, cancel := context.WithTimeout(context.Background(), wallTimeout)
    defer cancel()

    args := []string{
        "--time", fmt.Sprint(int(wallTimeout.Seconds())),
        "--rlimit_as", fmt.Sprint(memoryLimit),
        "--disable_clone_newnet",
        "--disable_clone_newns",
        "--quiet",
        "--"}
    args = append(args, innerCmd...)

    runCmd := exec.CommandContext(ctx, nsjailPath, args...)
    runCmd.Dir = workDir
    runCmd.Stdin = bytes.NewBufferString(stdin)

    var outBuf, errBuf bytes.Buffer
    runCmd.Stdout = &outBuf
    runCmd.Stderr = &errBuf

    err = runCmd.Run()
    stdout = outBuf.String()
    stderr = errBuf.String()

    if ctx.Err() == context.DeadlineExceeded {
        timedOut = true
    }

    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    }

    if exitCode == 137 {
        oom = true
    }

    return
} 