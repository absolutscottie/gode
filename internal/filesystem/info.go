package filesystem

import (
	"encoding/json"
	"fmt"
	"gode/internal/agent"
	"os"
	"path/filepath"
)

var FileInfoToolDescription = agent.ToolDescription{
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

type FileInfoTool struct {
	enabled bool
}

func (tool *FileInfoTool) Execute(call string) (string, error) {
	var args FileInfoArgs
	err := json.Unmarshal([]byte(call), &args)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %s", err)
	}
	result := FileInfo(args)
	if !result.Success {
		return "", fmt.Errorf("file info error: %s", result.Error)
	}
	return result.String()
}

func (t *FileInfoTool) GetName() string {
	return FileInfoToolDescription.Function.Name
}

func (t *FileInfoTool) Enabled() bool {
	return t.enabled
}

func (t *FileInfoTool) SetEnabled(enabled bool) {
	t.enabled = enabled
}

func (t *FileInfoTool) GetDescription() agent.ToolDescription {
	return FileInfoToolDescription
}
