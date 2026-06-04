package filesystem

import "gode/internal/agent"

type Tool interface {
	Execute(agent.ToolCall) string
	GetName() string
	GetDescription() agent.ToolDescription
	SetEnabled(bool)
	Enabled() bool
}

var allTools = map[string]Tool{
	"file_edit": &FileEditTool{enabled: true},
}
