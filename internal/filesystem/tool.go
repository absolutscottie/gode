package filesystem

import "gode/internal/agent"

type Tool interface {
	Execute(string) (string, error)
	Prompt(string) (string, error)
	GetName() string
	GetDescription() agent.ToolDescription
	SetEnabled(bool)
	Enabled() bool
}
