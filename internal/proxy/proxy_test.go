package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plarix-action/internal/ledger"
)

// TestProxyOpenAI tests the proxy with a mock OpenAI server.
func TestProxyOpenAI(t *testing.T) {
	// Mock OpenAI response
	mockResponse := map[string]interface{}{
		"id":      "chatcmpl-test123",
		"model":   "gpt-4o",
		"object":  "chat.completion",
		"choices": []map[string]interface{}{},
		"usage": map[string]interface{}{
			"prompt_tokens":     100,
			"completion_tokens": 50,
			"total_tokens":      150,
		},
	}

	// Create mock OpenAI server
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer mockOpenAI.Close()

	// Override provider target
	originalTarget := providerTargets["openai"]
	providerTargets["openai"] = mockOpenAI.URL
	defer func() { providerTargets["openai"] = originalTarget }()

	// Track received entries
	var receivedEntry ledger.Entry
	entryCh := make(chan ledger.Entry, 1)

	// Start proxy
	config := Config{
		Providers: []string{"openai"},
		OnEntry: func(e ledger.Entry) {
			entryCh <- e
		},
	}

	server := NewServer(config)
	port, err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer server.Stop()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%d/openai/v1/chat/completions", port),
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}

	// Verify response body was forwarded
	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if respData["model"] != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", respData["model"])
	}

	// Wait for entry
	select {
	case receivedEntry = <-entryCh:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for entry")
	}

	// Verify entry
	if receivedEntry.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", receivedEntry.Provider)
	}
	if receivedEntry.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", receivedEntry.Model)
	}
	if receivedEntry.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", receivedEntry.InputTokens)
	}
	if receivedEntry.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", receivedEntry.OutputTokens)
	}
	if !receivedEntry.CostKnown {
		t.Errorf("CostKnown = false, want true")
	}
}
