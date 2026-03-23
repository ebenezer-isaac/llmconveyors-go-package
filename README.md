# llmc — LLM Conveyors CLI

Official Go CLI for the [LLM Conveyors](https://llmconveyors.com) AI Agent Platform. Run AI agents, manage sessions, score resumes, and more — all from your terminal.

## Installation

### Homebrew (macOS + Linux)

```bash
brew install ebenezer-isaac/tap/llmc
```

### Debian / Ubuntu

```bash
curl -LO https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest/download/llmc_0.1.0_linux_amd64.deb
sudo dpkg -i llmc_0.1.0_linux_amd64.deb
```

### RPM (Fedora / RHEL)

```bash
sudo rpm -i https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest/download/llmc_0.1.0_linux_amd64.rpm
```

### Go install

```bash
go install github.com/llmconveyors/cli@latest
```

### Binary download

```bash
# Linux (amd64)
curl -LO https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest/download/llmconveyors-go-package_0.1.0_linux_amd64.tar.gz
tar xzf llmconveyors-go-package_0.1.0_linux_amd64.tar.gz
sudo mv llmc /usr/local/bin/

# macOS (Apple Silicon)
curl -LO https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest/download/llmconveyors-go-package_0.1.0_darwin_arm64.tar.gz
tar xzf llmconveyors-go-package_0.1.0_darwin_arm64.tar.gz
sudo mv llmc /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest/download/llmconveyors-go-package_0.1.0_darwin_amd64.tar.gz
tar xzf llmconveyors-go-package_0.1.0_darwin_amd64.tar.gz
sudo mv llmc /usr/local/bin/

# Windows — download .zip from GitHub Releases:
# https://github.com/ebenezer-isaac/llmconveyors-go-package/releases/latest
```

## Quick Start

```bash
# Configure your API key
llmc config init

# Or set it directly
export LLMC_API_KEY=llmc_your_key_here

# Check API health
llmc health

# Run the Job Hunter agent
llmc run job-hunter \
  --company "Acme Corp" \
  --title "Senior Engineer" \
  --jd "We are looking for..."

# Run B2B Sales agent
llmc run b2b-sales \
  --company "Acme Corp" \
  --website "https://acme.com"

# Check generation status
llmc status <jobId> --agent job-hunter --watch

# List your sessions
llmc sessions list
```

## Commands

```
llmc
  run <agent-type>              Start agent generation (job-hunter, b2b-sales)
  status <jobId>                Check generation status (--watch for polling)
  interact                      Resume phased generation with user input
  manifest <agent-type>         Show agent capabilities
  stream <generationId>         Raw SSE stream to stdout
  stream-health                 Check SSE server health

  sessions
    init                        Get session initialization data
    list                        List sessions (--page, --limit, --agent)
    get <id>                    Get session details
    delete <id>                 Delete session
    hydrate <id>                Full session with artifacts
    download <id>               Download artifact (--key, --dest)
    log <id>                    Append chat log entry (--role, --content)
    gen-log-init <sid> <gid>    Initialize generation log

  resume
    list                        List master resumes
    get <id>                    Get master resume
    create --file <json>        Create master resume
    update <id> --file <json>   Update master resume
    delete <id>                 Delete master resume
    parse <file>                Parse resume file to structured data
    validate --file <json>      Validate resume against schema
    render --file <json>        Render resume to PDF (--theme, --dest)
    preview --file <json>       Preview resume as HTML (--theme)
    themes                      List available themes

  ats
    score                       Score resume vs job description

  upload
    resume <file>               Upload and parse resume
    job <file>                  Upload and parse job description
    job-text                    Parse JD from text (--file or --text)

  content
    save --file <json>          Save source document
    delete-generation <id>      Delete a generation

  documents
    download                    Download generated document (--key, --dest)

  settings
    profile                     Show user profile (credits, tier)
    preferences                 Get/set preferences (--set key=value)
    usage                       Credit usage summary
    usage-logs                  Detailed usage log (--limit, --offset)
    byo-key get|set|delete      Manage BYO API key
    webhook-secret get|rotate   Manage webhook signing secret

  api-keys
    list                        List API keys
    create                      Create new key (--name, --scopes)
    revoke <hash>               Revoke API key
    rotate <hash>               Rotate API key
    usage <hash>                Key usage statistics

  shares
    create                      Create public share link (--data)
    list                        List your share links
    get <slug>                  Get public share data
    visit <slug>                Record share visit
    stats <slug>                Share visit statistics

  referral
    stats                       Referral statistics
    code                        Get your referral code
    set-vanity <code>           Set custom vanity code

  auth
    export                      Export all user data (GDPR) (--dest)
    delete-account              Delete account permanently (--confirm)

  privacy
    list                        List consent records
    grant <purpose>             Grant consent
    withdraw <purpose>          Withdraw consent

  log                           Forward structured log (--data)

  health                        API health check (no auth required)
  config init|set|get           Manage CLI configuration
  version                       Print version info
```

## Global Flags

```
--api-key string       API key (overrides LLMC_API_KEY env var and config)
--base-url string      API base URL (default: https://api.llmconveyors.com/api/v1)
--output string        Output format: json, table, text (default: text)
--debug                Enable debug logging (HTTP requests to stderr)
--no-color             Disable colored output
--timeout duration     Request timeout (default: 30s)
--config string        Config file path (default: ~/.llmconveyors/config.yaml)
```

## Configuration

```bash
# Interactive setup
llmc config init

# Manual config
llmc config set api_key llmc_your_key
llmc config get api_key
```

Config file: `~/.llmconveyors/config.yaml`

```yaml
api_key: llmc_...
base_url: https://api.llmconveyors.com/api/v1
output: text
timeout: 30s
max_retries: 3
debug: false
no_color: false
```

Priority: flag > env var (`LLMC_API_KEY`, `LLMC_BASE_URL`) > config file > default.

## Output Formats

```bash
# Human-readable (default)
llmc sessions list

# Machine-readable JSON (pipe to jq)
llmc sessions list --output json | jq '.sessions[0]'

# Tabular
llmc sessions list --output table
```

## Phased Execution

Some agents (e.g., Job Hunter in cold outreach mode) use phased execution with interaction gates:

```bash
# Phase A: start generation — exits with code 2 when awaiting input
llmc run job-hunter --company "Acme" --title "SWE" --jd "..."

# Review the interaction data, then submit selections
llmc interact --agent job-hunter \
  --generation-id <id> --session-id <id> \
  --type contact_selection \
  --data '{"selectedContactIds": ["id1"]}' \
  --stream   # auto-streams Phase B
```

## Development

```bash
# Build
go build -o llmc .

# Build with version info
go build -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o llmc .

# Test
go test ./... -count=1

# Vet
go vet ./...
```

## License

MIT
