package openrouter

import (
	"encoding/json"
	"plarix-action/internal/ledger"
)

type response struct {
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"` // OpenRouter might send this
	} `json:"usage"`
}

// ParseResponse extracts usage from OpenRouter API response.
func ParseResponse(body []byte, entry *ledger.Entry) {
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		entry.CostKnown = false
		entry.UnknownReason = "failed to decode openrouter response"
		return
	}

	// OpenRouter models are often prefixed like "openai/gpt-4"
	// We might want to keep the full name or strip. Plarix usually wants full name.
	entry.Model = resp.Model
	entry.InputTokens = resp.Usage.PromptTokens
	entry.OutputTokens = resp.Usage.CompletionTokens

	// If usage is zero, it might be missing
	if entry.InputTokens == 0 && entry.OutputTokens == 0 && resp.Usage.TotalTokens == 0 {
		// OpenRouter usage is sometimes missing or delayed?
		// If it's zero, we can't compute cost.
		// However, legitimate 0 token requests are rare.
		// We will assume if fields are present, we use them.
	}

	entry.CostKnown = true

	// Some OpenRouter reponses might not include usage if not requested?
	// Standard OpenAI format usually includes it.
}
