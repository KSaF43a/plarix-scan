package proxy

import (
	"bytes"
	"encoding/json"
	"io"

	"plarix-action/internal/ledger"
)

// usageStreamInterceptor wraps an io.ReadCloser (the upstream response body)
// to transparently inspect Server-Sent Events (SSE) as they are read by the client.
// It effectively "forks" the stream: one copy goes to the client (via Read),
// and another is processed internally to extract token usage stats.
type usageStreamInterceptor struct {
	originalBody io.ReadCloser
	provider     string
	onComplete   func(ledger.Entry)
	entry        ledger.Entry

	// buffering for incomplete lines
	lineBuffer bytes.Buffer
}

func newStreamInterceptor(body io.ReadCloser, provider, endpoint string, onComplete func(ledger.Entry)) *usageStreamInterceptor {
	return &usageStreamInterceptor{
		originalBody: body,
		provider:     provider,
		onComplete:   onComplete,
		entry: ledger.Entry{
			Provider:      provider,
			Endpoint:      endpoint,
			Streaming:     true,
			CostKnown:     false, // Default to false unless we find usage
			UnknownReason: "usage not found in stream",
		},
	}
}

// Read implements io.Reader. It reads from the upstream response and
// immediately inspects the chunk for usage data before returning it.
// This ensures that we capture usage even if the client disconnects later,
// provided the data actually came through the wire.
func (s *usageStreamInterceptor) Read(p []byte) (n int, err error) {
	n, err = s.originalBody.Read(p)
	if n > 0 {
		// Process the chunk we just read
		// Note: This might be expensive on high throughput, but necessary for inspection.
		// We copy to avoid interfering with the buffer passed to Read (though we only read from it).
		// Wait, 'p' is where data was written TO. We should inspect 'p[:n]'.
		s.scanChunk(p[:n])
	}
	return n, err
}

func (s *usageStreamInterceptor) Close() error {
	// When stream closes, finalize usage and call callback
	if s.onComplete != nil {
		s.onComplete(s.entry)
	}
	return s.originalBody.Close()
}

func (s *usageStreamInterceptor) scanChunk(chunk []byte) {
	// TCP packets don't respect line boundaries. We might get half a JSON line.
	// We buffer incoming bytes until we find a newline, then process the complete line.
	s.lineBuffer.Write(chunk)

	// Process complete lines
	for {
		line, err := s.lineBuffer.ReadBytes('\n')
		if err != nil {
			// EOF or no newline found.
			// If we read everything and no newline, put it back?
			// bytes.Buffer.ReadBytes consumes tokens.
			// If err == io.EOF, we have a partial line remaining in buffer (it's returned in line).
			// We should put it back for next time?
			// Actually ReadBytes returns the data before error.
			// If error is EOF, we should restore buffer.
			s.lineBuffer.Write(line) // Put back
			break
		}

		// We have a full line
		s.processLine(line)
	}
}

func (s *usageStreamInterceptor) processLine(line []byte) {
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data: ")) {
		// Anthropic also has "event: ..." lines, but data comes in "data:".
		return
	}

	data := bytes.TrimPrefix(trimmed, []byte("data: "))
	if string(data) == "[DONE]" {
		return
	}

	// Try to parse JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	// Check provider specific usage
	if s.provider == "openai" {
		// OpenAI stream_options usage comes in a separate chunk, usually the last one.
		// { "usage": { ... } }
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			s.extractOpenAIUsage(usage)
			// Also model might be in this chunk or previous chunks.
			// Usually usage chunk has model?
			// "model": "gpt-4-0613" is often in the first chunk or all chunks.
			// We should capture model from any chunk if missing.
		}
		if model, ok := payload["model"].(string); ok && s.entry.Model == "" {
			s.entry.Model = model
		}
	} else if s.provider == "anthropic" {
		// Anthropic SSE:
		// event: message_start -> data: { message: { usage: {...} } }
		// event: message_delta -> data: { usage: {...} } (output tokens)
		// event: message_stop -> data: ...

		// Note usage can be in message_start (input) and message_delta (output).
		// We need to accumulate?
		// "message_start": { "message": { "usage": { "input_tokens": 20 } } }
		// "message_delta": { "usage": { "output_tokens": 10 } }

		// Check for message_start type usage
		if msg, ok := payload["message"].(map[string]interface{}); ok {
			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				s.accumulateAnthropicUsage(usage)
			}
			if model, ok := msg["model"].(string); ok && s.entry.Model == "" {
				s.entry.Model = model
			}
		}

		// Check for usage directly (delta)
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			s.accumulateAnthropicUsage(usage)
		}
	}
}

func (s *usageStreamInterceptor) extractOpenAIUsage(usage map[string]interface{}) {
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		s.entry.InputTokens = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		s.entry.OutputTokens = int(ct)
	}
	// If we found usage, we mark it potentially known (depends on pricing)
	// But we definitely "found usage".
	s.entry.CostKnown = true
	s.entry.UnknownReason = ""
}

func (s *usageStreamInterceptor) accumulateAnthropicUsage(usage map[string]interface{}) {
	if it, ok := usage["input_tokens"].(float64); ok {
		s.entry.InputTokens += int(it)
	}
	if ot, ok := usage["output_tokens"].(float64); ok {
		s.entry.OutputTokens += int(ot)
	}
	// Mark as found
	s.entry.CostKnown = true
	s.entry.UnknownReason = ""
}
