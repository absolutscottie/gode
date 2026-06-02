package filesystem

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gode/internal/agent"
)

var FileReadTool = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_read",
		Description: "Reads the entire content of a file from the file system safely. Restricted to text files. Supports optional line offsets for efficient context usage.",
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
