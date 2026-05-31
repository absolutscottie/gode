package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gode/config/prompts"
	"gode/internal/agent"
	"gode/internal/agent/llamacpp"
	"gode/internal/filesystem"
	"gode/internal/toolerrors"
	"gode/internal/tui"

	tea "charm.land/bubbletea/v2"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	SystemRole string = "system"
	UserRole   string = "user"
	AgentRole  string = "assistant"
	ToolRole   string = "tool"
)

// should probably move this to a config
var host = "192.168.4.106:8080"
var modelName = "Qwen3.6-35B-A3B-UD-IQ3_XXS.gguf"

var llm llamacpp.LlamacppProvider
var messages []agent.Message
var availableTools []agent.ToolDescription

func init() {
	llm = llamacpp.NewProvider(host, modelName)
}

var logger zerolog.Logger

func main() {
	file, err := os.OpenFile(
		"app.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: file})
	log.Logger = log.Logger.With().Timestamp().Logger()
	logger = log.Logger

	userChan := make(chan any)
	ui := tea.NewProgram(tui.InitialModel(userChan))

	session := &llamacpp.Session{
		Messages: []agent.Message{
			{
				Role:    SystemRole,
				Content: prompts.CodingPrompt,
			},
		},
		ToolDescriptions: []agent.ToolDescription{
			filesystem.FileReadTool,
			filesystem.FileWriteTool,
		},
		ChunkFn:       func(s string) { handleChunk(ui, s) },
		FullMessageFn: func(s string) { handleFullMessage(ui, s) },
		ConfirmFn:     func(s string) { promptAndWait(ui, s) },
	}

	go userLoop(session, userChan)

	if _, err := ui.Run(); err != nil {
		log.Error().Msgf("Error starting program: %s", err)
	}
}

func userLoop(session *llamacpp.Session, userChan chan any) {
	var currentCtx context.Context
	var cancel context.CancelFunc

	for msg := range userChan {
		switch msg := msg.(type) {
		case string:
			session.StoreMessages(agent.Message{
				Role:    UserRole,
				Content: msg,
			})
		case *agent.ToolCall:
			session.StoreMessages(
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
		}

		currentCtx, cancel = context.WithCancel(context.Background())

		go func(ctx context.Context, cancel context.CancelFunc, userMsg any) {
			output, toolCall, err := llm.ChatStreamWithContext(ctx, session)
			if err != nil {
				if ctx.Err() != nil {
					logger.Info().Msgf("llm request cancelled: %s", err)
				} else {
					logger.Error().Msgf("llm error: %s", err)
				}
			}

			if output != "" {
				session.FullMessageFn(output)
				session.StoreMessages(agent.Message{
					Role:    AgentRole,
					Content: output,
				})
				return
			}

			if toolCall != nil {
				session.ConfirmFn(buildToolConfirmationPrompt(toolCall))
				content, err := handleToolCall(toolCall)
				if err != nil {
					toolCall.Result = err.Error()
				} else {
					toolCall.Result = content
				}

				userChan <- toolCall
			}

		}(currentCtx, cancel, msg)
	}
}

func handleChunk(ui *tea.Program, chunk string) {
	logger.Debug().Msgf("sending chunk: %s", chunk)
	ui.Send(tui.MessageChunk{Content: chunk})
}

func handleFullMessage(ui *tea.Program, fullMessage string) {
	logger.Info().Msgf("sending full message: %s", fullMessage)
	ui.Send(tui.MessageFull{Content: fullMessage})
}

func handleToolCall(t *agent.ToolCall) (string, *toolerrors.ToolError) {
	switch t.Function.Name {
	case "file_read":
		var args filesystem.FileReadArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", toolerrors.New(500, err.Error())
		}
		result := filesystem.FileRead(args)
		if !result.Success {
			logger.Error().Msgf("file read error: %s", result.Error)
		}
		return result.Content, nil
	case "file_write":
		var args filesystem.FileWriteArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return "", toolerrors.New(500, err.Error())
		}
		result := filesystem.FileWrite(args)
		if !result.Success {
			logger.Error().Msgf("file write error: %s", result.Error)
		}
		return result.Error, nil
	default:
		return "", toolerrors.New(404, "tool not found.")
	}
}

func promptAndWait(ui *tea.Program, userPrompt string) bool {
	cr := tui.ConfirmationRequest{
		Question:   userPrompt,
		ResultChan: make(chan bool),
	}

	ui.Send(cr)
	answer := <-cr.ResultChan
	close(cr.ResultChan)
	return answer
}

func buildToolConfirmationPrompt(t *agent.ToolCall) string {
	switch t.Function.Name {
	case "file_read":
		var args filesystem.FileReadArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return ""
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **read** `%s`?", args.Path)
	case "file_write":
		var args filesystem.FileWriteArgs
		err := json.Unmarshal([]byte(t.Function.Arguments), &args)
		if err != nil {
			logger.Error().Msgf("failed to parse arguments: %s", err)
			return ""
		}

		return fmt.Sprintf("Do you want to allow Cosmo to **write** `%s`?", args.Path)
	}

	return "How much wood could a wood chuck chuck if a wood chuck could chuck wood?"
}
