package shell

import (
	"fmt"
	"os/exec"
)

type ShellExecArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type ShellExecResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

func ShellExec(args ShellExecArgs) ShellExecResult {
	// TODO
	// 1. verify the command is allowed
	// 2. verify any files in the args are allowed

	cmd := exec.Command(args.Command, args.Args...)
	err := cmd.Run()
	if err != nil {
		return ShellExecResult{
			Success: false,
			Error:   fmt.Sprintf("shell error: %s", err.Error()),
		}
	}

	return ShellExecResult{
		Success: false,
		Error:   fmt.Sprintf("tool is unavailable"),
	}
}
