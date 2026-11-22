package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type RouterClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
}

type ChatRequest struct {
	Model        string `json:"model"`
	SystemPrompt string `json:"-"`
	Context      string `json:"-"`
	UserMessage  string `json:"-"`

	// Router specific
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

type ChatResponse struct {
	ID      string    `json:"id"`
	Choices []Choice  `json:"choices"`
	Usage   Usage     `json:"usage"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type apiRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`

	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

func NewOpenRouterClient(apiKey string) *RouterClient {
	return &RouterClient{
		apiKey:  apiKey,
		baseURL: "https://openrouter.ai/api/v1",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		maxRetries: 3,
	}
}

func (c *RouterClient) Chat(ctx context.Context, req ChatRequest) (string, error) {
	// Формируем messages
	messages := []Message{}

	// System message с контекстом
	if req.SystemPrompt != "" || req.Context != "" {
		systemContent := req.SystemPrompt
		if req.Context != "" {
			systemContent += "\n\nContext:\n" + req.Context
		}
		messages = append(messages, Message{
			Role:    "system",
			Content: systemContent,
		})
	}

	// User message
	messages = append(messages, Message{
		Role:    "user",
		Content: req.UserMessage,
	})

	apiReq := apiRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
	}

	// Retry логика
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Printf("Retry attempt %d after %v", attempt, backoff)

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := c.doRequest(ctx, apiReq)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Не ретраим если контекст отменен
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Не ретраим на 4xx ошибках (кроме 429)
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Code != "rate_limit_exceeded" && apiErr.Code != "timeout" {
				return "", err
			}
		}

		log.Printf("Request failed (attempt %d): %v", attempt+1, err)
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *RouterClient) doRequest(ctx context.Context, req apiRequest) (string, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/yourusername/ai-assistant") // опционально
	httpReq.Header.Set("X-Title", "AI Assistant")                                      // опционально

	log.Printf("Sending request to Router (model: %s, messages: %d)", req.Model, len(req.Messages))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Логируем для дебага
	if log.Default().Flags()&log.Lshortfile != 0 {
		log.Printf("Response status: %d, body length: %d", resp.StatusCode, len(body))
	}

	// Проверяем HTTP статус
	if resp.StatusCode != http.StatusOK {
		// Пытаемся распарсить ошибку
		var errResp ChatResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("API error (status %d): %s (code: %s)",
				resp.StatusCode, errResp.Error.Message, errResp.Error.Code)
		}
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Парсим успешный ответ
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Проверяем на ошибку в ответе
	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s (code: %s)", chatResp.Error.Message, chatResp.Error.Code)
	}

	// Проверяем что есть хотя бы один choice
	if len(chatResp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	// Логируем usage
	log.Printf("Usage: prompt=%d, completion=%d, total=%d tokens",
		chatResp.Usage.PromptTokens,
		chatResp.Usage.CompletionTokens,
		chatResp.Usage.TotalTokens)

	return chatResp.Choices[0].Message.Content, nil
}

// StreamChat - для streaming ответов (опционально)
func (c *RouterClient) StreamChat(ctx context.Context, req ChatRequest, callback func(string)) error {
	// TODO: реализовать streaming если нужно
	return errors.New("streaming not implemented")
}
