package toolerrors

import "encoding/json"

type ToolError struct {
	Status  string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(code int, message string) *ToolError {
	return &ToolError{
		Status:  "error",
		Code:    code,
		Message: message,
	}
}

func (t *ToolError) Error() string {
	encoded, _ := json.Marshal(t)
	return string(encoded)
}
