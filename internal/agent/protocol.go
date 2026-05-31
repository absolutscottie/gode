package agent

// Message represents a single message in a conversation.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallId string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type Delta struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Choices struct {
	FinishReason string `json:"finish_reason"`
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
}

type ChunkedResponse struct {
	Created           int64     `json:"created"`
	ID                string    `json:"id"`
	Model             string    `json:"model"`
	SystemFingerprint string    `json:"system_fingerprint"`
	Object            string    `json:"object"`
	Choices           []Choices `json:"choices"`
}

type ToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
	Result   string       `json:"-"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // This streams as a partial string fragment!
}

type Payload struct {
	Messages    []Message         `json:"messages"`
	Temperature float32           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Tools       []ToolDescription `json:"tools,omitempty"`
}
