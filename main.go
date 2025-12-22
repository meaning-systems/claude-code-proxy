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
	"bufio"
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
	Stream   bool      `json:"stream"`
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
	Message      Message `json:"message,omitempty"`
	Delta        *Delta  `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
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

// Claude CLI streaming JSON structures
type ClaudeStreamMessage struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"text"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Result string `json:"result"`
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

	log.Printf("Claude Code proxy starting on :%s (default model: %s, streaming: enabled)", port, defaultModel)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	// Verify API key
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != apiKey {
		w.Header().Set("Content-Type", "application/json")
		sendError(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		sendError(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
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

	if req.Stream {
		handleStreamingRequest(w, prompt.String(), requestModel)
	} else {
		handleNonStreamingRequest(w, prompt.String(), requestModel)
	}
}

func handleNonStreamingRequest(w http.ResponseWriter, prompt string, model string) {
	w.Header().Set("Content-Type", "application/json")

	cmd := exec.Command("claude",
		"--print",
		"--model", model,
	)
	cmd.Stdin = strings.NewReader(prompt)

	log.Printf("Processing request (model: %s, %d chars)", model, len(prompt))
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

	resp := ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
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
			PromptTokens:     len(prompt) / 4,
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(prompt) + len(response)) / 4,
		},
	}

	json.NewEncoder(w).Encode(resp)
}

func handleStreamingRequest(w http.ResponseWriter, prompt string, model string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("claude",
		"--print",
		"--model", model,
		"--output-format", "stream-json",
		"--verbose",
	)
	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		sendSSEError(w, flusher, "Failed to start Claude CLI")
		return
	}

	log.Printf("Processing streaming request (model: %s, %d chars)", model, len(prompt))
	start := time.Now()

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start Claude CLI: %v", err)
		sendSSEError(w, flusher, "Failed to start Claude CLI")
		return
	}

	chatID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	sentRole := false

	scanner := bufio.NewScanner(stdout)
	// Increase buffer size for large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)

		// Handle assistant message with content
		if msgType == "assistant" {
			if message, ok := msg["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].([]interface{}); ok {
					for _, c := range content {
						if contentMap, ok := c.(map[string]interface{}); ok {
							if text, ok := contentMap["text"].(string); ok && text != "" {
								// Send role first if not sent
								if !sentRole {
									chunk := ChatResponse{
										ID:      chatID,
										Object:  "chat.completion.chunk",
										Created: created,
										Model:   model,
										Choices: []Choice{{
											Index: 0,
											Delta: &Delta{Role: "assistant"},
										}},
									}
									sendSSEChunk(w, flusher, chunk)
									sentRole = true
								}

								// Send content chunk
								chunk := ChatResponse{
									ID:      chatID,
									Object:  "chat.completion.chunk",
									Created: created,
									Model:   model,
									Choices: []Choice{{
										Index: 0,
										Delta: &Delta{Content: text},
									}},
								}
								sendSSEChunk(w, flusher, chunk)
							}
						}
					}
				}
			}
		}

		// Handle result message (final)
		if msgType == "result" {
			if result, ok := msg["result"].(string); ok && result != "" && !sentRole {
				// Fallback: send full result if we didn't get streaming content
				chunk := ChatResponse{
					ID:      chatID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []Choice{{
						Index: 0,
						Delta: &Delta{Role: "assistant", Content: result},
					}},
				}
				sendSSEChunk(w, flusher, chunk)
				sentRole = true
			}
		}
	}

	// Send final chunk with finish_reason
	finalChunk := ChatResponse{
		ID:      chatID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []Choice{{
			Index:        0,
			Delta:        &Delta{},
			FinishReason: "stop",
		}},
	}
	sendSSEChunk(w, flusher, finalChunk)

	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	cmd.Wait()
	log.Printf("Streaming response completed in %v", time.Since(start))
}

func sendSSEChunk(w http.ResponseWriter, flusher http.Flusher, chunk ChatResponse) {
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func sendSSEError(w http.ResponseWriter, flusher http.Flusher, message string) {
	errResp := map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    "error",
		},
	}
	data, _ := json.Marshal(errResp)
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func sendError(w http.ResponseWriter, message string, status int) {
	w.WriteHeader(status)
	resp := ErrorResponse{}
	resp.Error.Message = message
	resp.Error.Type = "error"
	json.NewEncoder(w).Encode(resp)
}
