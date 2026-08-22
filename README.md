# litellm-mux

A Go-based CLI and proxy tool for managing models on [LiteLLM](https://docs.litellm.ai/) proxy gateways.

`litellm-mux` lets you list, delete, and copy/multiplex model deployments on a LiteLLM proxy — including multiplexing a single source model across multiple credentials of the same provider.

## Features

- **List models** (`models ls`) with selectable columns: provider, provider model string, LiteLLM model ID, tags, max tokens, input/output costs, mode, API base, and credential
- **Filtering** with reusable regex filters, optionally scoped to a column (e.g. `provider:deepinfra`)
- **Delete models** (`models rm`) by exact name/ID or by filter, with dry-run and confirmation prompt
- **Copy / multiplex models** (`models copy`) to another provider or credential, optionally fanning out across all other credentials of the target provider
- **Dry-run support** for destructive operations
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

# All columns
litellm-mux models ls -a

# Specific columns
litellm-mux models ls --id --tags --credential

# Model names only, on one line (handy for scripting)
litellm-mux models ls -1

# Filter by regex, optionally column-scoped
litellm-mux models ls "gemini" "provider:deepinfra"
```

Available flags for `models ls`: `-l/--minimal`, `--id`, `--model-string`, `--tags`, `--tokens`, `--costs`, `--mode`, `--api-base`, `--credential`, `-a/--all`, `-1/--oneline`.

### Delete models

```bash
# Preview what would be deleted
litellm-mux models rm -n "my-model" -f "provider:openrouter"

# Delete by name or ID (asks for confirmation unless -y is given)
litellm-mux models rm "my-model"

# Delete everything matching a filter
litellm-mux models rm -f "provider:deepinfra" -y
```

Filters use the same syntax as `models ls`: either a bare regex matched against all columns, or `column:regex` where `column` is one of `MODEL NAME`, `PROVIDER`, `MAX TOKENS`, `INPUT / 1M ($)`, `OUTPUT / 1M ($)`, `MODE`, `API BASE`, `CREDENTIAL`. Multiple filters are combined with logical AND.

### Copy / multiplex models

```bash
# Preview the copy plan
litellm-mux models copy -n "source-model" "new-name"

# Copy to a new credential of the same provider
litellm-mux models copy "source-model" "new-name" --credential "other-cred"

# Fan out: create one copy per other credential of the target provider
litellm-mux models copy "source-model" --all-other-credentials

# Copy with an overridden provider model string
litellm-mux models copy "source-model" --provider openrouter --model-string "google/gemini-2.5-flash"
```

When copying to multiple credentials, the new model names get a credential-derived suffix automatically.

## Project structure

```text
cmd/litellm-mux/main.go   # Entry point
cmd/root.go               # Root command, persistent flags (--url, --master-key)
cmd/models.go             # `models` command group
cmd/models_ls.go          # `models ls`
cmd/models_rm.go          # `models rm`
cmd/models_copy.go        # `models copy`
internal/config/          # Config resolution (flags > env > .env)
internal/client/          # LiteLLM API client (Bearer auth)
internal/models/          # API response types
internal/filter/          # Reusable regex filter engine
```

## License

MIT
