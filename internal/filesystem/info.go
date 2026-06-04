package filesystem

import (
	"encoding/json"
	"fmt"
	"gode/internal/agent"
	"os"
	"path/filepath"
)

var FileInfoTool = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_info",
		Description: "Gets information about a file, including its size and line count.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file to get information about (e.g. `/home/scottie/file.txt`)",
				},
			},
		},
		Required: []string{
			"path",
		},
		AdditionalProperties: false,
	},
}

type FileInfoArgs struct {
	Path string `json:"path"`
}

type FileInfoResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	LineCount int    `json:"line_count"`
}

func (r FileInfoResult) String() (string, error) {
	encoded, err := json.Marshal(r)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func FileInfo(args FileInfoArgs) FileInfoResult {
	// Resolve to absolute path to prevent directory traversal attacks
	absPath, err := filepath.Abs(args.Path)
	if err != nil {
		return FileInfoResult{Success: false, Error: err.Error()}
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return FileInfoResult{Success: false, Error: err.Error()}
	}

	if info.IsDir() {
		return FileInfoResult{Success: false, Error: fmt.Errorf("read error: the provided path is not a file, it is a directory").Error()}
	}

	// Count lines
	content, err := os.ReadFile(absPath)
	if err != nil {
		return FileInfoResult{Success: false, Error: err.Error()}
	}

	lineCount := 0
	for _, line := range content {
		if line == '\n' {
			lineCount++
		}
	}
	// If the file doesn't end with a newline, we still count the last line
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lineCount++
	}

	return FileInfoResult{
		Success:   true,
		SizeBytes: info.Size(),
		LineCount: lineCount,
	}
}
