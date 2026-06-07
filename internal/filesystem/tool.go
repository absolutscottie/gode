package filesystem

import "gode/internal/agent"

type Tool interface {
	Execute(string) (string, error)
	Prompt(string) (string, error)
	GetName() string
	GetDescription() agent.ToolDescription
	SetEnabled(bool)
	Enabled() bool
	Validate(string) error
}

type InvalidArgumentsError struct {
	err error
}

func NewInvalidArgumentsError(base error) InvalidArgumentsError {
	return InvalidArgumentsError{
		err: base,
	}
}

func (err InvalidArgumentsError) Error() string {
	return err.Error()
}
