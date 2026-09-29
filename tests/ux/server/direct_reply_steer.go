package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var directSteerPattern = regexp.MustCompile(`UX direct steer:([a-zA-Z0-9_-]+)`)

// serveDirectReplySteer uses the ordinary OpenAI-compatible provider route.
// A held first direct reply lets the native HTTP/SQLite/SSE path accept Steer.
func serveDirectReplySteer(w http.ResponseWriter, body map[string]any, gates string, emit func(any)) bool {
	messages, _ := body["messages"].([]any)
	var token string
	var hasSteer, hasFirstAnswer bool
	for _, entry := range messages {
		message, _ := entry.(map[string]any)
		encoded, _ := json.Marshal(message["content"])
		content := string(encoded)
		if message["role"] == "user" {
			if match := directSteerPattern.FindStringSubmatch(content); len(match) > 1 {
				token = match[1]
			}
			if strings.Contains(content, "steer after direct answer") {
				hasSteer = true
			}
		}
		if message["role"] == "assistant" && strings.Contains(content, "First direct answer "+token) {
			hasFirstAnswer = true
		}
	}
	if token == "" {
		return false
	}
	if hasSteer {
		answer := "Second direct answer " + token
		if !hasFirstAnswer {
			answer = "MISSING_FIRST_ANSWER " + token
		}
		emitDirectSteerAnswer(w, emit, answer)
		return true
	}
	// The first response is already accepted by the provider but not finished.
	// Wait for an explicit test-owned release after Gi accepts steering.
	deadline := time.After(30 * time.Second)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			return true
		case <-tick.C:
			if _, err := os.Stat(filepath.Join(gates, token)); err == nil {
				emitDirectSteerAnswer(w, emit, "First direct answer "+token)
				return true
			}
		}
	}
}

func emitDirectSteerAnswer(w http.ResponseWriter, emit func(any), answer string) {
	emit(map[string]any{"id": "direct-steer-fixture", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": answer}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 8, "total_tokens": 18}})
	fmt.Fprint(w, "data: [DONE]\n\n")
}
