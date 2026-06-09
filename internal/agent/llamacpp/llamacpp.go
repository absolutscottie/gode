package llamacpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"gode/internal/agent"
)

type LlamacppProvider struct {
	host  string
	model string
}

type Session struct {
	Messages         []agent.Message
	ToolDescriptions []agent.ToolDescription
	ChunkFn          func(string)
	FullMessageFn    func(string)
	ConfirmFn        func(string) bool
	StartFn          func()
	StopFn           func()
	TokenUsageFn     func(agent.Usage)
	messageMutex     sync.Mutex
}

func (s *Session) StoreMessages(msg ...agent.Message) {
	s.messageMutex.Lock()
	defer s.messageMutex.Unlock()
	s.Messages = append(s.Messages, msg...)
}

const (
	dataPrefix = "data: "
	doneMarker = dataPrefix + "[DONE]"
)

func NewProvider(host, model string) LlamacppProvider {
	return LlamacppProvider{
		host:  host,
		model: model,
	}
}

func buildChatURL(host, model string) string {
	return fmt.Sprintf("http://%s/chat/completions", host)
}

func buildCompletionURL(host string) string {
	return fmt.Sprintf("http://%s/chat/completions", host)
}

// CompletionPayload maps to the native llama.cpp /completion endpoint.
type CompletionPayload struct {
	Model       string              `json:"model,omitempty"`
	Messages    []CompletionMessage `json:"messages,omitempty"`
	CachePrompt *bool               `json:"cache_prompt,omitempty"`
	NPredict    int                 `json:"n_predict,omitempty"`
	Temperature float32             `json:"temperature,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
}

// CompletionMessage is a message in a /completion request.
type CompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResult is the response from the native /completion endpoint.
type CompletionResult struct {
	Content         string `json:"content"`
	Stop            bool   `json:"stop"`
	GenerationTime  int64  `json:"generation_time"`
	TokensKept      int    `json:"tokens_kept"`
	TokensBaked     int    `json:"tokens_baked"`
	TokensGenerated int    `json:"tokens_generated"`
	TokensCached    int    `json:"tokens_cached"`
	TokensPrompt    int    `json:"tokens_prompt"`
}

// CompletionOption allows callers to configure a Completion request.
type CompletionOption func(*CompletionConfig)

type CompletionConfig struct {
	systemPrompt string
	cachePrompt  *bool
	nPredict     int
	temperature  float32
	stop         []string
}

// WithSystemPrompt sets the system prompt for the completion request.
func WithSystemPrompt(systemPrompt string) CompletionOption {
	return func(c *CompletionConfig) {
		c.systemPrompt = systemPrompt
	}
}

// WithCachePrompt enables or disables KV cache for the prompt prefix.
func WithCachePrompt(enabled bool) CompletionOption {
	return func(c *CompletionConfig) {
		c.cachePrompt = &enabled
	}
}

// WithNPredict sets the maximum number of tokens to generate.
func WithNPredict(n int) CompletionOption {
	return func(c *CompletionConfig) {
		c.nPredict = n
	}
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float32) CompletionOption {
	return func(c *CompletionConfig) {
		c.temperature = t
	}
}

// WithStop sets stop sequences that halt generation.
func WithStop(stop ...string) CompletionOption {
	return func(c *CompletionConfig) {
		c.stop = stop
	}
}

// Completion sends a single, non-streaming request to the /completion endpoint.
func (llama LlamacppProvider) Completion(ctx context.Context, userMessage, systemPrompt string, opts ...CompletionOption) (string, error) {
	cfg := &CompletionConfig{
		systemPrompt: systemPrompt,
		cachePrompt:  boolPtr(true),
		nPredict:     512,
		temperature:  0.4,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	messages := []CompletionMessage{
		{
			Role:    "system",
			Content: cfg.systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	payload := CompletionPayload{
		Model:       llama.model,
		Messages:    messages,
		CachePrompt: cfg.cachePrompt,
		//NPredict:    cfg.nPredict,
		Temperature: cfg.temperature,
		Stop:        cfg.stop,
	}

	jm, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal completion payload: %w", err)
	}

	log.Logger.Debug().Msgf("sending payload: %s", jm)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		buildCompletionURL(llama.host),
		bytes.NewReader(jm),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create completion request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute completion request: %w", err)
	}
	defer resp.Body.Close()

	var output []byte
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Err() != nil {
			if scanner.Err() == io.EOF {
				break
			}

			return "", scanner.Err()
		}
		buf := scanner.Bytes()
		output = append(output, buf...)
	}

	log.Logger.Debug().Msgf("received summary output: %s", string(output))

	var result agent.Response
	err = json.Unmarshal(output, &result)
	if err != nil {
		return "", fmt.Errorf("failed to decode completion response: %w", err)
	}

	return result.Choices[0].Message.Content, nil
}

func boolPtr(b bool) *bool {
	return &b
}

func (llama LlamacppProvider) ChatStreamWithContext(ctx context.Context, session *Session) (string, *agent.ToolCall, error) {
	payload := agent.Payload{
		Messages:    session.Messages,
		Tools:       session.ToolDescriptions,
		Temperature: 0.4,
		Stream:      true,
	}

	req, err := llama.buildRequest(ctx, payload)
	if err != nil {
		return "", nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}

	defer resp.Body.Close()
	fullMessage, toolCall, err := llama.scanResponse(resp.Body, session.ChunkFn, session.TokenUsageFn)

	return fullMessage, toolCall, nil
}

func (llama LlamacppProvider) buildRequest(ctx context.Context, payload agent.Payload) (*http.Request, error) {
	jm, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		buildChatURL(llama.host, llama.model),
		bytes.NewReader(jm))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (llama LlamacppProvider) scanResponse(body io.ReadCloser, chunkFn func(string), tokenUsageFn func(agent.Usage)) (string, *agent.ToolCall, error) {
	var toolCall *agent.ToolCall
	fullMessage := ""

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		err := scanner.Err()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}

		var prefix string
		if scanner.Text() == doneMarker {
			break
		} else if strings.HasPrefix(scanner.Text(), dataPrefix) {
			prefix = dataPrefix
		} else {
			continue
		}

		var ch agent.ChunkedResponse
		err = json.Unmarshal(scanner.Bytes()[len(prefix):], &ch)
		if err != nil {
			return "", nil, err
		}

		c := ch.Choices[0].Delta.Content
		t := ch.Choices[0].Delta.ToolCalls
		if c != "" {
			fullMessage += c
		}

		if t != nil {
			if toolCall == nil {
				toolCall = &t[0]
			} else {
				toolCall.Function.Arguments += t[0].Function.Arguments
			}
		}

		if c != "" {
			chunkFn(c)
		}

		if ch.Usage.TotalTokens > 0 && tokenUsageFn != nil {
			tokenUsageFn(ch.Usage)
		}
	}

	log.Logger.Info().Msgf("returning message: %s, tool call: %#v", fullMessage, toolCall)
	return fullMessage, toolCall, nil
}
