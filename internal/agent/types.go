package agent

type ToolDescription struct {
	Type     string   `json:"type"`
	Function Function `json:"function,omitempty"`
}

type Function struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Params      FunctionParam `json:"parameters"`
	Required    []string      `json:"required"`
}

type FunctionParam struct {
	Type       string                           `json:"type"`
	Properties map[string]FunctionParamProperty `json:"properties"`
}

type FunctionParamProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}
