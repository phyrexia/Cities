// Package ai provides LLM integration for generating city initiatives.
// Supports three backends (selected via LLM_BACKEND env var):
//
//   ANTHROPIC  — Claude API via Anthropic SDK (default, best quality)
//   OPENROUTER — OpenRouter.ai (access 200+ models with one API key)
//   LOCAL      — Local LLM via OpenAI-compatible HTTP endpoint (Ollama, LM Studio, etc.)
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Backend selects which LLM provider to use.
type Backend string

const (
	BackendAnthropic  Backend = "ANTHROPIC"
	BackendOpenRouter Backend = "OPENROUTER"
	BackendLocal      Backend = "LOCAL"
)

// Client wraps LLM backends for initiative generation.
type Client struct {
	backend    Backend
	model      string
	enabled    bool
	// Anthropic SDK client (used only for BackendAnthropic)
	anthropicSDK *anthropic.Client
	// HTTP client for OpenRouter / Local backends
	httpClient   *http.Client
	baseURL      string
	apiKey       string
}

// NewClient creates an LLM client based on environment variables.
//
// Environment variables:
//   LLM_BACKEND        = ANTHROPIC | OPENROUTER | LOCAL  (default: ANTHROPIC)
//   ANTHROPIC_API_KEY  = sk-ant-...  (Anthropic backend)
//   ANTHROPIC_MODEL    = claude-opus-4-6  (optional, default)
//   OPENROUTER_API_KEY = sk-or-...  (OpenRouter backend)
//   OPENROUTER_MODEL   = anthropic/claude-opus-4-6  (or google/gemini-flash-1.5, etc.)
//   LOCAL_LLM_URL      = http://localhost:11434/v1  (Ollama / LM Studio)
//   LOCAL_LLM_MODEL    = llama3  (or mistral, phi3, etc.)
func NewClient() *Client {
	backend := Backend(strings.ToUpper(os.Getenv("LLM_BACKEND")))
	if backend == "" {
		backend = BackendAnthropic
	}
	switch backend {
	case BackendOpenRouter:
		return newOpenRouterClient()
	case BackendLocal:
		return newLocalClient()
	default:
		return newAnthropicClient()
	}
}

func newAnthropicClient() *Client {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("[AI] No ANTHROPIC_API_KEY — using fallback initiatives.")
		fmt.Println("[AI] Alternatives: set LLM_BACKEND=OPENROUTER or LLM_BACKEND=LOCAL")
		return &Client{backend: BackendAnthropic, enabled: false}
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-opus-4-6"
	}
	fmt.Printf("[AI] Backend: Anthropic Claude (%s)\n", model)
	return &Client{
		backend:      BackendAnthropic,
		model:        model,
		enabled:      true,
		anthropicSDK: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func newOpenRouterClient() *Client {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Println("[AI] No OPENROUTER_API_KEY — using fallback initiatives")
		return &Client{backend: BackendOpenRouter, enabled: false}
	}
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "anthropic/claude-opus-4-6"
	}
	fmt.Printf("[AI] Backend: OpenRouter (%s)\n", model)
	return &Client{
		backend:    BackendOpenRouter,
		model:      model,
		enabled:    true,
		httpClient: &http.Client{},
		baseURL:    "https://openrouter.ai/api/v1",
		apiKey:     apiKey,
	}
}

func newLocalClient() *Client {
	baseURL := os.Getenv("LOCAL_LLM_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1" // Ollama default
	}
	model := os.Getenv("LOCAL_LLM_MODEL")
	if model == "" {
		model = "llama3"
	}
	fmt.Printf("[AI] Backend: Local LLM at %s (%s)\n", baseURL, model)
	return &Client{
		backend:    BackendLocal,
		model:      model,
		enabled:    true,
		httpClient: &http.Client{},
		baseURL:    baseURL,
	}
}

// Complete sends a prompt and returns the text response.
func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
	if !c.enabled {
		return "", fmt.Errorf("LLM client not configured (see LLM_BACKEND env var)")
	}
	switch c.backend {
	case BackendAnthropic:
		return c.completeAnthropic(ctx, prompt)
	case BackendOpenRouter, BackendLocal:
		return c.completeOpenAICompat(ctx, prompt)
	default:
		return "", fmt.Errorf("unknown backend: %s", c.backend)
	}
}

func (c *Client) completeAnthropic(ctx context.Context, prompt string) (string, error) {
	msg, err := c.anthropicSDK.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.Model(c.model)),
		MaxTokens: anthropic.F(int64(2048)),
		System: anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(systemPrompt),
		}),
		Messages: anthropic.F([]anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		}),
	})
	if err != nil {
		return "", fmt.Errorf("anthropic API: %w", err)
	}
	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in anthropic response")
}

// openAIRequest is the payload for OpenAI-compatible chat endpoints.
type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) completeOpenAICompat(ctx context.Context, prompt string) (string, error) {
	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 2048,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.backend == BackendOpenRouter {
		req.Header.Set("HTTP-Referer", "https://cities-game.local")
		req.Header.Set("X-Title", "Cities Social Simulator")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w\nraw: %s", err, string(respBody))
	}
	if result.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return result.Choices[0].Message.Content, nil
}

// IsEnabled returns true if the client is configured.
func (c *Client) IsEnabled() bool { return c.enabled }

// BackendName returns a human-readable description of the active backend.
func (c *Client) BackendName() string {
	return fmt.Sprintf("%s/%s", c.backend, c.model)
}

const systemPrompt = `You are a game coordinator AI for "Cities", a multiplayer city-building simulator.
You generate balanced, contextually appropriate policy proposals for mayors.
Always respond with valid JSON only — no markdown, no explanation, no code blocks.`
