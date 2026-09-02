package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// LLMClassifier interface for plugging in different LLM providers
type LLMClassifier interface {
	Classify(ctx context.Context, input FailureLogInput) (*ClassificationResult, error)
}

// OpenAIClassifier uses OpenAI's API for classification
type OpenAIClassifier struct {
	apiKey  string
	baseURL string
	model   string
}

// NewOpenAIClassifier creates a new OpenAI-based classifier
func NewOpenAIClassifier(apiKey string) *OpenAIClassifier {
	return &OpenAIClassifier{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		model:   "gpt-3.5-turbo",
	}
}

// Classify sends the failure logs to OpenAI for classification
func (o *OpenAIClassifier) Classify(ctx context.Context, input FailureLogInput) (*ClassificationResult, error) {
	if o.apiKey == "" {
		return nil, errors.New("OpenAI API key not configured")
	}

	// Construct the prompt
	prompt := constructClassificationPrompt(input)

	// Build request
	req := &openaiRequest{
		Model: o.model,
		Messages: []openaiMsg{
			{Role: "system", Content: "You are a CI failure classifier. Respond only with valid JSON."},
			{Role: "user", Content: prompt},
		},
	}

	// Call OpenAI API
	resp, err := callOpenAI(ctx, o.apiKey, o.baseURL, req)
	if err != nil {
		return nil, err
	}

	// Parse response
	return parseOpenAIResponse(resp)
}

func constructClassificationPrompt(input FailureLogInput) string {
	sb := strings.Builder{}
	sb.WriteString("Classify this CI failure as 'infra', 'flaky', or 'real'.\n\n")
	sb.WriteString("Job: " + input.JobName + "\n")
	if input.TestName != "" {
		sb.WriteString("Test: " + input.TestName + "\n")
	}
	sb.WriteString("Provider: " + input.Provider + "\n\n")
	sb.WriteString("Logs:\n")
	sb.WriteString(input.Logs)
	sb.WriteString("\n\nRespond with JSON: {\"category\": \"infra|flaky|real\", \"confidence\": 0.0-1.0, \"reasoning\": \"...\"}")
	return sb.String()
}

// MockLLMClassifier for testing
type MockLLMClassifier struct {
	Response *ClassificationResult
	Error    error
}

func (m *MockLLMClassifier) Classify(ctx context.Context, input FailureLogInput) (*ClassificationResult, error) {
	return m.Response, m.Error
}

// openaiRequest represents the API request structure
type openaiRequest struct {
	Model    string      `json:"model"`
	Messages []openaiMsg `json:"messages"`
	JSONMode bool        `json:"response_format,omitempty"` // for structured output
}

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse represents the API response structure
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// parseOpenAIResponse extracts the classification from OpenAI response
func parseOpenAIResponse(resp *openaiResponse) (*ClassificationResult, error) {
	if resp.Error != nil {
		return nil, errors.New("openai error: " + resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	content := resp.Choices[0].Message.Content

	// Parse JSON from content
	var classif struct {
		Category   string  `json:"category"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(content), &classif); err != nil {
		return nil, err
	}

	// Convert category string to FailureCategory
	var cat FailureCategory
	switch strings.ToLower(classif.Category) {
	case "infra":
		cat = CategoryInfra
	case "flaky":
		cat = CategoryFlaky
	case "real":
		cat = CategoryReal
	default:
		cat = CategoryReal
	}

	return &ClassificationResult{
		Category:   cat,
		Confidence: classif.Confidence,
		Reasoning:  classif.Reasoning,
		Method:     "llm",
	}, nil
}

// callOpenAI makes the HTTP request to OpenAI
func callOpenAI(ctx context.Context, apiKey, baseURL string, req *openaiRequest) (*openaiResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("OpenAI API error: " + resp.Status)
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, err
	}

	return &openaiResp, nil
}
