// Package openai handles parsing OpenAI API responses.
//
// Purpose: Extract usage data from OpenAI Chat Completions and Responses API.
// Public API: ParseResponse
// Usage: Call ParseResponse with response body to get ledger entry fields.
package openai

import (
	"encoding/json"

	"plarix-action/internal/ledger"
)

// Response represents an OpenAI API response with usage data.
type Response struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Object string `json:"object"`
	Usage  *Usage `json:"usage,omitempty"`
}

// Usage holds token usage from OpenAI response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// Extended fields for newer models
	PromptTokensDetails     map[string]int `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails map[string]int `json:"completion_tokens_details,omitempty"`
}

// ParseResponse extracts usage data from an OpenAI API response.
// Updates the entry in place with model, tokens, and raw usage.
func ParseResponse(body []byte, entry *ledger.Entry) {
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		entry.CostKnown = false
		entry.UnknownReason = "failed to parse response"
		return
	}

	entry.Model = resp.Model
	entry.RequestID = resp.ID

	if resp.Usage == nil {
		entry.CostKnown = false
		entry.UnknownReason = "no usage field in response"
		return
	}

	entry.InputTokens = resp.Usage.PromptTokens
	entry.OutputTokens = resp.Usage.CompletionTokens

	// Store raw usage for transparency
	entry.RawUsage = map[string]interface{}{
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
	}

	// Include extended token details if present
	if len(resp.Usage.PromptTokensDetails) > 0 {
		entry.RawUsage["prompt_tokens_details"] = resp.Usage.PromptTokensDetails
	}
	if len(resp.Usage.CompletionTokensDetails) > 0 {
		entry.RawUsage["completion_tokens_details"] = resp.Usage.CompletionTokensDetails
	}

	// Mark as knowing tokens but cost calculation is external
	entry.CostKnown = true
}
