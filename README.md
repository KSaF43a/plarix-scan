# Plarix Scan

Free CI cost recorder for LLM API usage — records tokens and costs from real provider responses.

## What It Does

Plarix Scan is a GitHub Action that:

1. Starts a local HTTP forward-proxy (no TLS MITM, no custom certs)
2. Runs your test/build command
3. Intercepts LLM API calls when SDKs support base-URL overrides to plain HTTP
4. Records usage from real provider responses (not estimated)
5. Posts a cost summary to your PR

## Quick Start

```yaml
name: LLM Cost Tracking
on: [pull_request]

permissions:
  pull-requests: write

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: plarix-ai/scan@v1
        with:
          command: "pytest -q"
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `command` | Yes | — | Command to run (e.g., `pytest -q`, `npm test`) |
| `fail_on_cost_usd` | No | — | Exit non-zero if total cost exceeds threshold |
| `pricing_file` | No | bundled | Path to custom pricing JSON |
| `providers` | No | `openai,anthropic,openrouter` | Providers to intercept |
| `comment_mode` | No | `both` | Where to post: `pr`, `summary`, or `both` |
| `enable_openai_stream_usage_injection` | No | `false` | Opt-in for OpenAI streaming usage |

## Supported Providers (v1)

- **OpenAI** (Chat Completions + Responses API)
- **Anthropic** (Messages API)
- **OpenRouter** (OpenAI-compatible)

## How It Works

The action sets base URL environment variables to route SDK calls through the local proxy:

```
OPENAI_BASE_URL=http://127.0.0.1:<port>/openai
ANTHROPIC_BASE_URL=http://127.0.0.1:<port>/anthropic
OPENROUTER_BASE_URL=http://127.0.0.1:<port>/openrouter
```

**Requirements:**
- Your SDK must support base URL overrides via environment variables
- SDKs that require HTTPS or hardcode endpoints won't work

## Limitations

### Fork PRs
Secrets are usually unavailable on PRs from forks. In this case, Plarix Scan will report: "No provider secrets available; no real usage observed."

### Stubbed Tests
Many test suites stub LLM calls. If no real API calls are made, observed cost will be $0.

### SDK Compatibility
Not all SDKs support HTTP base URLs. If interception fails, the project is marked "Not interceptable".

## Output

- **PR Comment** (idempotent, updated each run)
- **GitHub Step Summary**
- `plarix-ledger.jsonl` — one JSON line per API call
- `plarix-summary.json` — aggregated totals

## Cost Calculation

Costs are computed **only** from provider-reported usage fields:
- No token estimation or guessing
- Unknown costs are reported explicitly
- Pricing from bundled `prices.json` (with staleness warnings)

## License

MIT