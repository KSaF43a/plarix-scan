package anthropic

import (
	"encoding/json"
	"plarix-action/internal/ledger"
)

type response struct {
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ParseResponse extracts usage from Anthropic API response.
func ParseResponse(body []byte, entry *ledger.Entry) {
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		entry.CostKnown = false
		entry.UnknownReason = "failed to decode anthropic response key"
		return
	}

	entry.Model = resp.Model
	entry.InputTokens = resp.Usage.InputTokens
	entry.OutputTokens = resp.Usage.OutputTokens

	// Anthropic always provides usage on success, so we mark it cost-known
	// (Pricing calculation will determine if we actually know the price)
	entry.CostKnown = true
}
