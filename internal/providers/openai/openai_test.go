package openai

import (
	"testing"

	"plarix-action/internal/ledger"
)

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantModel     string
		wantInput     int
		wantOutput    int
		wantCostKnown bool
		wantReason    string
	}{
		{
			name: "valid chat completion",
			body: `{
				"id": "chatcmpl-123",
				"model": "gpt-4o",
				"object": "chat.completion",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 50,
					"total_tokens": 150
				}
			}`,
			wantModel:     "gpt-4o",
			wantInput:     100,
			wantOutput:    50,
			wantCostKnown: true,
		},
		{
			name: "missing usage",
			body: `{
				"id": "chatcmpl-123",
				"model": "gpt-4o",
				"object": "chat.completion"
			}`,
			wantModel:     "gpt-4o",
			wantCostKnown: false,
			wantReason:    "no usage field in response",
		},
		{
			name: "with extended token details",
			body: `{
				"id": "chatcmpl-456",
				"model": "gpt-4o",
				"usage": {
					"prompt_tokens": 200,
					"completion_tokens": 100,
					"total_tokens": 300,
					"prompt_tokens_details": {"cached_tokens": 50},
					"completion_tokens_details": {"reasoning_tokens": 20}
				}
			}`,
			wantModel:     "gpt-4o",
			wantInput:     200,
			wantOutput:    100,
			wantCostKnown: true,
		},
		{
			name:          "invalid json",
			body:          `{invalid}`,
			wantCostKnown: false,
			wantReason:    "failed to parse response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &ledger.Entry{}
			ParseResponse([]byte(tt.body), entry)

			if entry.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", entry.Model, tt.wantModel)
			}
			if entry.InputTokens != tt.wantInput {
				t.Errorf("InputTokens = %d, want %d", entry.InputTokens, tt.wantInput)
			}
			if entry.OutputTokens != tt.wantOutput {
				t.Errorf("OutputTokens = %d, want %d", entry.OutputTokens, tt.wantOutput)
			}
			if entry.CostKnown != tt.wantCostKnown {
				t.Errorf("CostKnown = %v, want %v", entry.CostKnown, tt.wantCostKnown)
			}
			if tt.wantReason != "" && entry.UnknownReason != tt.wantReason {
				t.Errorf("UnknownReason = %q, want %q", entry.UnknownReason, tt.wantReason)
			}
		})
	}
}
