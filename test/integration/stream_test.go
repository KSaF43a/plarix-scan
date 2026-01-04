package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStreamingUsage(t *testing.T) {
	// 1. Start Mock Server (OpenAI Streaming)
	openaiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		// Flush immediately to ensure streaming behavior
		w.Write([]byte(`data: {"id":"chatcmpl-stream","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"}}]}
`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		w.Write([]byte(`data: {"id":"chatcmpl-stream","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"}}]}
`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Usage chunk (last)
		w.Write([]byte(`data: {"id":"chatcmpl-stream","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}
`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		w.Write([]byte(`data: [DONE]
`))
	}))
	defer openaiMock.Close()

	// 2. Run plarix-scan with curl (simulating client)
	workDir, err := os.MkdirTemp("", "plarix-stream")
	if err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	// We need a script that makes a streaming request.
	// simpler to just use a Go program again or inline logic.
	// Let's use a simple shell script with curl if available, or just a Victim Go program that sets stream=true.

	// I will write a simple go program to `stream_victim/main.go`
	victimSource := `package main
import (
    "bytes"
    "io"
    "net/http"
    "os"
)
func main() {
    url := os.Getenv("OPENAI_BASE_URL") + "/v1/chat/completions"
    req, _ := http.NewRequest("POST", url, bytes.NewBufferString("{\"stream\":true}"))
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { panic(err) }
    defer resp.Body.Close()
    // Read stream
    io.Copy(os.Stdout, resp.Body)
}
`
	victimDir := filepath.Join(workDir, "victim")
	os.MkdirAll(victimDir, 0755)
	os.WriteFile(filepath.Join(victimDir, "main.go"), []byte(victimSource), 0644)

	cmd := exec.Command(plarixScanPath, "run", "--command", "go run main.go")
	cmd.Dir = victimDir

	env := os.Environ()
	env = append(env, fmt.Sprintf("PLARIX_UPSTREAM_OPENAI=%s", openaiMock.URL))
	cmd.Env = env

	// Point to real prices
	wd, _ := os.Getwd()
	pricesPath := filepath.Join(wd, "../../prices/prices.json")
	cmd.Args = append(cmd.Args, "--pricing", pricesPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plarix-scan failed: %v\nOutput:\n%s", err, output)
	}
	t.Logf("Output: %s", output)

	// 3. Verify Ledger
	ledgerPath := filepath.Join(victimDir, "plarix-ledger.jsonl")
	f, err := os.Open(ledgerPath)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer f.Close()

	var entries []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e map[string]interface{}
		json.Unmarshal(scanner.Bytes(), &e)
		entries = append(entries, e)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e["streaming"] != true {
		t.Errorf("expected streaming=true")
	}
	// Verify usage captured
	// JSON numbers are floats in interface{}
	if input, ok := e["input_tokens"].(float64); !ok || input != 10 {
		t.Errorf("expected input 10, got %v", e["input_tokens"])
	}
	if output, ok := e["output_tokens"].(float64); !ok || output != 20 {
		t.Errorf("expected output 20, got %v", e["output_tokens"])
	}
	// Verify cost > 0 (GPT-4o)
	if cost, ok := e["cost_usd"].(float64); !ok || cost <= 0 {
		t.Errorf("expected >0 cost for GPT-4o, got %v", e["cost_usd"])
	}
}
