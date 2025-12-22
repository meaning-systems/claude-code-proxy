// claude-code-proxy - OpenAI-compatible API proxy for Claude Code CLI
//
// Uses your authenticated Claude Code (Max subscription) for inference
// instead of requiring separate API credits.
//
// Usage:
//   PROXY_API_KEY=your-secret go run main.go
//
// Then configure your app:
//   Endpoint: http://localhost:8080/v1/chat/completions
//   API Key: your-secret

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// OpenAI-compatible request/response structures
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

var (
	apiKey       string
	defaultModel string
)

// normalizeModel extracts the base model name (haiku, sonnet, opus)
func normalizeModel(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	// Strip common prefixes
	m = strings.TrimPrefix(m, "claude-")
	m = strings.TrimPrefix(m, "claude_")
	// Handle versioned names like "haiku-4-5" -> "haiku"
	for _, base := range []string{"haiku", "sonnet", "opus"} {
		if strings.HasPrefix(m, base) {
			return base
		}
	}
	// If not recognized, return as-is (let claude CLI handle it)
	if m == "" {
		return "sonnet" // default
	}
	return m
}

func main() {
	apiKey = os.Getenv("PROXY_API_KEY")
	if apiKey == "" {
		log.Fatal("PROXY_API_KEY environment variable required")
	}

	defaultModel = os.Getenv("CLAUDE_MODEL")
	if defaultModel == "" {
		defaultModel = "sonnet" // Default to sonnet
	}
	defaultModel = normalizeModel(defaultModel)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/v1/chat/completions", handleChat)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("Claude Code proxy starting on :%s (default model: %s)", port, defaultModel)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Verify API key
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != apiKey {
		sendError(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendError(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build prompt from messages
	var prompt strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n\n")
		case "user":
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n")
		case "assistant":
			prompt.WriteString("[Previous response: ")
			prompt.WriteString(msg.Content)
			prompt.WriteString("]\n")
		}
	}

	// Determine model: use request model if provided, otherwise default
	requestModel := normalizeModel(req.Model)
	if requestModel == "" {
		requestModel = defaultModel
	}

	// Call Claude Code CLI
	cmd := exec.Command("claude",
		"--print",
		"--model", requestModel,
	)
	cmd.Stdin = strings.NewReader(prompt.String())

	log.Printf("Processing request (model: %s, %d messages, %d chars)", requestModel, len(req.Messages), len(prompt.String()))
	start := time.Now()

	output, err := cmd.Output()
	if err != nil {
		log.Printf("Claude CLI error: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("Stderr: %s", string(exitErr.Stderr))
		}
		sendError(w, "Claude CLI failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(start)
	response := strings.TrimSpace(string(output))
	log.Printf("Response received in %v (%d chars)", elapsed, len(response))

	// Build OpenAI-format response
	resp := ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestModel,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: response,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     len(prompt.String()) / 4, // rough estimate
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(prompt.String()) + len(response)) / 4,
		},
	}

	json.NewEncoder(w).Encode(resp)
}

func sendError(w http.ResponseWriter, message string, status int) {
	w.WriteHeader(status)
	resp := ErrorResponse{}
	resp.Error.Message = message
	resp.Error.Type = "error"
	json.NewEncoder(w).Encode(resp)
}
