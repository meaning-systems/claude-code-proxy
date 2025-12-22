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
	apiKey string
	model  string
)

func main() {
	apiKey = os.Getenv("PROXY_API_KEY")
	if apiKey == "" {
		log.Fatal("PROXY_API_KEY environment variable required")
	}

	model = os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "haiku" // Default to haiku (fast and cheap)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/v1/chat/completions", handleChat)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("Claude Code proxy starting on :%s (model: %s)", port, model)
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

	// Call Claude Code CLI
	cmd := exec.Command("claude",
		"--print",
		"--model", model,
	)
	cmd.Stdin = strings.NewReader(prompt.String())

	log.Printf("Processing request (%d messages, %d chars)", len(req.Messages), len(prompt.String()))
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
		Model:   "claude-" + model,
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
