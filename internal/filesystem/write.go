package filesystem

import (
	"gode/internal/agent"
	"os"
	"path/filepath"
)

var FileWriteTool = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_write",
		Description: "Writes content to a file in the file system. Creates the file if it does not exist, or overwrites it if it does.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file to write to (e.g. `/home/scottie/file.txt`)",
				},
				"content": {
					Type:        "string",
					Description: "The content to write to the file",
				},
			},
		},
		Required: []string{
			"path",
			"content",
		},
	},
}

type FileWriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type FileWriteResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func FileWrite(args FileWriteArgs) FileWriteResult {
	// Resolve to absolute path to prevent directory traversal attacks
	absPath, err := filepath.Abs(args.Path)
	if err != nil {
		return FileWriteResult{Success: false, Error: err.Error()}
	}

	// Ensure the directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return FileWriteResult{Success: false, Error: err.Error()}
	}

	// Write the file
	err = os.WriteFile(absPath, []byte(args.Content), 0644)
	if err != nil {
		return FileWriteResult{Success: false, Error: err.Error()}
	}

	return FileWriteResult{Success: true}
}
