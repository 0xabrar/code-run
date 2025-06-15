# CodeRunner  ![License](https://img.shields.io/badge/license-MIT-blue)

CodeRunner is a lightweight, self-hosted judge service that executes arbitrary code against multiple test-cases in isolated containers.

* Written in Go ‑ only the standard library is used inside the application.
* Runs completely on your own Kubernetes cluster (KinD, k3s, EKS, etc.).
* Supports Go 1.22, Python 3.x and JavaScript (Node 18+) out of the box.
* Uses nsjail to provide cgroups, seccomp, mount & network isolation.

---

## Architecture

```
┌────────────┐      POST /run               ┌────────────┐
│  Client    ├─────────────────────────────►│ Dispatcher │
└────────────┘                              │  Service   │
        ▲                                   └─────┬──────┘
        │                              GET /queue/next│  POST /result
        │                                            │
        │                                            ▼
  GET /status/{id}                            ┌────────────┐
        │                                     │ Runner Pod │   (Go / Python / JS)
        └─────────────────────────────────────┤   + nsjail │
                                              └─────┬──────┘
                                                    │ (nsjail sandbox)
                                                    ▼
                                               Test binaries
```

1. **Dispatcher** – Stateless HTTP service accepting jobs, storing results in-memory and serving a simple pull queue for workers.
2. **Runner Pods** – Long-running Deployments (1-N replicas per language). Each pod:
   * Long-polls `/queue/next` for work.
   * Compiles / interprets code inside an nsjail sandbox for every test.
   * Collects `stdout`, `stderr`, `exitCode`, timeout/OOM flags.
   * POSTs an array of results back to `/result/{jobID}`.

> Because the queue lives only in memory, jobs in-flight will be lost if the Dispatcher restarts. That trade-off keeps the design broker-less and ultra-simple.

---

## Security Hardening

Each test is executed via `nsjail` with **no network**, a readonly rootfs and strict seccomp profile (see `config/seccomp.json`). Resource limits:

* **CPU** – 250 m per pod (configurable in `k8s/runner.yaml`).
* **Memory** – 256 MiB cgroup limit.
* **Wall-clock** – 5 seconds enforced by nsjail.
* **Address-space** – 256 MiB via `--rlimit_as`.

The default seccomp profile allows only a handful of syscalls (`read`, `write`, …). Adjust it as your language runtimes require.

---

## Getting Started

### Quick Start (clone → run)

```bash
# 0. Prerequisites: Docker, Kind, kubectl, go (for tests)

# 1. Create a Kind cluster if you don't have one
kind create cluster                     # one-time

# 2. Build & deploy everything
./scripts/deploy.sh                    # images → Kind → manifests

# 3. Expose API locally
kubectl -n coderunner port-forward svc/coderunner-dispatcher 8080:80
```

Now open a new terminal and try the sample request:

```bash
curl -X POST -H "Content-Type: application/json" --data @sample.json \
     http://localhost:8080/run | jq

# then poll
curl http://localhost:8080/status/<jobID> | jq
```

Or run the automated end-to-end test suite (must keep port-forward running):

```bash
go test ./e2e -v            # runs Go, Python, JS solutions against the API
```

### Example Request

```bash
cat <<'JSON' > sample.json
{
  "language": "javascript",
  "code": "console.log(require('fs').readFileSync(0,'utf8').toUpperCase())",
  "tests": [
    {"stdin": "hello\n", "expected": "HELLO\n"},
    {"stdin": "world\n", "expected": "WORLD\n"}
  ]
}
JSON

# Submit
curl -X POST -H "Content-Type: application/json" --data @sample.json http://localhost:8080/run
# => {"jobID":"…","status":"/status/…"}

# Poll until array of RunResult appears
curl http://localhost:8080/status/<jobID>
```

---

## Extending Language Support

1. Create a new `Dockerfile.runner-<lang>` that compiles/installs your runtime + `/runner` binary.
2. Add an extra clause in `internal/sandbox.Run` for `language == "<lang>"`.
3. Copy `k8s/runner-go.yaml` to `runner-<lang>.yaml`, tweak `image:` and `LANGUAGE` env var.
4. Update `scripts/deploy.sh` to build & deploy the new image.

---

## Developer Notes

* All Go packages sit under `internal/`.
* Back-pressure: each per-language queue is size-100; when full, `/run` returns 503 (TODO).
* Observability: logfmt/JSON out-of-the-box; Prometheus `/metrics` endpoint pending.
* Authentication: design is header-based API key once needed.

---

## License

MIT (c) You
