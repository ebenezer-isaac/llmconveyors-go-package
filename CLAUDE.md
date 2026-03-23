# llmconveyors — Go CLI & SDK

Official Go CLI for the LLM Conveyors AI Agent Platform. Distributed as native binaries (Linux `.deb`/`.rpm`, macOS Homebrew, Windows `.exe`, Snap).

## Architecture

- **CLI framework**: `github.com/spf13/cobra` — subcommand tree
- **Config**: `github.com/spf13/viper` — env vars, `~/.llmconveyors/config.yaml`, flags
- **HTTP**: stdlib `net/http` — no external HTTP libraries
- **JSON**: stdlib `encoding/json` — struct tags for marshal/unmarshal
- **SSE**: stdlib `bufio.Scanner` — line-by-line SSE parser
- **Release**: GoReleaser — cross-compile + `.deb`/`.rpm`/Homebrew/Snap/archives
- **Testing**: stdlib `testing` + `net/http/httptest` — table-driven tests

**Zero heavy dependencies.** Only cobra and viper. Everything else is stdlib.

## Project Structure

```
llmconveyors-go/
  main.go                       — Entry: calls cmd.Execute()
  cmd/
    root.go                     — Root command, global flags (--api-key, --base-url, --output)
    agents.go                   — `llmc run job-hunter`, `llmc run b2b-sales`, `llmc status`
    sessions.go                 — `llmc sessions list|get|delete|hydrate`
    resume.go                   — `llmc resume list|get|create|update|delete|render|themes`
    ats.go                      — `llmc ats score`
    settings.go                 — `llmc settings profile|preferences|usage|api-keys`
    upload.go                   — `llmc upload resume|job|job-text`
    stream.go                   — `llmc stream <generationId>` (raw SSE stream to stdout)
    health.go                   — `llmc health`
    version.go                  — `llmc version` (injected at build via ldflags)
    config_cmd.go               — `llmc config set|get|init` (manage ~/.llmconveyors/config.yaml)
  internal/
    client/
      client.go                 — LLMConveyorsClient struct, request/response helpers
      errors.go                 — Typed error types matching API error codes
      retry.go                  — Exponential backoff with jitter
    config/
      config.go                 — Viper setup: env > config file > flags
    sse/
      parser.go                 — SSE line parser (event, data, id fields)
    output/
      format.go                 — Output formatting: json, table, text (--output flag)
  .goreleaser.yml               — Cross-platform release config
  .github/
    workflows/
      ci.yml                    — Test + lint on PR
      release.yml               — GoReleaser on tag push
```

## API Reference

### Base URL & Auth

```
Base URL: https://api.llmconveyors.com/api/v1
Auth header: X-API-Key: llmc_...
```

API keys MUST start with `llmc_` prefix. Validate this client-side before making requests.

### Response Envelope

ALL JSON responses (except SSE streams and binary downloads) are wrapped:

**Success (2xx):**
```json
{
  "success": true,
  "data": { ... },
  "requestId": "uuid",
  "timestamp": "ISO-8601"
}
```

**Error (4xx/5xx):**
```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "hint": "Developer guidance (optional)",
    "details": { "fieldErrors": { "field": ["error1"] } }
  },
  "requestId": "uuid",
  "timestamp": "ISO-8601",
  "path": "/api/v1/..."
}
```

**Go struct:**
```go
type APIResponse[T any] struct {
    Success   bool   `json:"success"`
    Data      T      `json:"data"`
    RequestID string `json:"requestId,omitempty"`
    Timestamp string `json:"timestamp,omitempty"`
}

type APIError struct {
    Success   bool        `json:"success"`
    Error     ErrorDetail `json:"error"`
    RequestID string      `json:"requestId,omitempty"`
    Timestamp string      `json:"timestamp,omitempty"`
    Path      string      `json:"path,omitempty"`
}

type ErrorDetail struct {
    Code    string                   `json:"code"`
    Message string                   `json:"message"`
    Hint    string                   `json:"hint,omitempty"`
    Details *ErrorDetails            `json:"details,omitempty"`
}

type ErrorDetails struct {
    FieldErrors map[string][]string `json:"fieldErrors,omitempty"`
}
```

### Error Codes (17 API + 6 client-side)

| Code | HTTP | Retryable | Description |
|------|------|-----------|-------------|
| `VALIDATION_ERROR` | 400 | No | Invalid request body |
| `UNAUTHORIZED` | 401 | No | Missing or invalid API key |
| `FORBIDDEN` | 403 | No | Key valid but action not allowed |
| `INSUFFICIENT_SCOPE` | 403 | No | Key missing required scope |
| `NOT_FOUND` | 404 | No | Resource not found |
| `UNKNOWN_AGENT` | 404 | No | Invalid agent type |
| `CONFLICT` | 409 | No | Duplicate operation |
| `INSUFFICIENT_CREDITS` | 402 | No | Not enough credits |
| `RATE_LIMITED` | 429 | **Yes** | Too many requests (respect Retry-After header) |
| `CONCURRENT_GENERATION_LIMIT` | 429 | **Yes** | Parallel gen limit hit |
| `INTERNAL_ERROR` | 500 | No | Server error |
| `AI_PROVIDER_ERROR` | 502 | **Yes** | Upstream AI provider failed |
| `GENERATION_TIMEOUT` | 504 | **Yes** | Generation took too long |
| `SERVER_RESTARTING` | — | **Yes** | SSE-only: server restarting |
| `STREAM_NOT_FOUND` | — | No | SSE-only: generation ID invalid |
| `STREAM_ERROR` | — | **Yes** | SSE-only: stream broken |
| `SESSION_DELETED` | — | No | SSE-only: session was deleted |

Client-side codes (not from API): `INVALID_AGENT_TYPE`, `MALFORMED_RESPONSE`, `INTERACTION_HANDLER_REQUIRED`, `STREAM_INCOMPLETE`, `ABORTED`, `POLL_TIMEOUT`.

### Retry Logic

- **Retryable codes**: `RATE_LIMITED`, `CONCURRENT_GENERATION_LIMIT`, `AI_PROVIDER_ERROR`, `GENERATION_TIMEOUT`, `SERVER_RESTARTING`, `STREAM_ERROR`
- **Also retry**: HTTP 502, 503, 504 and network errors (connection refused, timeout, DNS)
- **Max retries**: 3 (configurable)
- **Backoff**: exponential — `min(baseDelay * 2^attempt, maxDelay) + jitter`
- **Base delay**: 1s, Max delay: 30s, Jitter: 0-500ms random
- **Retry-After header**: If present on 429, use it instead of calculated backoff
- **Rate limit headers**: Parse `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

### Endpoints

#### Health (no auth required)

```
GET /health         → { status, timestamp, uptime, version, checks: { mongo, redis }, memory }
GET /health/ready   → 200 or 503
GET /health/live    → 200
```

**CLI**: `llmc health`

#### Agent Generation

```
POST /agents/:agentType/generate    → 202 { jobId, generationId, sessionId, status: "queued", streamUrl }
GET  /agents/:agentType/status/:jobId → 200 { jobId, generationId, sessionId, agentType, status, progress, currentStep, logs?, artifacts?, failedReason?, interactionData?, result, createdAt, completedAt? }
POST /agents/:agentType/interact    → 202 { success, jobId?, streamUrl? }
GET  /agents/:agentType/manifest    → 200 { agent capabilities }
```

**Agent types**: `job-hunter`, `b2b-sales`

**Job Hunter generate body (minimal):**
```json
{
  "companyName": "string (required, max 200)",
  "jobTitle": "string (required, max 512)",
  "jobDescription": "string (required, max 50KB)",
  "companyWebsite": "string (required, URL)"
}
```

Optional fields: `masterResumeId`, `tier` (free|byo), `model` (flash|pro), `sessionId`, `generationId`, `contactName`, `contactEmail`, `theme`, `mode` (standard|cold_outreach), `autoSelectContacts`, `skipResearchCache`, `originalCV`, `extensiveCV`, `cvStrategy`, `coverLetterStrategy`, `coldEmailStrategy`, `reconStrategy`.

**B2B Sales generate body (minimal):**
```json
{
  "companyName": "string (required, max 200)",
  "companyWebsite": "string (required, URL)"
}
```

Optional fields: `masterResumeId`, `tier`, `model`, `sessionId`, `generationId`, `userCompanyContext`, `targetCompanyContext`, `contactName`, `contactEmail`, `salesStrategy`, `reconStrategy`, `companyResearch`, `researchMode` (parallel|sequential), `skipResearchCache`, `senderName`.

**Status polling**: `GET /agents/:agentType/status/:jobId`
- Status values: `queued`, `processing`, `completed`, `failed`, `awaiting_input`
- Query params: `include=logs`, `include=artifacts` (comma-separated)

**Interact (resume phased generation):**
```json
{
  "generationId": "from Phase A",
  "sessionId": "from original generation",
  "interactionType": "e.g., contact_selection",
  "interactionData": { "field": "value" }
}
```

**CLI**:
```bash
llmc run job-hunter --company "Acme" --title "SWE" --jd "..." --website "https://acme.com"
llmc run b2b-sales --company "Acme" --website "https://acme.com"
llmc status <jobId> --agent job-hunter --watch          # poll until terminal
llmc status <jobId> --agent job-hunter --include logs,artifacts
llmc interact --agent job-hunter --generation-id X --session-id Y --type contact_selection --data '{"selectedContactIds": ["id1"]}'
llmc manifest job-hunter
```

The `run` command should:
1. POST to `/agents/:agentType/generate`
2. Print `jobId`, `sessionId`, `generationId`
3. If `--stream` flag (default): connect to SSE endpoint and stream progress/chunks to stdout
4. If `--poll` flag: poll status endpoint every 2s until terminal state
5. If `--no-wait` flag: just print the IDs and exit
6. On `awaiting_input`: print interaction data and exit with code 2 (special exit code)

#### SSE Streaming

```
GET /stream/generation/:generationId
Headers: X-API-Key: llmc_..., Last-Event-ID: <number> (optional, for reconnection)
Response: text/event-stream
```

**Event format** (each event is newline-delimited):
```
event: progress
data: {"event":"progress","data":{"jobId":"...","sessionId":"...","step":"research","percent":25,"message":"Researching company..."}}

event: chunk
data: {"event":"chunk","data":{"jobId":"...","sessionId":"...","chunk":"text fragment","index":0}}

event: log
data: {"event":"log","data":{"messageId":"...","generationId":"...","sessionId":"...","content":"Log message","level":"info","timestamp":"ISO-8601"}}

event: complete
data: {"event":"complete","data":{"jobId":"...","sessionId":"...","success":true,"artifacts":[...],"mergedArtifactState":{...}}}

event: error
data: {"event":"error","data":{"jobId":"...","sessionId":"...","code":"AI_PROVIDER_ERROR","message":"..."}}

event: heartbeat
data: {"event":"heartbeat","data":{"jobId":"...","sessionId":"...","timestamp":"ISO-8601"}}
```

**SSE parser** (`internal/sse/parser.go`):
- Read lines with `bufio.Scanner`
- Lines starting with `event:` set the event type
- Lines starting with `data:` contain JSON payload
- Empty line = end of event, dispatch it
- Lines starting with `:` are comments (ignore)
- `Last-Event-ID` support: track the last `id:` field, send on reconnect
- Reconnect on disconnect with exponential backoff (reuse retry logic)
- Silently ignore `heartbeat` events

**CLI streaming output**:
- `progress` → print colored progress bar: `[25%] research: Researching company...`
- `chunk` → print text fragment to stdout (no newline between chunks)
- `log` → print to stderr: `[INFO] Log message`
- `complete` → print summary, exit 0
- `error` → print error to stderr, exit 1
- `heartbeat` → ignore

#### Sessions

```
GET    /sessions                → { data: Session[], total, page, limit }
POST   /sessions                → 201 { session }
GET    /sessions/:id            → { session }
DELETE /sessions/:id            → { success }
GET    /sessions/:id/hydrate    → { session with full artifacts }
GET    /sessions/:id/download?key=<artifact-key> → binary file
```

**CLI**:
```bash
llmc sessions list [--page 1] [--limit 20] [--agent job-hunter]
llmc sessions get <id>
llmc sessions delete <id>
llmc sessions hydrate <id>
llmc sessions download <id> --key cv/resume.pdf --output ./resume.pdf
```

#### Resume Management

```
GET    /resume/master           → { masters: MasterResume[] }
POST   /resume/master           → 201 { master }
GET    /resume/master/:id       → { master }
PUT    /resume/master/:id       → { master }
DELETE /resume/master/:id       → { success }
POST   /resume/render           → { url } (PDF render)
POST   /resume/preview          → { html }
GET    /resume/themes           → { themes: Theme[] }
POST   /resume/validate         → { valid, errors? }
```

**CLI**:
```bash
llmc resume list
llmc resume get <id>
llmc resume create --file ./resume.json
llmc resume update <id> --file ./resume.json
llmc resume delete <id>
llmc resume render --file ./resume.json --theme modern [--output ./resume.pdf]
llmc resume themes
```

#### ATS Scoring

```
POST /ats/score → { dimensions, overall, breakdown }
```

Body: `{ resumeText: string, jobDescription: string }` (or `masterResumeId` + JD)

**CLI**:
```bash
llmc ats score --resume-file ./resume.txt --jd-file ./job.txt
llmc ats score --resume-id <masterId> --jd "paste JD here"
```

#### Upload

```
POST /upload/resume     → multipart/form-data { parsed resume }
POST /upload/job        → multipart/form-data { parsed JD }
POST /upload/job-text   → JSON { text, source? } → { parsed JD }
```

**CLI**:
```bash
llmc upload resume ./resume.pdf
llmc upload job ./job-description.pdf
llmc upload job-text --file ./jd.txt
llmc upload job-text --text "paste JD here"
```

#### Settings

```
GET  /settings/profile                      → { user profile }
GET  /settings/preferences                  → { preferences }
POST /settings/preferences                  → { updated preferences }
GET  /settings/usage-summary                → { credits, tier, usage }
GET  /settings/usage-logs?limit=50&offset=0 → { logs[], total }
POST /settings/platform-api-keys            → { key (shown once), hash }
GET  /settings/platform-api-keys            → [{ hash, name, scopes, createdAt }]
DELETE /settings/platform-api-keys/:hash    → { success }
POST /settings/platform-api-keys/:hash/rotate → { newKey (shown once) }
```

**CLI**:
```bash
llmc settings profile
llmc settings preferences [--set key=value]
llmc settings usage
llmc settings usage-logs [--limit 50]
llmc api-keys list
llmc api-keys create --name "my-key" --scopes jobs:write,jobs:read,sessions:read
llmc api-keys revoke <hash>
llmc api-keys rotate <hash>
```

## Command Tree Summary

```
llmc
  run <agent-type>              — Start generation (job-hunter, b2b-sales)
  status <jobId>                — Check generation status
  interact                      — Resume phased generation
  manifest <agent-type>         — Show agent capabilities
  stream <generationId>         — Raw SSE stream to stdout
  sessions
    list                        — List sessions
    get <id>                    — Get session details
    delete <id>                 — Delete session
    hydrate <id>                — Full session with artifacts
    download <id>               — Download artifact file
  resume
    list                        — List master resumes
    get <id>                    — Get master resume
    create                      — Create master resume
    update <id>                 — Update master resume
    delete <id>                 — Delete master resume
    render                      — Render resume to PDF
    themes                      — List available themes
  ats
    score                       — Score resume against JD
  upload
    resume <file>               — Upload and parse resume
    job <file>                  — Upload and parse JD
    job-text                    — Parse JD from text
  settings
    profile                     — Show user profile
    preferences                 — Get/set preferences
    usage                       — Credit usage summary
    usage-logs                  — Detailed usage log
  api-keys
    list                        — List API keys
    create                      — Create new API key
    revoke <hash>               — Revoke API key
    rotate <hash>               — Rotate API key
  health                        — API health check
  config
    init                        — Create config file interactively
    set <key> <value>           — Set config value
    get <key>                   — Get config value
  version                       — Print version
```

## Global Flags

```
--api-key string       API key (overrides LLMC_API_KEY env var and config file)
--base-url string      API base URL (default: https://api.llmconveyors.com/api/v1)
--output string        Output format: json, table, text (default: text)
--debug                Enable debug logging (print HTTP requests/responses to stderr)
--no-color             Disable colored output
--timeout duration     Request timeout (default: 30s)
--config string        Config file path (default: ~/.llmconveyors/config.yaml)
```

## Config File (`~/.llmconveyors/config.yaml`)

```yaml
api_key: llmc_...
base_url: https://api.llmconveyors.com/api/v1
output: text              # json | table | text
timeout: 30s
max_retries: 3
debug: false
no_color: false
```

**Priority**: flag > env var (`LLMC_API_KEY`, `LLMC_BASE_URL`) > config file > default

`llmc config init` creates this file interactively (prompts for API key).

## Output Formatting

Three output modes controlled by `--output` flag:

### `text` (default) — human-friendly
```
Job started successfully
  Job ID:         abc-123
  Session ID:     def-456
  Generation ID:  ghi-789
  Status:         queued
  Stream URL:     /api/v1/stream/generation/ghi-789
```

### `json` — machine-readable (raw API response data, unwrapped from envelope)
```json
{"jobId":"abc-123","sessionId":"def-456","generationId":"ghi-789","status":"queued","streamUrl":"/api/v1/stream/generation/ghi-789"}
```

### `table` — tabular (for list operations)
```
HASH       NAME       SCOPES              CREATED
a1b2c3     my-key     jobs:write,jobs:read 2026-03-23T10:00:00Z
d4e5f6     ci-key     sessions:read        2026-03-22T15:30:00Z
```

## Build & Release

### Local development
```bash
go build -o llmc .
./llmc health
./llmc run job-hunter --company "Acme" --title "SWE" --jd "..." --website "https://acme.com" --stream
```

### Version injection
```bash
go build -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o llmc .
```

### GoReleaser (`.goreleaser.yml`)

The `.goreleaser.yml` should produce:
- **Archives**: `tar.gz` (Linux/macOS), `zip` (Windows) — for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- **Debian package**: `.deb` with `llmc` binary in `/usr/bin/`
- **RPM package**: `.rpm` with `llmc` binary in `/usr/bin/`
- **Homebrew**: tap formula at `llmconveyors/homebrew-tap`
- **Snap**: `llmc` snap package
- **Checksums**: SHA256 checksums file
- **Changelog**: auto from conventional commits

### GitHub Actions

**`.github/workflows/ci.yml`** (on PR):
- `go vet ./...`
- `go test ./... -race -count=1`
- `golangci-lint run`

**`.github/workflows/release.yml`** (on tag `v*`):
- GoReleaser with `--clean`
- Publishes to GitHub Releases
- Updates Homebrew tap

### Installation methods (for README)

```bash
# Homebrew (macOS + Linux)
brew install llmconveyors/tap/llmc

# Debian/Ubuntu
curl -LO https://github.com/llmconveyors/cli/releases/latest/download/llmc_amd64.deb
sudo dpkg -i llmc_amd64.deb

# RPM (Fedora/RHEL)
sudo rpm -i https://github.com/llmconveyors/cli/releases/latest/download/llmc_amd64.rpm

# Snap
sudo snap install llmc

# Go install
go install github.com/llmconveyors/cli@latest

# Binary download
curl -LO https://github.com/llmconveyors/cli/releases/latest/download/llmc_linux_amd64.tar.gz
tar xzf llmc_linux_amd64.tar.gz
sudo mv llmc /usr/local/bin/
```

## Implementation Order

Build in this order — each step produces a compilable, testable increment:

### Phase 1: Core HTTP client + config
1. `internal/config/config.go` — viper setup, env/config/flag precedence
2. `internal/client/client.go` — `Client` struct, `Get`, `Post`, `Delete`, `Put` methods, response envelope unwrapping
3. `internal/client/errors.go` — typed error hierarchy: `APIError` base + per-code types, `parseErrorResponse` factory
4. `internal/client/retry.go` — exponential backoff, jitter, retryable detection, Retry-After parsing
5. `internal/output/format.go` — json/table/text formatters
6. `cmd/root.go` — cobra root, global flags, viper binding, client init in PersistentPreRun
7. `cmd/version.go` — version command with ldflags
8. `cmd/health.go` — health command (simplest endpoint, good first test)
9. `cmd/config_cmd.go` — config init/set/get

### Phase 2: Agent commands (core workflow)
10. `cmd/agents.go` — `run`, `status`, `interact`, `manifest` commands
11. `internal/sse/parser.go` — SSE line parser
12. `cmd/stream.go` — raw stream command + integration into `run --stream`

### Phase 3: Resource commands
13. `cmd/sessions.go` — list, get, delete, hydrate, download
14. `cmd/resume.go` — CRUD + render + themes
15. `cmd/ats.go` — score command
16. `cmd/upload.go` — resume, job, job-text (multipart for files)
17. `cmd/settings.go` — profile, preferences, usage, usage-logs
18. `cmd/api_keys.go` — list, create, revoke, rotate (split from settings for cleaner UX)

### Phase 4: Polish & release
19. Tests — table-driven tests for every command using httptest mock server
20. `.goreleaser.yml` — full release config
21. `.github/workflows/ci.yml` + `release.yml`
22. `README.md` — installation, quickstart, examples

## Testing Strategy

- **Table-driven tests** for all HTTP client methods (status codes, error parsing, retry behavior)
- **httptest.Server** for mock API — return canned responses, verify request shapes
- **SSE parser tests** — multi-event streams, reconnection, malformed data
- **CLI integration tests** — capture stdout/stderr, verify exit codes
- **No external test dependencies** — stdlib `testing` only

Example pattern:
```go
func TestClientGetStatus(t *testing.T) {
    tests := []struct {
        name       string
        statusCode int
        body       string
        wantErr    bool
        wantCode   string
    }{
        {"success", 200, `{"success":true,"data":{"status":"completed"}}`, false, ""},
        {"not found", 404, `{"success":false,"error":{"code":"NOT_FOUND","message":"Job not found"}}`, true, "NOT_FOUND"},
        {"rate limited", 429, `{"success":false,"error":{"code":"RATE_LIMITED","message":"Slow down"}}`, true, "RATE_LIMITED"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(tt.statusCode)
                w.Write([]byte(tt.body))
            }))
            defer srv.Close()
            // ... test client against srv.URL
        })
    }
}
```

## API Key Scopes Reference

| Scope | Endpoints |
|-------|-----------|
| `jobs:write` | POST /agents/job-hunter/generate, POST /agents/job-hunter/interact |
| `jobs:read` | GET /agents/job-hunter/status/:jobId, GET /stream/generation/:id |
| `sales:write` | POST /agents/b2b-sales/generate |
| `sales:read` | GET /agents/b2b-sales/status/:jobId |
| `sessions:read` | GET /sessions, GET /sessions/:id, GET /sessions/:id/hydrate, GET /sessions/:id/download |
| `sessions:write` | POST /sessions, DELETE /sessions/:id |
| `resume:read` | GET /resume/master, GET /resume/master/:id, GET /resume/themes |
| `resume:write` | POST /resume/master, PUT /resume/master/:id, DELETE /resume/master/:id, POST /resume/render, POST /resume/validate |
| `upload:write` | POST /upload/resume, POST /upload/job, POST /upload/job-text |
| `settings:read` | GET /settings/profile, GET /settings/preferences, GET /settings/usage-summary, GET /settings/usage-logs, GET /settings/platform-api-keys |
| `settings:write` | POST /settings/preferences, POST /settings/platform-api-keys, DELETE /settings/platform-api-keys/:hash, POST /settings/platform-api-keys/:hash/rotate |
| `ats:write` | POST /ats/score |
| `webhook:read` | GET /settings/webhook-secret |
| `webhook:write` | POST /settings/webhook-secret/rotate |

## Rules

- One Go file per resource group in `cmd/` — no mega-files
- All HTTP calls go through `internal/client/client.go` — never raw `http.Get` in commands
- Immutable structs — use value receivers, return new structs, never mutate in place
- All errors returned, never silently swallowed — explicit error handling at every level
- Exit code 0 = success, 1 = error, 2 = awaiting_input (special for phased workflows)
- Debug output goes to stderr, data output goes to stdout (enables piping: `llmc sessions list --output json | jq`)
- API key validated client-side before first request (must start with `llmc_`)
- No `fmt.Println` in command handlers — use the output formatter for data, `cmd.PrintErr` for errors
- Context propagation — pass `context.Context` through all calls for timeout/cancellation
- User-Agent header: `llmconveyors-go/<version>` on all requests
