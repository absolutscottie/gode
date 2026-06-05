package filesystem

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gode/internal/agent"
)

var FileReadToolDescription = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_read",
		Description: "Reads the content of a file from the file system safely. Restricted to text files. Supports optional line offsets for efficient context usage. Callers should use the file_info tool before calling this tool to determine if it makes sense to use offsets. Files < 1 kilobyte should be read without offsets.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file in the file system (e.g. `/home/scottie/file.txt`)",
				},
				"start_line": {
					Type:        "integer",
					Description: "The 1-indexed line number to start reading from (optional).",
				},
				"end_line": {
					Type:        "integer",
					Description: "The 1-indexed line number to end reading at (inclusive, optional).",
				},
			},
		},
		Required: []string{
			"path",
		},
		AdditionalProperties: false,
	},
}

type FileReadArgs struct {
	Path      string `json:"path"`
	StartLine *int   `json:"start_line,omitempty"`
	EndLine   *int   `json:"end_line,omitempty"`
}

type FileReadResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func FileRead(args FileReadArgs) FileReadResult {
	// Resolve to absolute path to prevent directory traversal attacks
	absPath, err := filepath.Abs(args.Path)
	if err != nil {
		return FileReadResult{
			Success: false,
			Error:   fmt.Sprintf("invalid path: %s", err.Error()),
		}
	}

	file, err := os.Open(absPath)
	if err != nil {
		return FileReadResult{
			Success: false,
			Error:   fmt.Sprintf("read error: %s", err.Error()),
		}
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		// If start_line is specified, skip lines before it
		if args.StartLine != nil && lineNumber < *args.StartLine {
			continue
		}

		// If end_line is specified, stop after it
		if args.EndLine != nil && lineNumber > *args.EndLine {
			break
		}

		// If start_line is not specified, read from the beginning
		if args.StartLine == nil {
			content.WriteString(scanner.Text() + "\n")
		} else {
			content.WriteString(scanner.Text() + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return FileReadResult{
			Success: false,
			Error:   fmt.Sprintf("scan error: %s", err.Error()),
		}
	}

	return FileReadResult{
		Success: true,
		Content: content.String(),
	}
}

type FileReadTool struct {
	enabled bool
}

func (tool *FileReadTool) Prompt(call string) (string, error) {
	var args FileReadArgs
	err := json.Unmarshal([]byte(call), &args)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Do you want to allow Cosmo to **read** `%s`?", args.Path), nil
}

func (tool *FileReadTool) Execute(call string) (string, error) {
	var args FileReadArgs
	err := json.Unmarshal([]byte(call), &args)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %s", err)
	}
	result := FileRead(args)
	if !result.Success {
		return "", fmt.Errorf("file read error: %s", result.Error)
	}
	return result.Content, nil
}

func (t *FileReadTool) GetName() string {
	return FileReadToolDescription.Function.Name
}

func (t *FileReadTool) Enabled() bool {
	return t.enabled
}

func (t *FileReadTool) SetEnabled(enabled bool) {
	t.enabled = enabled
}

func (t *FileReadTool) GetDescription() agent.ToolDescription {
	return FileReadToolDescription
}
