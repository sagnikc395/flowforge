// Package einoagent adapts an Eino chat model to anchora.Agent.
package einoagent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const defaultBaseURL = "https://router.huggingface.co/v1"

// Config configures an OpenAI-compatible Eino chat model. Hugging Face's
// router is the default endpoint, but BaseURL makes the adapter usable with
// any compatible provider.
type Config struct {
	Name, ModelID, TokenEnv, Instruction, BaseURL string
	MaxTokens                                     int
	Timeout                                       time.Duration
	HTTPClient                                    *http.Client
}

// Agent is an Anchora agent backed by Eino's OpenAI-compatible ChatModel.
type Agent struct {
	model       *openai.ChatModel
	instruction string
}

// New creates an Eino chat model for an OpenAI-compatible endpoint.
func New(ctx context.Context, config Config) (*Agent, error) {
	if config.Name == "" || config.ModelID == "" {
		return nil, errors.New("Eino agent requires a name and model ID")
	}
	if config.TokenEnv == "" {
		config.TokenEnv = "HF_TOKEN"
	}
	token := os.Getenv(config.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("environment variable %q is not set", config.TokenEnv)
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	var maxTokens *int
	if config.MaxTokens > 0 {
		maxTokens = &config.MaxTokens
	}
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     token,
		BaseURL:    config.BaseURL,
		Model:      config.ModelID,
		MaxTokens:  maxTokens,
		Timeout:    config.Timeout,
		HTTPClient: config.HTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model: %w", err)
	}
	return &Agent{model: model, instruction: config.Instruction}, nil
}

// Run generates one response using Eino's standard chat-model interface.
func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
	if a == nil || a.model == nil {
		return "", errors.New("Eino agent is not initialized")
	}
	messages := make([]*schema.Message, 0, 2)
	if a.instruction != "" {
		messages = append(messages, schema.SystemMessage(a.instruction))
	}
	messages = append(messages, schema.UserMessage(prompt))
	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate with Eino chat model: %w", err)
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return "", errors.New("Eino chat model returned no text output")
	}
	return response.Content, nil
}
