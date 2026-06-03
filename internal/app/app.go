package app

import (
	"context"
	"encoding/json"
	"fmt"

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

// App orchestrates the LLM session and the TUI.
type App struct {
	llm      llamacpp.LlamacppProvider
	session  *llamacpp.Session
	userChan chan any
	ui       *tea.Program
	logger   zerolog.Logger
	fileList []string
}

// New creates a new App with the given configuration.
func New(host, modelName string, fileList []string, logger zerolog.Logger) *App {
	llm := llamacpp.NewProvider(host, modelName)

	return &App{
		llm:      llm,
		userChan: make(chan any),
		logger:   logger,
		fileList: fileList,
	}
}

// Run starts the TUI and begins processing user input.
func (a *App) Run() (tea.Model, error) {
	a.ui = tea.NewProgram(tui.InitialModel(a.userChan))

	// Wire up session callbacks now that we have the TUI program.
	a.session = &llamacpp.Session{
		Messages: []agent.Message{
			{
				Role:    SystemRole,
				Content: prompts.BuildPrompt(a.fileList),
			},
		},
		ToolDescriptions: []agent.ToolDescription{
			filesystem.FileReadTool,
			filesystem.FileWriteTool,
			filesystem.FileEditTool,
			filesystem.FileInfoTool,
		},
		ChunkFn:       func(s string) { a.sendChunk(s) },
		FullMessageFn: func(s string) { a.sendFullMessage(s) },
		ConfirmFn:     func(s string) bool { return a.promptAndWait(s) },
		StartFn:       func() { a.sendAgentStart() },
		StopFn:        func() { a.sendAgentStop() },
	}

	go a.userLoop()
	return a.ui.Run()
}

func (a *App) userLoop() {
	var cancel context.CancelFunc

	for msg := range a.userChan {
		// Cancel the previous goroutine's context if it exists
		if cancel != nil {
			cancel()
		}

		switch msg := msg.(type) {
		case string:
			a.session.StoreMessages(agent.Message{
				Role:    UserRole,
				Content: msg,
			})
		case *agent.ToolCall:
			a.session.StoreMessages(
				agent.Message{
					Role:    AgentRole,
					Content: "",
					ToolCalls: []agent.ToolCall{
						*msg,
					},
				},
				agent.Message{
					Role:       ToolRole,
					Content:    msg.Result,
					ToolCallId: msg.ID,
					Name:       msg.Function.Name,
				},
			)
		case tui.StopGeneration:
			a.logger.Info().Msg("user requested stop generation")
			a.ui.Send(tui.GenerationStopped{})
			continue
		}

		currentCtx, cancel := context.WithCancel(context.Background())

		go func(ctx context.Context, cancel context.CancelFunc, userMsg any) {
			a.session.StartFn()
			output, toolCall, err := a.llm.ChatStreamWithContext(ctx, a.session)
			if err != nil {
				if ctx.Err() != nil {
					a.logger.Info().Msgf("llm request cancelled: %s", err)
				} else {
					a.logger.Error().Msgf("llm error: %s", err)
				}
			}
			a.session.StopFn()

			if output != "" {
				a.session.FullMessageFn(output)
				a.session.StoreMessages(agent.Message{
					Role:    AgentRole,
					Content: output,
				})
				return
			}

			if toolCall != nil {
				prompt, err := buildToolConfirmationPrompt(a.logger, toolCall)
				if err != nil {
					toolCall.Result = err.Error()
				}
				answer := a.session.ConfirmFn(prompt)
				if !answer {
					toolCall.Result = fmt.Errorf("user denied tool request").Error()
				} else {
					result, err := handleToolCall(a.logger, toolCall)
					if err != nil {
						toolCall.Result = err.Error()
					} else {
						toolCall.Result = result
					}
				}

				a.userChan <- toolCall
			}

		}(currentCtx, cancel, msg)
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

func handleToolCall(logger zerolog.Logger, t *agent.ToolCall) (string, error) {
	switch t.Function.Name {
	case "file_read":
		var args filesystem.FileReadArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", fmt.Errorf("failed to parse arguments: %s", err)
		}
		result := filesystem.FileRead(args)
		if !result.Success {
			logger.Error().Msgf("file read error: %s", result.Error)
			return "", fmt.Errorf("file read error: %s", result.Error)
		}
		return result.Content, nil
	case "file_write":
		var args filesystem.FileWriteArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", fmt.Errorf("failed to parse arguments: %s", err)
		}
		result := filesystem.FileWrite(args)
		if !result.Success {
			logger.Error().Msgf("file write error: %s", result.Error)
			return "", fmt.Errorf("file write error: %s", result.Error)
		}
		return "", nil
	case "file_edit":
		var args filesystem.FileEditArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", fmt.Errorf("failed to parse arguments: %s", err)
		}
		result := filesystem.FileEdit(args)
		if !result.Success {
			logger.Error().Msgf("file edit error: %s", result.Error)
			return "", fmt.Errorf("file edit error: %s", result.Error)
		}
		return "", nil
	case "file_info":
		var args filesystem.FileInfoArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", fmt.Errorf("failed to parse arguments: %s", err)
		}
		result := filesystem.FileInfo(args)
		if !result.Success {
			logger.Error().Msgf("file info error: %s", result.Error)
			return "", fmt.Errorf("file info error: %s", result.Error)
		}
		return result.String()
	default:
		return "", fmt.Errorf("tool not found: %s", t.Function.Name)
	}
}

func buildToolConfirmationPrompt(logger zerolog.Logger, t *agent.ToolCall) (string, error) {
	switch t.Function.Name {
	case "file_read":
		var args filesystem.FileReadArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", err
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **read** `%s`?", args.Path), nil
	case "file_write":
		var args filesystem.FileWriteArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", err
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **write** `%s`?", args.Path), nil

	case "file_edit":
		var args filesystem.FileEditArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", err
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **edit** `%s`?", args.Path), nil

	case "file_info":
		var args filesystem.FileInfoArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", err
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **get info** on `%s`?", args.Path), nil
	}

	return "How much wood could a wood chuck chuck if a wood chuck could chuck wood?", fmt.Errorf("unknown tool request: " + t.Function.Name)
}
