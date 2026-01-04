package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
)

func main() {
	// 1. OpenAI Call
	openaiBase := os.Getenv("OPENAI_BASE_URL")
	if openaiBase == "" {
		fmt.Println("Error: OPENAI_BASE_URL not set")
		os.Exit(1)
	}

	makeRequest(openaiBase+"/v1/chat/completions", `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`)

	// 2. Anthropic Call
	anthropicBase := os.Getenv("ANTHROPIC_BASE_URL")
	if anthropicBase == "" {
		fmt.Println("Error: ANTHROPIC_BASE_URL not set")
		os.Exit(1)
	}

	makeRequest(anthropicBase+"/v1/messages", `{"model": "claude-3-opus-20240229", "messages": [{"role": "user", "content": "Hello"}]}`)

	// 3. Unknown Model Call
	makeRequest(openaiBase+"/v1/chat/completions", `{"model": "unknown-model-99", "messages": [{"role": "user", "content": "Hello"}]}`)
}

func makeRequest(url, body string) {
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// Mock servers won't check keys, but we send them to simulate real client
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set("x-api-key", "sk-test") // Anthropic

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Request returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}
