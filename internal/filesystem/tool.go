package filesystem

import "gode/internal/agent"

type Tool interface {
	Execute(agent.ToolCall) (string, error)
	GetName() string
	GetDescription() agent.ToolDescription
	SetEnabled(bool)
	Enabled() bool
}
