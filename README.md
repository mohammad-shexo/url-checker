# URL Status Checker — Golang Microservice

> A production-ready REST microservice that checks the HTTP status of a list of URLs in parallel, with structured error reporting, in-memory caching, and a complete CI/CD pipeline.

---

## Table of Contents

1. [Purpose](#purpose)
2. [Real-World Problem Solved](#real-world-problem-solved)
3. [Why This Service Is Useful](#why-this-service-is-useful)
4. [Architecture Overview](#architecture-overview)
5. [API Reference](#api-reference)
6. [Setup & Running](#setup--running)
7. [Docker Usage](#docker-usage)
8. [Test Strategy](#test-strategy)
9. [CI/CD Pipeline](#cicd-pipeline)
10. [Deployment Notes](#deployment-notes)
11. [Security Considerations](#security-considerations)
12. [Future Improvements](#future-improvements)

---

## Purpose

URL Status Checker is a lightweight Go microservice that accepts a list of URLs via a REST API and returns each URL's HTTP status code, response time, and any errors (timeout, DNS failure, connection refused, invalid URL, etc.).

---

## Real-World Problem Solved

Modern applications, monitoring scripts, and DevOps pipelines regularly need to verify whether a set of URLs (APIs, CDN assets, third-party services, documentation pages) are reachable and returning acceptable HTTP responses. Doing this sequentially is slow; doing it without structure leads to noisy error reporting. This service solves both problems in a single POST call.

**Typical use cases:**

- Uptime monitoring sidecars
- Pre-deployment smoke-test runners
- Link-rot detectors for documentation sites
- Health-check aggregators in Kubernetes init containers
- CI pipeline quality gates validating external service availability

---

## Why This Service Is Useful

| Capability | Benefit |
|---|---|
| Parallel URL checking | 50 URLs checked in ~3 s instead of 150 s |
| Structured error typing | Downstream systems can act on `dns`, `timeout`, `network` differently |
| In-memory caching | Reduces redundant traffic for repeated checks |
| Standard library only | Zero heavyweight framework dependencies |
| Multi-stage Docker build | ~12 MB image — fast pull, tiny attack surface |
| Full CI/CD | Auto-release on every push to `main` |

---

## Architecture Overview

```
┌───────────────────────────────────────────────────────────────┐
│                        HTTP Client                            │
│  POST /check  {"urls": ["https://a.com", "https://b.com"]}   │
└──────────────────────────┬────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                    cmd/server/main.go                         │
│   • Wires dependencies (http.Client → Checker → Handler)     │
│   • Starts net/http server on :8080                          │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│              internal/handlers/check.go                       │
│   • Validates HTTP method (POST only)                        │
│   • Decodes JSON body                                        │
│   • Enforces max-50-URL guard                                │
│   • Delegates to Checker service                             │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│             internal/services/checker.go                      │
│                                                              │
│   CheckURLs(urls)                                            │
│      │                                                       │
│      ├─ goroutine 1 ──► checkOne(url[0])                    │
│      ├─ goroutine 2 ──► checkOne(url[1])    ◄── sync.WaitGroup
│      └─ goroutine N ──► checkOne(url[N-1])                  │
│                                                              │
│   checkOne:                                                  │
│      ├─ cache HIT  ──► return cached URLResult              │
│      └─ cache MISS ──► fetch() ──► cache write              │
│                                                              │
│   fetch:                                                     │
│      ├─ validateURL()   (scheme, host checks)               │
│      ├─ http.Client.Head()                                   │
│      └─ classifyError() (dns / timeout / network / unknown) │
└──────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│              internal/utils/http_client.go                    │
│   • NewHTTPClient(timeout) → *http.Client                    │
│   • No-redirect policy                                       │
│   • Configurable TLS + idle-connection pool                  │
└──────────────────────────────────────────────────────────────┘
```

### Directory Layout

```
url-checker/
├── cmd/
│   └── server/
│       └── main.go                  # Entry point
├── internal/
│   ├── handlers/
│   │   └── check.go                 # HTTP handler (POST /check)
│   ├── models/
│   │   └── url.go                   # Request / Response structs
│   ├── services/
│   │   └── checker.go               # Core checking logic + cache
│   └── utils/
│       └── http_client.go           # Reusable HTTP client factory
├── tests/
│   ├── check_handler_test.go        # Handler integration tests
│   └── checker_service_test.go      # Service unit tests
├── .github/
│   └── workflows/
│       └── ci.yml                   # GitHub Actions CI/CD
├── Dockerfile                       # Multi-stage production build
├── go.mod
└── README.md
```

---

## API Reference

### `POST /check`

Check the status of one or more URLs.

**Request**

```http
POST /check
Content-Type: application/json

{
  "urls": [
    "https://google.com",
    "https://does-not-exist.abc",
    "https://httpbin.org/status/500"
  ]
}
```

Constraints:
- `urls` must not be empty
- Maximum **50** URLs per request
- Each URL must use `http` or `https` scheme

**Response `200 OK`**

```json
{
  "results": [
    {
      "url": "https://google.com",
      "status": 301,
      "duration_ms": 84,
      "error": ""
    },
    {
      "url": "https://does-not-exist.abc",
      "status": 0,
      "duration_ms": 3012,
      "error": "[dns] DNS resolution failed"
    },
    {
      "url": "https://httpbin.org/status/500",
      "status": 500,
      "duration_ms": 230,
      "error": ""
    }
  ]
}
```

**Response `400 Bad Request`**

```json
{
  "message": "urls array must not be empty"
}
```

**Response `405 Method Not Allowed`**

```json
{
  "message": "only POST is accepted"
}
```

### `GET /health`

Simple liveness probe.

```json
{"status":"ok"}
```

### Error Kind Reference

| Kind | Meaning |
|---|---|
| `invalid_url` | Malformed URL, empty string, or unsupported scheme |
| `dns` | Host could not be resolved |
| `timeout` | Request exceeded the 3-second deadline |
| `network` | Connection refused or other transport error |
| `unknown` | Uncategorised network error |

---

## Setup & Running

### Prerequisites

- Go 1.21+
- Docker (optional)

### Clone and Run

```bash
git clone https://github.com/<your-org>/url-checker.git
cd url-checker

# Download dependencies
go mod download

# Run the server (default port 8080)
go run ./cmd/server

# Override port
PORT=9090 go run ./cmd/server
```

### Test the API

```bash
curl -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://google.com", "https://github.com"]}'
```

---

## Docker Usage

### Build the image

```bash
docker build -t url-checker:latest .
```

### Run the container

```bash
docker run -p 8080:8080 url-checker:latest
```

### Using the GitHub Container Registry image

```bash
docker pull ghcr.io/<your-org>/url-checker:latest
docker run -p 8080:8080 ghcr.io/<your-org>/url-checker:latest
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | TCP port the server listens on |

---

## Test Strategy

Tests live in `tests/` and are split into two files:

| File | Focus |
|---|---|
| `checker_service_test.go` | Unit tests for the `Checker` service |
| `check_handler_test.go` | Integration tests using `httptest` |

### Test Cases

| # | Test | Verifies |
|---|---|---|
| 1 | Valid URL → 200 | Happy path, duration recorded |
| 2 | Valid URL → 404 | Non-2xx status codes propagated |
| 3 | Valid URL → 500 | Server errors propagated |
| 4 | Invalid URL (bad scheme) | `invalid_url` error kind |
| 5 | Empty URL string | `invalid_url` error kind |
| 6 | Timeout | `timeout` error kind, client TTL respected |
| 7 | Unreachable host | `network` or error returned |
| 8 | Parallel execution | 10 URLs complete in < 150 ms |
| 9 | Caching | Server called only once for 2 identical requests |
| 10 | Result URL = input URL | Result `url` field preserved correctly |
| 11 | Result order preserved | Slice indices match input order |
| 12 | DNS failure | DNS error reported correctly |
| H1 | Handler: valid request | 200 + correct response body |
| H2 | Handler: GET rejected | 405 returned |
| H3 | Handler: empty array | 400 returned |
| H4 | Handler: malformed JSON | 400 returned |
| H5 | Handler: >50 URLs | 400 returned |
| H6 | Handler: Content-Type header | `application/json` set |
| H7 | Handler: multiple results | Correct count returned |
| H8 | Handler: invalid URL in body | Error in result, not HTTP error |

### Run Tests

```bash
# All tests
go test ./tests/... -v

# With race detector
go test -race ./tests/... -v

# With coverage
go test -coverprofile=coverage.out ./tests/...
go tool cover -html=coverage.out
```

---

## CI/CD Pipeline

The pipeline runs on GitHub Actions with three sequential jobs:

```
push to main
     │
     ▼
┌─────────────────────┐
│  build-and-test      │   go vet + go test -race + go build
│  (ubuntu-latest)     │   → uploads binary artifact
└──────────┬──────────┘
           │ needs: build-and-test
           ▼
┌─────────────────────┐
│  docker              │   docker buildx build --push
│  (ubuntu-latest)     │   → ghcr.io/<org>/url-checker:sha-* + :latest
└──────────┬──────────┘
           │ needs: docker
           ▼
┌─────────────────────┐
│  release             │   auto-increments patch semver tag
│  (ubuntu-latest)     │   → creates GitHub Release + attaches binary
└─────────────────────┘
```

**Features:**
- Dependency caching (Go module cache + Docker layer cache via `cache-from: type=gha`)
- Multi-platform Docker build (`linux/amd64` + `linux/arm64`)
- Race-detector enabled test run
- Automatic semver tag increment (v1.0.0 → v1.0.1 → …)
- GitHub Release auto-generated with release notes

---

## Deployment Notes

- The service is **stateless** except for the in-memory cache. Cache is per-process; deploy multiple replicas safely with a load balancer.
- For distributed caching across replicas, replace the in-memory cache with Redis (see [Future Improvements](#future-improvements)).
- The service exposes `/health` for Kubernetes `livenessProbe` and `readinessProbe`.
- The Docker image runs as a non-root user (`appuser`) for compliance with restricted Pod Security Standards.

### Kubernetes Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: url-checker
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: url-checker
          image: ghcr.io/<your-org>/url-checker:latest
          ports:
            - containerPort: 8080
          env:
            - name: PORT
              value: "8080"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 3
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
```

---

## Security Considerations

| Concern | Mitigation |
|---|---|
| SSRF (Server-Side Request Forgery) | Only `http`/`https` URLs allowed; no `file://`, `ftp://`, `localhost` whitelist should be added per deployment policy |
| DoS via large URL lists | Hard-capped at 50 URLs per request |
| Slow-loris / read timeout | Server-level `ReadTimeout: 10s` + per-URL `Timeout: 3s` |
| Container privilege escalation | Non-root user in Docker; no capabilities granted |
| Dependency surface | Only `testify` pulled in (test binary only); no runtime third-party deps |
| Secret leakage | No secrets required at runtime; `GITHUB_TOKEN` used only in CI |

---

## Future Improvements

- **Redis-backed cache** — share cache state across horizontally scaled replicas
- **SSRF blocklist** — reject requests to RFC 1918 / loopback addresses
- **Metrics endpoint** — expose Prometheus metrics (request count, error rates, latency histograms)
- **Webhook support** — POST results to a callback URL when checks complete asynchronously
- **Scheduled checks** — persist a list of URLs and check them on a cron schedule
- **Authentication** — API-key or JWT middleware to restrict access
- **OpenAPI / Swagger spec** — auto-generated from handler annotations
- **Graceful shutdown** — catch `SIGTERM`, drain in-flight goroutines before exit
- **Configurable max URLs** — environment variable override for the 50-URL cap
- **HEAD → GET fallback** — automatically retry with GET when server rejects HEAD
