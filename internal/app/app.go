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

	log "github.com/rs/zerolog"
)

const (
	SystemRole string = "system"
	UserRole   string = "user"
	AgentRole  string = "assistant"
	ToolRole   string = "tool"
)

// Context compression thresholds
const (
	MaxContextSize    = 49152 // tokens
	CompressThreshold = 80    // compress at 80% (per mille)
	NPredictFraction  = 2457  // 5% of 49152
)

// toolRegistry maps tool names to their implementations.
var toolRegistry = map[string]filesystem.Tool{}

// App orchestrates the LLM session and the TUI.
type App struct {
	llm               llamacpp.LlamacppProvider
	session           *llamacpp.Session
	userChan          chan any
	cancelChan        chan any
	ui                *tea.Program
	logger            log.Logger
	fileList          []string
	currentCancelFunc context.CancelFunc
	firstUserMsgSent  bool
	tokensUsed        int
}

// New creates a new App with the given configuration.
func New(host, modelName string, fileList []string, logger log.Logger) *App {
	llm := llamacpp.NewProvider(host, modelName)

	// Load security policy
	policy, err := filesystem.InitPolicy("shell_policy.toml")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load security policy")
	}

	// Initialize tools with policy
	toolRegistry = map[string]filesystem.Tool{
		"file_read":  filesystem.NewFileReadTool(policy, &logger),
		"file_write": filesystem.NewFileWriteTool(policy),
		"shell_exec": filesystem.NewShellExecTool(policy),
	}

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
			filesystem.ShellExecToolDescription,
		},
		ChunkFn:       func(s string) { a.sendChunk(s) },
		FullMessageFn: func(s string) { a.sendFullMessage(s) },
		ConfirmFn:     func(s string) bool { return a.promptAndWait(s) },
		StartFn:       func() { a.sendAgentStart() },
		StopFn:        func() { a.sendAgentStop() },
		TokenUsageFn: func(u agent.Usage) {
			a.tokensUsed = u.PromptTokens
		},
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

func (a *App) userLoop() {
	for msg := range a.userChan {
		switch msg := msg.(type) {
		case string:
			a.session.StoreMessages(agent.Message{
				Role:    UserRole,
				Content: msg,
			})
		}

		// Generate a session title from the first user message only.
		if !a.firstUserMsgSent {
			a.firstUserMsgSent = true
			a.generateSessionTitle()
		}

		// Compress context if it exceeds the threshold
		if a.shouldCompressContext() {
			a.compressContext()
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
			a.logger.Debug().Msgf("storing tool call: with name: %s and arguments %s", toolCall.Function.Name, toolCall.Function.Arguments)

			result, err := a.handleToolCall(toolCall)
			if err != nil {
				a.logger.Debug().Msgf("handled tool call yielding result: %s", err.Error())
				toolCall.Function.Arguments = "{\"invalid\":true}"
				a.session.StoreMessages(
					agent.Message{
						Role:    AgentRole,
						Content: "",
						ToolCalls: []agent.ToolCall{
							*toolCall,
						},
					},
					agent.Message{
						Role:       ToolRole,
						Content:    err.Error(),
						ToolCallId: toolCall.ID,
						Name:       toolCall.Function.Name,
					},
				)
				a.sendToolCall(toolCall, "error")
				continue
			}

			a.logger.Debug().Msgf("handled tool call yielding result: %s", result)
			a.session.StoreMessages(
				agent.Message{
					Role: AgentRole,
					ToolCalls: []agent.ToolCall{
						*toolCall,
					},
				},
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

// shouldCompressContext returns true if the current session has used enough
// prompt tokens to warrant context compression.
func (a *App) shouldCompressContext() bool {
	if a.tokensUsed == 0 {
		return false
	}
	threshold := MaxContextSize * CompressThreshold / 100
	return a.tokensUsed > threshold
}

// compressContext sends the full conversation history to the LLM via the
// non-streaming endpoint and replaces the session messages with a compressed
// summary. The system prompt embeds all messages so the LLM treats them as
// a conversation history to summarize, not as messages to respond to.
func (a *App) compressContext() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build the conversation history string from all messages except the system prompt.
	var sb strings.Builder
	for i := 1; i < len(a.session.Messages); i++ {
		msg := a.session.Messages[i]
		role := strings.Title(strings.ToLower(msg.Role))
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}
	conversationHistory := sb.String()

	systemPrompt := fmt.Sprintf(`You are an advanced context-compression module for an LLM agent harness.
Your job is to compress the provided conversation history into a dense, technically accurate summary.

CRITICAL INSTRUCTIONS:
1. Do not engage in or reply to the user's questions in the log. Only summarize.
2. You should aim to reduce the overall size of the data by 80%%, but prioritize technical completeness over strict length limits.
3. Preserve all critical project constraints, chosen technologies, file paths, database schemas, and configuration values.
4. Keep track of the current state of the agent's tasks (e.g., "User asked for X, agent completed Y, currently debugging Z").
5. Do not lose specific error messages or exact function names discussed.

<conversation_history>
%s
</conversation_history>`, conversationHistory)

	a.logger.Info().Msg("compressing context history")

	summary, err := a.llm.Completion(ctx, "", systemPrompt,
		llamacpp.WithCachePrompt(false),
		llamacpp.WithNPredict(NPredictFraction),
		llamacpp.WithTemperature(0.2),
	)
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to compress context")
		return
	}

	if summary == "" {
		a.logger.Warn().Msg("compression returned empty summary, skipping")
		return
	}

	// Replace session messages: keep system prompt, add compressed summary, then the latest user/agent exchange.
	var compressed []agent.Message
	compressed = append(compressed, a.session.Messages[0]) // system prompt

	// Keep the last 2 messages (user + agent/tool exchange) uncompressed for immediate context.
	remaining := len(a.session.Messages) - 2
	if remaining < 0 {
		remaining = 0
	}

	// Add compressed summary
	compressed = append(compressed, agent.Message{
		Role:    AgentRole,
		Content: summary,
	})

	// Append the last 2 messages (last user message and last agent/tool response)
	if remaining > 0 && len(a.session.Messages) > 2 {
		compressed = append(compressed, a.session.Messages[remaining:]...)
	}

	a.session.Messages = compressed
	a.tokensUsed = 0
	a.logger.Info().Int("compressed_to", len(compressed)).Msg("context compressed")
}

// generateSessionTitle sends a non-cached completion request to summarize
// the first user message as a short session title.
func (a *App) generateSessionTitle() {
	// The first user message is the last stored message.
	if len(a.session.Messages) == 0 {
		a.logger.Warn().Msg("no messages available for session title generation")
		return
	}
	firstUserMsg := a.session.Messages[len(a.session.Messages)-1].Content
	if firstUserMsg == "" {
		a.logger.Warn().Msg("first user message is empty, skipping title generation")
		return
	}

	a.logger.Info().Str("user_message", firstUserMsg).Msg("generating session title")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	systemPrompt := "You are an expert AI assistant that specializes in summarizing coding conversations. Analyze the user's first message below and generate a short, descriptive title for the session. The title should be exactly 3 to 6 words long and focus on the core programming task, technology, or bug being addressed. Do not use quotation marks or introductory text"

	a.logger.Debug().Str("system prompt", systemPrompt).Str("user message", firstUserMsg).Msg("completion prompt")

	title, err := a.llm.Completion(ctx, firstUserMsg, systemPrompt, llamacpp.WithCachePrompt(false), llamacpp.WithNPredict(64))
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to generate session title")
		return
	}

	if title == "" {
		a.logger.Warn().Msg("session title is empty, using fallback")
		title = "New Session"
	}

	a.logger.Info().Str("title", title).Msg("session title generated")

	a.sendWindowTitleChanged(title)
}

// sendToolCallPending sends a pending tool call message to the TUI.
func (a *App) sendToolCall(tc *agent.ToolCall, status string) {
	a.ui.Send(tui.ToolCallMessage{
		ID:       tc.ID,
		ToolName: tc.Function.Name,
		Args:     tc.Function.Arguments,
		Status:   status,
	})
}

// sendChunk sends a chunk of streamed output to the TUI.
func (a *App) sendChunk(chunk string) {
	a.ui.Send(tui.MessageChunk{Content: chunk})
}

// sendFullMessage sends a complete agent message to the TUI.
func (a *App) sendFullMessage(fullMessage string) {
	a.logger.Debug().Msgf("sending full message to user: %s", fullMessage)
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

func (a *App) sendWindowTitleChanged(title string) {
	a.ui.Send(tui.WindowTitleChangedMessage{Title: title})
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

	// Pre-approval validation  hard block before showing to user
	if err := tool.Validate(t.Function.Arguments); err != nil {
		return "", err
	}

	answer := a.promptAndWait(prompt)
	if !answer {
		a.sendToolCall(t, "denied")
		return "", fmt.Errorf("user rejected tool call for: %s", t.Function.Name)
	}

	a.sendToolCall(t, "approved")

	return tool.Execute(t.Function.Arguments)
}
