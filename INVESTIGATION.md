# Blast Radius Investigation — Root Cause Analysis

> Generated 2026-03-23 from 12 parallel Opus agent research vectors.
> Every `.go` file in every module was read line-by-line.

---

## Critical Bugs (Will crash, won't compile, or cause data loss)

### C1. `max()` redeclares Go 1.22 builtin — WILL NOT COMPILE
- **File:** `cmd/sessions.go:170-175`
- **Root cause:** Custom `max(a, b int) int` function defined. Go 1.21+ has a builtin `max`. With `go 1.22` in `go.mod`, this is a **compilation error**.
- **Fix:** Delete lines 170-175. The builtin `max` has the same signature.
- **Blast radius:** `cmd/sessions.go` only. Used on line 71.

### C2. `commandSkipsAuth` matches leaf name — nil-pointer panic on `sessions get`, `resume get`
- **File:** `cmd/root.go:86-90`
- **Root cause:** `cmd.Name()` returns only the leaf command name (e.g., `"get"`), not the full path. The skip list includes `"get"` and `"set"`, which matches `llmc sessions get`, `llmc resume get`, etc. — skipping auth and leaving `apiClient = nil`. Any subsequent `apiClient.Get(...)` call panics.
- **Affected commands that will PANIC:**
  - `llmc sessions get <id>` → `cmd.Name() == "get"` → auth skipped → `apiClient` nil → panic at `sessions.go:85`
  - `llmc resume get <id>` → same → panic at `resume.go:69`
  - `llmc config get <key>` → correctly skips auth (intended), no panic (doesn't use `apiClient`)
- **Fix:** Replace `cmd.Name()` with `cmd.CommandPath()` and match full paths, OR use a cobra annotation on commands that skip auth.
- **Blast radius:** `cmd/root.go` (the function), affects every subcommand named "get" or "set".

### C3. `health` command requires auth but API endpoint doesn't
- **File:** `cmd/root.go:86-90`, `cmd/health.go`
- **Root cause:** `"health"` is not in the `commandSkipsAuth` list. The API `GET /health` requires no auth, but the CLI demands an API key → `client.New()` fails if no key is configured.
- **Fix:** Add health to the auth-skip mechanism.
- **Blast radius:** `cmd/root.go`, `cmd/health.go`.

### C4. `rand.Int63n(0)` panics when `MaxJitter = 0`
- **File:** `internal/client/retry.go:38`
- **Root cause:** `rand.Int63n(n)` panics if `n <= 0`. If `MaxJitter` is set to 0 (e.g., in tests or by user config), this panics at runtime.
- **Fix:** Guard with `if rc.MaxJitter > 0 { ... }`.
- **Blast radius:** `internal/client/retry.go`. Also affects `internal/client/retry_test.go:14` which sets `MaxJitter: 0`.

### C5. `http.Client.Timeout` of 30s kills SSE streams after 30 seconds
- **File:** `internal/client/client.go:45`, `cmd/stream.go`
- **Root cause:** `http.Client.Timeout` covers the ENTIRE response body read. SSE streams run for minutes. The 30s timeout will terminate any stream after 30s with a deadline exceeded error.
- **Fix:** Use a separate `http.Client` with `Timeout: 0` for SSE/streaming, or set timeout to 0 on `GetRaw`/`PostRaw` requests.
- **Blast radius:** `internal/client/client.go` (new streaming client or method), `cmd/stream.go`, `cmd/agents.go` (streaming path in `run`).

### C6. Missing `go.sum` — project cannot build
- **File:** (missing)
- **Root cause:** `go mod tidy` was never run. No `go.sum` exists. `go.mod` is also missing indirect dependencies.
- **Fix:** Run `go mod tidy` on a machine with Go installed.
- **Blast radius:** `go.mod`, `go.sum` (new file).

---

## High Bugs (Incorrect behavior, resource leaks, broken features)

### H1. `defer resp.Body.Close()` inside retry loop leaks file descriptors
- **File:** `internal/client/client.go:145`
- **Root cause:** `defer` is function-scoped, not block-scoped. Each retry iteration defers a new close, but none execute until `do()` returns. With `MaxRetries=3`, up to 3 response bodies are held open simultaneously during retries.
- **Fix:** Extract loop body into helper function, or explicitly close before `continue`.
- **Blast radius:** `internal/client/client.go` only. Every HTTP call goes through `do()`.

### H2. Response body not closed before retry `continue`
- **File:** `internal/client/client.go:148-163`
- **Root cause:** On 429 Retry-After path (line 157 `continue`) and general retryable path (line 163 `continue`), the response body from `ParseErrorResponse` is consumed via `io.ReadAll` but never explicitly closed before the next iteration.
- **Fix:** Add `resp.Body.Close()` before every `continue` in the retry loop.
- **Blast radius:** Same as H1 — `internal/client/client.go`.

### H3. `ShouldRetry` returns `true` for `context.Canceled` / `context.DeadlineExceeded`
- **File:** `internal/client/retry.go:94-100`
- **Root cause:** The catch-all `return true` for non-`*APIError` errors matches context errors. The code retries deliberately-cancelled requests wastefully.
- **Fix:** Check `errors.Is(err, context.Canceled)` and `errors.Is(err, context.DeadlineExceeded)` before returning `true`.
- **Blast radius:** `internal/client/retry.go`. Needs `"errors"` import added.

### H4. `lastEventID` parameter is dead code — never sent as header
- **File:** `cmd/stream.go:36-39`, `internal/client/client.go:112-119`
- **Root cause:** `streamGeneration` accepts `lastEventID` but `apiClient.GetRaw(ctx, path)` has no mechanism to pass custom headers. `Last-Event-ID` is never sent.
- **Fix:** Add a `GetRawWithHeaders` method or accept `http.Header` in `GetRaw`.
- **Blast radius:** `internal/client/client.go` (new method or signature change), `cmd/stream.go`.

### H5. SSE reconnection logic is completely missing
- **File:** `cmd/stream.go:62`
- **Root cause:** When the SSE stream disconnects (EOF), `processSSEStream` returns `nil` (success). Mid-generation disconnects are silently treated as success — no reconnection attempt, no error.
- **Fix:** Detect EOF before `complete`/`error` event, reconnect with `Last-Event-ID` and exponential backoff.
- **Blast radius:** `cmd/stream.go`. Depends on H4 being fixed first.

### H6. No signal handling — Ctrl+C causes abrupt exit
- **File:** `main.go`, `cmd/root.go`
- **Root cause:** No `signal.Notify` anywhere. Ctrl+C triggers Go's default `os.Exit(2)`, skipping all deferred cleanup, context cancellation, and connection teardown.
- **Fix:** Add signal handler in `Execute()` that cancels a root context on SIGINT/SIGTERM. Pass this context to all commands.
- **Blast radius:** `cmd/root.go` (context setup), every `cmd/*.go` file (use `cmd.Context()` instead of `context.Background()`).

### H7. All commands use `context.Background()` — no timeout/cancellation propagation
- **File:** Every `cmd/*.go` file
- **Root cause:** `cfg.Timeout` is only used for `http.Client.Timeout`. No command creates a `context.WithTimeout`. The `ctx.Done()` check in `pollUntilDone` is dead code.
- **Fix:** After signal handling (H6), commands should use `cmd.Context()` which carries the cancellable context.
- **Blast radius:** Every command file.

### H8. `os.Exit(2)` for `awaiting_input` bypasses all deferred cleanup
- **File:** `cmd/agents.go:218`, `cmd/stream.go:122`
- **Root cause:** `os.Exit()` inside `RunE` bypasses cobra error handling, deferred functions, and cleanup.
- **Fix:** Define a sentinel error (e.g., `ErrAwaitingInput`). Return it from `RunE`. Handle it in `Execute()` to set exit code 2.
- **Blast radius:** `cmd/agents.go`, `cmd/stream.go`, `cmd/root.go` (Execute function).

### H9. `os.Exit(1)` in health.go bypasses cobra error handling
- **File:** `cmd/health.go:34-36`
- **Root cause:** Same anti-pattern as H8 but for error exit.
- **Fix:** Replace with `return fmt.Errorf("health check failed: %w", err)`.
- **Blast radius:** `cmd/health.go` only.

### H10. User-Agent is always `llmconveyors-go/dev` in release builds
- **File:** `cmd/root.go`, `internal/client/client.go:48`
- **Root cause:** `root.go` never calls `client.WithUserAgent()`. The version string from `versionStr` is available in the `cmd` package but not passed to the client.
- **Fix:** Pass `client.WithUserAgent("llmconveyors-go/" + versionStr)` in `PersistentPreRunE`.
- **Blast radius:** `cmd/root.go` only.

### H11. Two viper instances — config commands ignore `--config` flag
- **File:** `cmd/config_cmd.go:92,126`, `cmd/root.go:110-111`
- **Root cause:** `config.Load()` uses `viper.New()` (local instance). `config_cmd.go` calls `viper.ConfigFileUsed()` on the **global** viper (never initialized). Always returns empty string. Falls back to `DefaultConfigPath()`, ignoring `--config /custom/path`.
- **Fix:** Read the `--config` flag directly: `configPath, _ := cmd.Flags().GetString("config")`.
- **Blast radius:** `cmd/config_cmd.go` (both set and get commands).

### H12. Dead viper code in root.go init()
- **File:** `cmd/root.go:110-111`
- **Root cause:** `viper.SetEnvPrefix("LLMC")` and `viper.AutomaticEnv()` configure the global viper, but `config.Load()` uses a separate instance. These lines do nothing.
- **Fix:** Remove lines 110-111.
- **Blast radius:** `cmd/root.go` only.

---

## Medium Bugs (Wrong behavior, UX issues, missing validation)

### M1. `doMultipart` has no retry support
- **File:** `internal/client/client.go:217-252`
- **Root cause:** Single-shot execution, no retry loop. Multipart uploads fail permanently on transient 429/502/503/504. Even if retry were added, `io.Reader` is not seekable.
- **Fix:** Accept `io.ReadSeeker` or buffer the body for retry. Add retry loop mirroring `do()`.
- **Blast radius:** `internal/client/client.go`, `cmd/upload.go` (caller signatures may change).

### M2. `run` command doesn't validate required fields client-side
- **File:** `cmd/agents.go` init block
- **Root cause:** `--company`, `--website`, `--title`, `--jd` are not marked required. The CLI sends an empty body and gets server-side `VALIDATION_ERROR`.
- **Fix:** Add agent-type-specific validation after parsing `args[0]`.
- **Blast radius:** `cmd/agents.go` only.

### M3. `formatter.WriteText()` return errors discarded in multiple commands
- **File:** `cmd/agents.go:85`, `cmd/sessions.go:63,68`, `cmd/resume.go:49`, `cmd/api_keys.go:45,89,132`, `cmd/settings.go:90`, `cmd/stream.go:101`
- **Root cause:** `WriteText` and `WriteJSON` return errors that are silently dropped (no `return` before the call).
- **Fix:** Add `if err := formatter.WriteText(...); err != nil { return err }` pattern.
- **Blast radius:** Every file listed above.

### M4. Lazy JSON fallback in text mode — 13+ commands dump JSON when `--output text`
- **Files:** `cmd/sessions.go` (get, hydrate), `cmd/resume.go` (get, create, update), `cmd/ats.go` (score), `cmd/upload.go` (all), `cmd/settings.go` (profile, preferences, usage-logs), `cmd/agents.go` (manifest)
- **Root cause:** Many commands use `formatter.WriteJSON()` as the `default:` case in their format switch, producing JSON even in text mode.
- **Fix:** Add proper `WriteText` formatting for each command's text mode.
- **Blast radius:** All files listed. Large but non-blocking improvement.

### M5. Missing `FormatTable` handling in 15+ commands
- **Files:** Same as M4 plus additional commands
- **Root cause:** `default:` branch catches both `FormatText` and `FormatTable`. `--output table` silently produces text or JSON instead of tables.
- **Fix:** Add explicit `FormatTable` cases where applicable.
- **Blast radius:** Same as M4.

### M6. `--output` flag shadowed on download/render commands
- **File:** `cmd/sessions.go:183`, `cmd/resume.go:243`
- **Root cause:** Local `--output` flag (file path) shadows the persistent `--output` flag (format). `llmc sessions download --output json` creates a file named "json" instead of switching format.
- **Fix:** Rename local flag to `--out-file` or `--dest`.
- **Blast radius:** `cmd/sessions.go`, `cmd/resume.go`.

### M7. `StringSlice` for `--set` in preferences splits on commas
- **File:** `cmd/settings.go:115`
- **Root cause:** cobra's `StringSlice` splits on commas. `--set "tags=a,b,c"` becomes `["tags=a", "b", "c"]`. The second and third entries fail the `key=value` parse.
- **Fix:** Use `StringArray` instead of `StringSlice`.
- **Blast radius:** `cmd/settings.go` only.

### M8. `strings.Split(scopes, ",")` doesn't trim whitespace or filter empties
- **File:** `cmd/api_keys.go:74`
- **Root cause:** `"jobs:write, jobs:read,"` → `["jobs:write", " jobs:read", ""]`. Leading spaces and trailing empty strings sent to API.
- **Fix:** Trim each scope and filter empty strings.
- **Blast radius:** `cmd/api_keys.go` only.

### M9. `setIfFlag` always sends strings — no boolean/numeric API field support
- **File:** `cmd/agents.go:116-121`
- **Root cause:** All values go through `GetString`. Boolean fields like `autoSelectContacts` would be sent as `"true"` (string) not `true` (boolean).
- **Fix:** Add typed variants (`setBoolIfFlag`, `setIntIfFlag`).
- **Blast radius:** `cmd/agents.go` only.

### M10. `version` command fails if config file is malformed
- **File:** `cmd/root.go` PersistentPreRunE, `cmd/version.go`
- **Root cause:** `PersistentPreRunE` runs for ALL subcommands including `version`. If `config.Load()` fails (bad YAML), the version command errors out even though it needs no config.
- **Fix:** Have `PersistentPreRunE` return nil early for auth-skipped commands before loading config, or give `versionCmd` its own override.
- **Blast radius:** `cmd/root.go`.

### M11. SSE stream errors shown as raw JSON instead of structured error
- **File:** `cmd/stream.go:45-48`
- **Root cause:** HTTP errors from the stream endpoint are read as raw text instead of using `ParseErrorResponse`.
- **Fix:** Use `client.ParseErrorResponse` for 4xx/5xx responses.
- **Blast radius:** `cmd/stream.go` only.

### M12. `Write()` method on Formatter is broken for non-JSON formats
- **File:** `internal/output/format.go:88-96`
- **Root cause:** Both `FormatJSON` and `default` cases call `WriteJSON`. The method is a trap — any future caller gets JSON regardless of format.
- **Fix:** Either make `Write()` dispatch correctly or remove it.
- **Blast radius:** `internal/output/format.go` only. Currently no callers.

### M13. `APIResponse[T any]` generic type is declared but never used
- **File:** `internal/client/client.go:14-19`
- **Root cause:** Dead code. The `decodeResponse` method uses anonymous structs with `json.RawMessage`.
- **Fix:** Remove the unused type or integrate it into `decodeResponse`.
- **Blast radius:** `internal/client/client.go` only.

---

## Low / Test-Only Issues

### L1. Test `BaseDelay = 1` is 1 nanosecond, not 1 millisecond
- **File:** `internal/client/client_test.go:207`
- **Fix:** Change to `1 * time.Millisecond`.

### L2. No integration test for Retry-After header in full client
- **File:** `internal/client/client_test.go` (missing test)
- **Fix:** Add test with httptest returning 429 + Retry-After header.

### L3. 3 of 4 `decodeResponse` error paths untested
- **File:** `internal/client/client_test.go` (missing tests)
- **Fix:** Add tests for ReadAll failure, success=false, and data unmarshal failure.

### L4. No test for body-close behavior in retry loop
- **File:** `internal/client/client_test.go` (missing test)
- **Fix:** Add test with tracking `ReadCloser` that asserts `Close()` calls.

### L5. No test for `"data:value"` (no space after prefix) in SSE parser
- **File:** `internal/sse/parser_test.go` (missing test)
- **Fix:** Add edge case test.

### L6. SSE parser fallback wraps raw string as `json.RawMessage` without validation
- **File:** `internal/sse/parser.go:122-127`
- **Fix:** Validate JSON before wrapping, or document the assumption.

### L7. `PostRaw` method is defined but never used (dead code)
- **File:** `internal/client/client.go:107-109`
- **Fix:** Remove or add a caller.

### L8. `Formatter.PrintError` and `PrintSuccess` are defined but never called
- **File:** `internal/output/format.go:108-117`
- **Fix:** Remove dead code or integrate into commands.

---

## Files That Need Changes (Sorted by Impact)

| File | Issues | Priority |
|------|--------|----------|
| `internal/client/client.go` | C5, H1, H2, M1, M13, L7 | **Critical** |
| `cmd/root.go` | C2, C3, H6, H7, H8, H10, H12, M10 | **Critical** |
| `internal/client/retry.go` | C4, H3 | **Critical** |
| `cmd/stream.go` | C5, H4, H5, H8, M3, M11 | **Critical** |
| `cmd/agents.go` | H8, M2, M3, M9 | **High** |
| `cmd/sessions.go` | C1, M3, M6 | **High** |
| `cmd/health.go` | C3, H9 | **High** |
| `cmd/config_cmd.go` | H11 | **High** |
| `go.mod` / `go.sum` | C6 | **High** (requires Go install) |
| `internal/output/format.go` | M12, L8 | **Medium** |
| `internal/sse/parser.go` | L6 | **Medium** |
| `cmd/resume.go` | M3, M4, M6 | **Medium** |
| `cmd/settings.go` | M3, M4, M7 | **Medium** |
| `cmd/api_keys.go` | M3, M8 | **Medium** |
| `cmd/upload.go` | M4 | **Medium** |
| `cmd/ats.go` | M4 | **Medium** |
| `cmd/version.go` | M10 (indirect) | **Low** |
| `main.go` | H6 (indirect) | **Low** |
| `internal/client/client_test.go` | L1, L2, L3, L4 | **Test** |
| `internal/client/retry_test.go` | C4 (affected by fix) | **Test** |
| `internal/sse/parser_test.go` | L5 | **Test** |

---

## Dependency Graph of Fixes

```
C6 (go.sum) ← independent, needs Go installed
C1 (max builtin) ← standalone fix
C4 (rand panic) ← standalone fix
C2+C3 (commandSkipsAuth) ← must fix before any testing
M10 (version fails on bad config) ← related to C2/C3 fix
H12 (dead viper code) ← cleanup after C2/C3
H11 (config cmd ignores --config) ← standalone fix
H9 (health os.Exit) ← standalone fix
H1+H2 (defer in loop) ← standalone fix in client.go
H3 (ShouldRetry context) ← standalone fix in retry.go
C5 (timeout kills SSE) ← must fix before streaming works
  └─ H4 (lastEventID dead) ← depends on client.go changes
     └─ H5 (reconnection missing) ← depends on H4
H6 (signal handling) ← must be done before H7
  └─ H7 (context.Background everywhere) ← depends on H6
H8 (os.Exit(2) sentinel) ← depends on root.go changes from C2
H10 (User-Agent version) ← standalone, needs versionStr access
M1-M12 ← can be done after all H-level fixes
L1-L8 ← test and cleanup, do last
```

---

## Implementation Order (Recommended)

### Wave 1: Compilation blockers & crash fixes
1. C1 — Delete `max` function in sessions.go
2. C2+C3 — Rewrite `commandSkipsAuth` using `cmd.CommandPath()`, add health
3. C4 — Guard `rand.Int63n` for zero jitter
4. H1+H2 — Fix defer-in-loop in client.go
5. H3 — Fix ShouldRetry for context errors
6. H9 — Remove os.Exit from health.go
7. H11 — Fix config commands to read --config flag
8. H12 — Remove dead viper code from root.go

### Wave 2: Streaming & context infrastructure
9. C5 — Separate streaming client with no timeout
10. H4 — Add header support to GetRaw for Last-Event-ID
11. H5 — Implement SSE reconnection with backoff
12. H6 — Add signal handling with cancellable root context
13. H7 — Switch all commands to use cmd.Context()
14. H8 — Replace os.Exit(2) with sentinel error
15. H10 — Wire version string into User-Agent
16. M10 — Skip config load for auth-skipped commands

### Wave 3: Validation & UX
17. M1 — Add retry to doMultipart (with ReadSeeker)
18. M2 — Validate required run command fields
19. M3 — Fix all dropped WriteText/WriteJSON errors
20. M6 — Rename --output to --out-file on download/render
21. M7 — StringSlice → StringArray for preferences --set
22. M8 — Trim and filter scopes in api-keys create
23. M9 — Add typed setIfFlag variants
24. M11 — Use ParseErrorResponse in stream.go error path

### Wave 4: Output polish & dead code
25. M4+M5 — Add proper text/table formatting to all commands
26. M12 — Fix or remove Formatter.Write()
27. M13 — Remove unused APIResponse generic type
28. L7+L8 — Remove dead code (PostRaw, PrintError, PrintSuccess)

### Wave 5: Tests
29. L1 — Fix BaseDelay in test
30. L2 — Add Retry-After integration test
31. L3 — Add decodeResponse error path tests
32. L4 — Add body-close tracking test
33. L5 — Add SSE no-space data test
34. C6 — Run go mod tidy (needs Go installed)

---

## Appendix: Gaps vs Official Documentation (llmconveyors.com/docs)

> Cross-referenced on 2026-03-23 against the live API docs at llmconveyors.com/docs.

### API1. Missing endpoints — not implemented in CLI at all

| Endpoint | Method | Purpose | Priority |
|----------|--------|---------|----------|
| `/sessions/init` | GET | Session initialization data | Medium |
| `/sessions/:id/log` | POST | Append chat log entry | Low |
| `/sessions/:id/generation-logs/:genId/init` | POST | Initialize generation log | Low |
| `/resume/parse` | POST | Parse resume file (distinct from upload) | Medium |
| `/resume/validate` | POST | Validate resume data | Medium |
| `/resume/preview` | POST | Preview resume as HTML | Medium |
| `/content/save` | POST | Save source document | Low |
| `/content/generations/:id` | DELETE | Delete generation | Low |
| `/documents/download` | GET | Download document (separate from session download) | Medium |
| `/settings/api-key` | GET/POST/DELETE | BYO API key management | Medium |
| `/settings/platform-api-keys/:hash/usage` | GET | Per-key usage stats | Low |
| `/settings/webhook-secret` | GET | Get webhook secret | Low |
| `/settings/webhook-secret/rotate` | POST | Rotate webhook secret | Low |
| `/shares` | POST | Create share link | Low |
| `/shares/stats` | GET | List user shares | Low |
| `/shares/:slug/public` | GET | Get public share data | Low |
| `/shares/:slug/visit` | POST | Record share visit | Low |
| `/shares/:slug/stats` | GET | Share visit statistics | Low |
| `/referral/stats` | GET | Referral statistics | Low |
| `/referral/code` | GET | Get referral code | Low |
| `/referral/vanity-code` | POST | Set vanity referral code | Low |
| `/auth/export` | GET | GDPR data export | Low |
| `/auth/account` | DELETE | Delete account | Low |
| `/privacy/consents` | GET | Get consent records | Low |
| `/privacy/consents/:purpose` | POST/DELETE | Grant/withdraw consent | Low |
| `/log` | POST | Forward structured client log | Low |
| `/stream/health` | GET | SSE stream health check | Low |

**Impact:** The CLAUDE.md spec covered the core 80% of endpoints. The missing ones are mostly ancillary features (shares, referrals, GDPR compliance, BYO keys). The medium-priority ones (`resume/parse`, `resume/validate`, `resume/preview`, `documents/download`, BYO API key management) should be added for completeness.

### API2. Job Hunter `generate` — `companyWebsite` is NOT in the minimal required fields

The official docs show the minimal required body for job-hunter as:
```json
{
  "companyName": "string",
  "jobTitle": "string",
  "jobDescription": "string"
}
```

The CLAUDE.md spec lists `companyWebsite` as required. The official docs do NOT include it in the minimal required set. This means our CLI's `--website` flag should NOT be mandatory for job-hunter — it should be optional.

**Impact:** `cmd/agents.go` — if we add client-side validation (M2), we must NOT require `--website` for job-hunter.

### API3. `AI_PROVIDER_ERROR` is NOT listed as retryable in official docs

The official error handling docs list these as retryable:
- `RATE_LIMITED`, `GENERATION_TIMEOUT`, `SERVER_RESTARTING`, `STREAM_ERROR`, `CONCURRENT_GENERATION_LIMIT`

Our CLAUDE.md spec adds `AI_PROVIDER_ERROR` to the retryable set. The official docs list it at HTTP 502 but do NOT include it in the retryable list.

**Impact:** `internal/client/errors.go` — `AI_PROVIDER_ERROR` should be removed from `retryableCodes`. HTTP 502 status IS still retried via `IsRetryableStatus`, so 502 responses are still retried regardless of the error code.

### API4. Rate limit jitter — official docs say 0-1000ms, CLAUDE.md says 0-500ms

Official: "Add random jitter (0-1000ms) to prevent thundering herd"
CLAUDE.md: "Jitter: 0-500ms random"

**Impact:** `internal/client/retry.go` — `DefaultRetryConfig.MaxJitter` should be `1000 * time.Millisecond`, not `500 * time.Millisecond`.

### API5. Retry-After — official docs say default to 60s if missing

Official: "Extract Retry-After value from response headers (default to 60 seconds)"
Our implementation: Returns 0 if header missing (no default).

**Impact:** `internal/client/retry.go` — `ParseRetryAfter` should return a 60s default when the header is absent on 429 responses.

### API6. Settings profile response has different shape than assumed

Official response fields:
- `credits` (number) — current credit balance
- `tier` (string) — "free" or "byo"
- `byoKeyEnabled` (boolean) — whether BYO API key is active

Our `settings profile` command uses `map[string]interface{}` and dumps JSON, so it technically works, but the text-mode output should show these specific fields.

**Impact:** `cmd/settings.go` — add proper text formatting for profile.

### API7. Usage summary response has different field names

Official: `totalCreditsUsed`, `totalGenerations`, `averageCreditsPerGeneration`
Our command checks for `credits` and `tier` — wrong field names.

**Impact:** `cmd/settings.go` — fix the text-mode field references in `settingsUsageCmd`.

### API8. Webhook support missing from generate requests

Official docs show `webhookUrl` is an optional field in generate request bodies. Our CLI doesn't expose it.

**Impact:** `cmd/agents.go` — add `--webhook-url` flag to `runCmd`.

### API9. Complete event has more fields than our struct models

Official complete event fields include:
- `generationId` — we don't capture this
- `warnings` (string[]) — non-fatal issues, not modeled
- `awaitingInput` (boolean) — critical for phased execution detection
- `interactionType`, `completedPhase`, `interactionData` — phased execution fields
- `persistenceWarning` (boolean) — artifact storage failure flag

**Impact:** `internal/sse/parser.go` — `CompleteData` struct needs additional fields. `cmd/stream.go` — `processSSEStream` needs to handle `awaitingInput` from the `complete` event (not just from a separate `interaction_required` event type).

### API10. Phased execution — Phase B requires a NEW SSE stream

Official docs: "Client establishes **new SSE stream** for Phase B". After submitting interaction via `POST /interact`, the response includes a new `streamUrl` and the client must connect to a new SSE endpoint.

Our `interact` command just prints the result and exits. It should optionally connect to the new stream.

**Impact:** `cmd/agents.go` (`interactCmd`) — add `--stream` flag to auto-connect to Phase B stream after interaction.

### API11. API key rotate endpoint returns `newKey`, not described as having grace period

Official: "Rotates API key with grace period for old key"
Our implementation assumes immediate rotation.

**Impact:** No code change needed, but worth noting for documentation.

### API12. Max 10 API keys per user

Official: "Maximum 10 API keys per user account"
Our CLI doesn't enforce or warn about this.

**Impact:** Low — server-side enforced.

---

## Updated File Change Map (incorporating official docs findings)

| File | Additional Changes from Docs Review |
|------|-------------------------------------|
| `internal/client/errors.go` | API3: Remove `AI_PROVIDER_ERROR` from retryableCodes |
| `internal/client/retry.go` | API4: MaxJitter 500ms → 1000ms; API5: Default Retry-After 60s on 429 |
| `internal/sse/parser.go` | API9: Expand CompleteData struct with missing fields |
| `cmd/stream.go` | API9: Handle `awaitingInput` in complete event |
| `cmd/agents.go` | API2: Don't require --website for job-hunter; API8: Add --webhook-url; API10: Stream after interact |
| `cmd/settings.go` | API6: Fix profile text output; API7: Fix usage summary field names |
