package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"gode/internal/agent"
)

var ShellExecToolDescription = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "shell_exec",
		Description: "Executes a shell command and returns the output. Requires user approval before execution.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
				"timeout": {
					Type:        "integer",
					Description: "Maximum execution time in seconds (default: 30)",
				},
			},
		},
		Required: []string{
			"command",
		},
		AdditionalProperties: false,
	},
}

type ShellExecArgs struct {
	Command string `json:"command"`
	Timeout *int   `json:"timeout,omitempty"`
}

type ShellExecResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

func (r ShellExecResult) String() (string, error) {
	encoded, err := json.Marshal(r)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func ShellExec(args ShellExecArgs) ShellExecResult {
	timeout := 30
	if args.Timeout != nil {
		timeout = *args.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	exitCode := 0

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return ShellExecResult{
				Success:  false,
				Error:    fmt.Sprintf("command timed out after %d seconds", timeout),
				ExitCode: -1,
			}
		} else {
			return ShellExecResult{
				Success: false,
				Error:   err.Error(),
			}
		}
	}

	return ShellExecResult{
		Success:  true,
		Output:   string(output),
		ExitCode: exitCode,
	}
}

type ShellExecTool struct {
	enabled bool
}

func (tool *ShellExecTool) Execute(call string) (string, error) {
	var args ShellExecArgs
	err := json.Unmarshal([]byte(call), &args)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %s", err)
	}
	result := ShellExec(args)
	if !result.Success {
		return "", fmt.Errorf("shell exec error: %s", result.Error)
	}
	return result.String()
}

func (t *ShellExecTool) GetName() string {
	return ShellExecToolDescription.Function.Name
}

func (t *ShellExecTool) Enabled() bool {
	return t.enabled
}

func (t *ShellExecTool) SetEnabled(enabled bool) {
	t.enabled = enabled
}

func (t *ShellExecTool) GetDescription() agent.ToolDescription {
	return ShellExecToolDescription
}
