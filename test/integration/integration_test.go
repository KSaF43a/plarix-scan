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

var (
	plarixScanPath string
)

func TestMain(m *testing.M) {
	// Build plarix-scan binary
	tmpDir, err := os.MkdirTemp("", "plarix-integration")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	plarixScanPath = filepath.Join(tmpDir, "plarix-scan")
	cmd := exec.Command("go", "build", "-o", plarixScanPath, "../../cmd/plarix-scan")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build plarix-scan: %v\nOutput:\n%s\n", err, output)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestEndToEnd(t *testing.T) {
	// 1. Start Mock Servers
	openaiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "Not found", 404)
			return
		}

		// Check for unknown model
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		model := body["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		if model == "unknown-model-99" {
			w.Write([]byte(`{
				"id": "chatcmpl-unknown",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "unknown-model-99",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "Who am I?"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10}
			}`))
		} else {
			// GTP-4
			w.Write([]byte(`{
				"id": "chatcmpl-gpt4",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "I am GPT-4"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
			}`))
		}
	}))
	defer openaiMock.Close()

	anthropicMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "Not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "msg_claude",
			"type": "message",
			"role": "assistant",
			"model": "claude-3-opus-20240229",
			"content": [{"type": "text", "text": "I am Claude"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 15, "output_tokens": 25}
		}`))
	}))
	defer anthropicMock.Close()

	// 2. Prepare environment and inputs
	// We need to tell plarix-scan about the mock provider URLs.
	// Wait, plarix-scan hardcodes provider URLs? No, it proxies requests.
	// BUT plarix-scan needs to know where to send the request *to*.
	// The current implementation of plarix-scan likely forwards to hardcoded https://api.openai.com etc.
	// I need to check `internal/proxy/proxy.go` to see how it resolves upstreams.
	// If it hardcodes upstreams, I need to override them for testing.

	// Let's assume for now I need to check proxy configuration.
	// I will read `internal/proxy/proxy.go` AFTER this to verify.
	// For now, I'll proceed assuming I can override upstreams via env vars or it respects the request host?
	// Actually, `plarix-scan` sets `OPENAI_BASE_URL` to `localhost:port/openai`.
	// The proxy at `localhost:port/openai` receives the request.
	// It strips `/openai` and forwards to... where?
	// If it forwards to `api.openai.com` hardcoded, I cannot test without internet.
	// I MUST be able to override the upstream for testing.
	// I will add a check for `PLARIX_UPSTREAM_OPENAI` etc. in the proxy code if not present.
	// Or I'll see if I can pass it.

	// I will finish writing this test file assuming I can set env vars like `TEST_UPSTREAM_OPENAI`.

	workDir, err := os.MkdirTemp("", "plarix-work")
	if err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	// Copy victim to workDir (or just run it)

	// 3. Run plarix-scan
	cmd := exec.Command(plarixScanPath, "run", "--command", "go run ../victim/main.go")
	cmd.Dir = workDir

	// Set env vars for upstreams (Checking proxy.go is critical next step)
	env := os.Environ()
	// Override upstreams to point to mocks
	env = append(env, fmt.Sprintf("PLARIX_UPSTREAM_OPENAI=%s", openaiMock.URL))
	env = append(env, fmt.Sprintf("PLARIX_UPSTREAM_ANTHROPIC=%s", anthropicMock.URL))
	// We also need to locate the victim code correctly.
	// If workDir is temp, `../victim/main.go` might not be valid relative path.
	// I should use absolute path to victim.
	wd, _ := os.Getwd()
	victimPath := filepath.Join(wd, "victim", "main.go")
	cmd.Args = []string{plarixScanPath, "run", "--command", fmt.Sprintf("go run %s", victimPath)}

	cmd.Env = env

	// We need to make sure plarix-scan can find `prices/prices.json`.
	// Since we are running from a temp dir, we might need to point it to the prices file.
	pricesPath := filepath.Join(wd, "../../prices/prices.json")
	cmd.Args = append(cmd.Args, "--pricing", pricesPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plarix-scan failed: %v\nOutput:\n%s", err, output)
	}
	t.Logf("plarix-scan output:\n%s", output)

	// 4. Verify Ledger
	ledgerPath := filepath.Join(workDir, "plarix-ledger.jsonl")
	f, err := os.Open(ledgerPath)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	defer f.Close()

	var entries []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("bad json in ledger: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 ledger entries, got %d", len(entries))
	}

	// Check GPT-4
	gpt4 := entries[0]
	if gpt4["model"] != "gpt-4" {
		t.Errorf("entry 0 model mismatch: %v", gpt4["model"])
	}
	// Verify cost > 0
	if cost, ok := gpt4["cost_usd"].(float64); !ok || cost <= 0 {
		t.Errorf("entry 0 cost invalid: %v", gpt4["cost_usd"])
	}

	// Check Unknown
	unknown := entries[2]
	if unknown["model"] != "unknown-model-99" {
		t.Errorf("entry 2 model mismatch: %v", unknown["model"])
	}
	// Verify unknown
	if unknown["cost_known"].(bool) {
		t.Errorf("entry 2 should have unknown cost")
	}
}
