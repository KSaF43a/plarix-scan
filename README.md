# Plarix Scan

**Free CI cost recorder for LLM API usage.**
Records tokens and costs from *real* provider responses (no estimation).

## Use Cases
- **CI/CD**: Block PRs that exceed cost allowance.
- **Local Dev**: Measure cost of running your test suite.
- **Production**: Monitor LLM sidecar traffic via Docker.

---

## Quick Start (GitHub Action)

Add this to your `.github/workflows/cost.yml`:

```yaml
name: LLM Cost Check
on: [pull_request]

permissions:
  pull-requests: write # Required for PR comments

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: plarix-ai/scan@v1
        with:
          command: "pytest -v" # Your test command
          fail_on_cost_usd: 1.0 # Optional: fail if > $1.00
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

## How It Works in 3 Steps
1. **Starts a Proxy** on `localhost`.
2. **Injects Env Vars** (e.g. `OPENAI_BASE_URL`) so your SDK routes traffic to the proxy.
3. **Records Usage** from the actual API response body before passing it back to your app.

### Supported Providers
The proxy sets these environment variables:

| Provider | Env Var Injected | Notes |
|----------|------------------|-------|
| **OpenAI** | `OPENAI_BASE_URL` | Chat Completions + Responses |
| **Anthropic** | `ANTHROPIC_BASE_URL` | Messages API |
| **OpenRouter**| `OPENROUTER_BASE_URL` | OpenAI-compatible endpoint |

> **Requirement**: Your LLM SDK must respect these standard environment variables or allow configuring the `base_url`.

---

## Output Files

Artifacts are written to the working directory:

### `plarix-ledger.jsonl`
One entry per API call.
```json
{"ts":"2026-01-04T12:00:00Z","provider":"openai","model":"gpt-4o","input_tokens":50,"output_tokens":120,"cost_usd":0.001325,"cost_known":true}
```

### `plarix-summary.json`
Aggregated totals.
```json
{
  "total_calls": 5,
  "total_known_cost_usd": 0.045,
  "model_breakdown": {
    "gpt-4o": {"calls": 5, "known_cost_usd": 0.045}
  }
}
```

---

## Usage Guide

### 1. Local Development
Run the binary to wrap your test command:

```bash
# Build (or download)
go build -o plarix-scan ./cmd/plarix-scan

# Run
./plarix-scan run --command "npm test"
```

### 2. Production (Docker Sidecar)
Run Plarix as a long-lived proxy sidecar.

**docker-compose.yaml:**
```yaml
services:
  plarix:
    image: plarix-scan:latest # (Build locally provided Dockerfile)
    ports:
      - "8080:8080"
    volumes:
      - ./ledgers:/data
    command: proxy --port 8080 --ledger /data/plarix-ledger.jsonl

  app:
    image: my-app
    environment:
      - OPENAI_BASE_URL=http://plarix:8080/openai
      - ANTHROPIC_BASE_URL=http://plarix:8080/anthropic
```

### 3. CI Configuration

**Inputs:**
- `command` (Required): The command to execute.
- `fail_on_cost_usd` (Optional): Exit code 1 if cost exceeded.
- `pricing_file` (Optional): Path to custom `prices.json`.
- `enable_openai_stream_usage_injection` (Optional, default `false`): Forces usage reporting for OpenAI streams.

---

## Accuracy Guarantee

Plarix Scan prioritizes **correctness over estimation**.
- **Provider Reported**: We ONLY record costs if the provider returns usage fields (e.g., `usage: { prompt_tokens: ... }`).
- **Real Streaming**: We intercept streaming bodies to parse usage chunks (e.g. OpenAI `stream_options`).
- **Unknown Models**: If a model is not in our pricing table, we record usage but mark cost as **Unknown**. We do not guess.

> **Note on Stubs**: If your tests use stubs/mocks (e.g. VCR cassettes), Plarix won't see any traffic, and cost will be $0. This is expected.