package tui

import "time"

type MessageChunk struct {
	Content string
}

type MessageFull struct {
	Content string
}

type AgentStart struct{}

type AgentStop struct{}

type WindowTitleChangedMessage struct {
	Title string
}

type ConfirmationRequest struct {
	ResultChan chan bool
	Question   string
}

type TickMsg time.Time

type ChatMessage struct {
	ID      string
	Sender  string
	Content string
}

func NewChatMessage(sender, content string) ChatMessage {
	return ChatMessage{
		Sender:  sender,
		Content: content,
	}
}

type DecisionMessage struct {
	Approved bool
}

type TokenUsageMessage struct {
	PromptTokens              int
	CompletionTokens          int
	TotalTokens               int
	PromptCachedTokens        int
	CompletionReasoningTokens int
}

type StopGeneration struct{}

type GenerationStopped struct{}

type ToolCallMessage struct {
	ID       string
	ToolName string
	Args     string
	Status   string // "approved", "denied", "error"
}
