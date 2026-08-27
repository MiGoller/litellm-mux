# litellm-mux

A Go-based CLI and proxy tool for managing models on [LiteLLM](https://docs.litellm.ai/) proxy gateways.

`litellm-mux` lets you list, delete, copy/multiplex, enable/disable, manage tags, guardrails, and set token pricing (including cache read/write costs) on a LiteLLM proxy.

## Features

- **List models** (`models ls`) with selectable columns: provider, provider model string, LiteLLM model ID, tags, guardrails, status, max tokens, input/output costs, cache read/write costs, mode, API base, and credential
- **Filtering** with reusable regex filters, optionally scoped to any column (e.g. `provider:deepinfra`, `status:active`)
- **Delete models** (`models rm`) by exact name/ID or by filter, with dry-run and confirmation prompt
- **Copy / multiplex models** (`models copy`) to another provider or credential, optionally fanning out across all other credentials of the target provider
- **Enable / Disable models** (`models enable` / `models disable`) to block or unblock model access without deleting historical data
- **Manage tags** (`models tags`) and **Guardrails** (`models guardrails`) with before/after plans
- **Manage model pricing / costs** (`models costs set`) to set per-token pricing (input, output, cache read, cache write) per 1M tokens
- **Dry-run support** for all destructive or mutating operations (`-n` / `--dry-run`)
- Configuration via CLI flags, environment variables, or `.env` file

## Installation

### Prerequisites

- Go 1.22 or later

### Build from source

```bash
git clone https://github.com/MiGoller/litellm-mux.git
cd litellm-mux
go build -o litellm-mux ./cmd/litellm-mux/main.go
```

## Configuration

`litellm-mux` needs the URL of your LiteLLM gateway and its master key. They are resolved in this order (highest priority first):

1. CLI flags: `--url`, `--master-key`
2. Environment variables: `LITELLM_URL`, `MASTER_KEY` (or `LITELLM_MASTER_KEY`)
3. `.env` file in the working directory

> ⚠️ Never commit `.env` files or credentials. `.gitignore` already excludes them.

## Usage

```text
litellm-mux [command] --url <URL> --master-key <KEY>
```

### List models

```bash
# Minimal view (model name + provider)
litellm-mux models ls -l

# All columns (including guardrails, status, and cache costs)
litellm-mux models ls -a

# Specific columns
litellm-mux models ls --id --tags --status --costs

# Model names only, on one line (handy for scripting)
litellm-mux models ls -1

# Filter by regex, optionally column-scoped
litellm-mux models ls "gemini" "provider:deepinfra" "status:active"
```

Available flags for `models ls`: `-l/--minimal`, `--id`, `--model-string`, `--tags`, `--guardrails`, `--status`, `--tokens`, `--costs`, `--mode`, `--api-base`, `--credential`, `-a/--all`, `-1/--oneline`.

### Delete models

```bash
# Preview what would be deleted
litellm-mux models rm -n "my-model" -f "provider:openrouter"

# Delete by name or ID (asks for confirmation unless -y is given)
litellm-mux models rm "my-model"

# Delete everything matching a filter
litellm-mux models rm -f "provider:deepinfra" -y
```

Filters use the same syntax everywhere: either a bare regex matched against all columns, or `column:regex`. Multiple filters are combined with logical AND.

### Enable / Disable models

```bash
# Disable models matching a filter
litellm-mux models -f "provider:openrouter" disable --dry-run
litellm-mux models -f "provider:openrouter" disable -y

# Enable models
litellm-mux models -f "provider:openrouter" enable
```

### Manage tags and guardrails

```bash
# Add tags
litellm-mux models -f "provider:deepinfra" tags add -t "paid" -t "team:dev"

# Assign guardrails
litellm-mux models -f "provider:gemini" guardrails add --guardrail "my-guardrail"
```

### Set model pricing / costs

```bash
# Set input, output, cache read and cache write costs per 1M tokens
litellm-mux models -f "provider:gemini" costs set --input 0.15 --output 0.60 --cache-read 0.03 --cache-write 0.07
```

### Copy / multiplex models

```bash
# Preview the copy plan
litellm-mux models copy -n "source-model" "new-name"

# Copy to a new credential of the same provider
litellm-mux models copy "source-model" "new-name" --credential "other-cred"

# Fan out: create one copy per other credential of the target provider
litellm-mux models copy "source-model" --all-other-credentials
```

## Project structure

```text
cmd/litellm-mux/main.go   # Entry point
cmd/root.go               # Root command, persistent flags (--url, --master-key)
cmd/models.go             # `models` command group, shared model selection (-f filters)
cmd/models_ls.go          # `models ls`
cmd/models_rm.go          # `models rm`
cmd/models_copy.go        # `models copy`
cmd/models_tag.go         # `models tags ls/add/rm`
cmd/models_guardrails.go  # `models guardrails ls/add/rm`
cmd/models_enable.go      # `models enable / disable`
cmd/models_costs.go       # `models costs set`
internal/config/          # Config resolution (flags > env > .env)
internal/client/          # LiteLLM API client (Bearer auth)
internal/models/          # API response types
internal/filter/          # Reusable regex filter engine
```

## License

MIT
