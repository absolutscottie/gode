package app

import (
	"context"
	"fmt"
	"strings"

	"gode/config/prompts"
	"gode/internal/agent"
	"gode/internal/agent/llamacpp"
	"gode/internal/filesystem"
	"gode/internal/tui"

	tea "charm.land/bubbletea/v2"

	"github.com/rs/zerolog"
)

const (
	SystemRole string = "system"
	UserRole   string = "user"
	AgentRole  string = "assistant"
	ToolRole   string = "tool"
)

// toolRegistry maps tool names to their implementations.
var toolRegistry = map[string]filesystem.Tool{
	"file_read":  &filesystem.FileReadTool{},
	"file_write": &filesystem.FileWriteTool{},
}

// App orchestrates the LLM session and the TUI.
type App struct {
	llm               llamacpp.LlamacppProvider
	session           *llamacpp.Session
	userChan          chan any
	cancelChan        chan any
	ui                *tea.Program
	logger            zerolog.Logger
	fileList          []string
	currentCancelFunc context.CancelFunc
}

// New creates a new App with the given configuration.
func New(host, modelName string, fileList []string, logger zerolog.Logger) *App {
	llm := llamacpp.NewProvider(host, modelName)

	return &App{
		llm:        llm,
		userChan:   make(chan any, 2),
		cancelChan: make(chan any),
		logger:     logger,
		fileList:   fileList,
	}
}

// Run starts the TUI and begins processing user input.
func (a *App) Run() (tea.Model, error) {
	a.ui = tea.NewProgram(tui.InitialModel(a.userChan, a.cancelChan))

	// Wire up session callbacks now that we have the TUI program.
	a.session = &llamacpp.Session{
		Messages: []agent.Message{
			{
				Role:    SystemRole,
				Content: prompts.BuildPrompt(a.fileList),
			},
		},
		ToolDescriptions: []agent.ToolDescription{
			filesystem.FileWriteToolDescription,
			filesystem.FileReadToolDescription,
		},
		ChunkFn:       func(s string) { a.sendChunk(s) },
		FullMessageFn: func(s string) { a.sendFullMessage(s) },
		ConfirmFn:     func(s string) bool { return a.promptAndWait(s) },
		StartFn:       func() { a.sendAgentStart() },
		StopFn:        func() { a.sendAgentStop() },
	}

	go a.userLoop()
	go a.cancelLoop()
	return a.ui.Run()
}

func (a *App) cancelLoop() {
	for range a.cancelChan {
		if a.currentCancelFunc != nil {
			a.currentCancelFunc()
		}
	}
}

// resolveFileWords replaces words in the message with their full file paths
// from the fileList if the word is a substring of any file path.
// Only words that look like file path components (contain '.' or '/') are matched.
func (a *App) resolveFileWords(msg string) string {
	words := strings.Fields(msg)
	for i, word := range words {
		// Only attempt to match if the word looks like a file path component
		if !strings.ContainsAny(word, "./") {
			continue
		}
		for _, file := range a.fileList {
			if strings.Contains(file, word) {
				words[i] = file
				break
			}
		}
	}
	return strings.Join(words, " ")
}

func (a *App) userLoop() {
	for msg := range a.userChan {
		switch msg := msg.(type) {
		case string:
			msg = a.resolveFileWords(msg)
			a.session.StoreMessages(agent.Message{
				Role:    UserRole,
				Content: msg,
			})
		}

		var cancelCtx context.Context
		cancelCtx, a.currentCancelFunc = context.WithCancel(context.Background())

		a.session.StartFn()
		output, toolCall, err := a.llm.ChatStreamWithContext(cancelCtx, a.session)
		if err != nil {
			if cancelCtx.Err() != nil {
				a.logger.Info().Msgf("llm request cancelled: %s", err)
			} else {
				// edit history
				if len(a.session.Messages) >= 2 && a.session.Messages[len(a.session.Messages)-1].Role == ToolRole && a.session.Messages[len(a.session.Messages)-2].Role == AgentRole {
					am := a.session.Messages[len(a.session.Messages)-1]
					am.ToolCalls[0].Function.Arguments = "error parsing tool arguments"
				}

				a.userChan <- 0
				continue
			}

			a.logger.Error().Msgf("llm error: %s", err)
		}
		a.session.StopFn()

		if output != "" {
			a.session.FullMessageFn(output)
			a.session.StoreMessages(agent.Message{
				Role:    AgentRole,
				Content: output,
			})
		}

		if toolCall != nil {
			a.session.StoreMessages(
				agent.Message{
					Role:    AgentRole,
					Content: "",
					ToolCalls: []agent.ToolCall{
						*toolCall,
					},
				},
			)

			result, err := a.handleToolCall(toolCall)
			if err != nil {
				a.session.StoreMessages(
					agent.Message{
						Role:       ToolRole,
						Content:    err.Error(),
						ToolCallId: toolCall.ID,
						Name:       toolCall.Function.Name,
					},
				)
				continue
			}

			a.session.StoreMessages(
				agent.Message{
					Role:       ToolRole,
					Content:    result,
					ToolCallId: toolCall.ID,
					Name:       toolCall.Function.Name,
				},
			)

			a.userChan <- toolCall
		}

		a.currentCancelFunc = nil
	}
}

// sendChunk sends a chunk of streamed output to the TUI.
func (a *App) sendChunk(chunk string) {
	a.ui.Send(tui.MessageChunk{Content: chunk})
}

// sendFullMessage sends a complete agent message to the TUI.
func (a *App) sendFullMessage(fullMessage string) {
	a.logger.Info().Msgf("sending full message: %s", fullMessage)
	a.ui.Send(tui.MessageFull{Content: fullMessage})
}

// promptAndWait sends a confirmation request to the TUI and waits for the user's response.
func (a *App) promptAndWait(userPrompt string) bool {
	cr := tui.ConfirmationRequest{
		Question:   userPrompt,
		ResultChan: make(chan bool),
	}

	a.ui.Send(cr)
	answer := <-cr.ResultChan
	close(cr.ResultChan)
	return answer
}

// sendAgentStart notifies the TUI that the agent has started.
func (a *App) sendAgentStart() {
	a.ui.Send(tui.AgentStart{})
}

// sendAgentStop notifies the TUI that the agent has stopped.
func (a *App) sendAgentStop() {
	a.ui.Send(tui.AgentStop{})
}

func (a *App) handleToolCall(t *agent.ToolCall) (string, error) {
	tool, ok := toolRegistry[t.Function.Name]
	if !ok {
		return "", fmt.Errorf("tool: %s not found", t.Function.Name)
	}

	prompt, err := tool.Prompt(t.Function.Arguments)
	if err != nil {
		return "", err
	}

	answer := a.promptAndWait(prompt)
	if !answer {
		return "", fmt.Errorf("user rejected tool call for: %s", t.Function.Name)
	}

	return tool.Execute(t.Function.Arguments)
}
