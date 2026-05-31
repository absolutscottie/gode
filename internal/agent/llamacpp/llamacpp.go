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
	ConfirmFn        func(string)
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

func (llama LlamacppProvider) ChatStreamWithContext(ctx context.Context, session *Session) (string, *agent.ToolCall, error) {
	payload := agent.Payload{
		Messages:    session.Messages,
		Tools:       session.ToolDescriptions,
		Temperature: 0.2,
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
	fullMessage, toolCall, err := llama.scanResponse(resp.Body, session.ChunkFn)

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

func (llama LlamacppProvider) scanResponse(body io.ReadCloser, chunkFn func(string)) (string, *agent.ToolCall, error) {
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
	}

	log.Logger.Info().Msgf("returning message: %s, tool call: %#v", fullMessage, toolCall)
	return fullMessage, toolCall, nil
}
