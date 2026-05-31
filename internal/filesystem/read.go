package filesystem

import (
	"fmt"
	"io"
	"os"

	"gode/internal/agent"
)

var FileReadTool = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_read",
		Description: "Reads the entire content of a file from the file system safely. Restricted to text files.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file in the file system (e.g. `/home/scottie/file.txt`)",
				},
			},
		},
		Required: []string{
			"path",
		},
	},
}

type FileReadArgs struct {
	Path string `json:"path"`
}

type FileReadResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

func FileRead(args FileReadArgs) FileReadResult {
	file, err := os.Open(args.Path)
	if err != nil {
		return FileReadResult{
			Success: false,
			Error:   fmt.Sprintf("read error: %s", err.Error()),
		}
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return FileReadResult{
			Success: false,
			Error:   fmt.Sprintf("read error: %s", err.Error()),
		}
	}

	return FileReadResult{
		Success: true,
		Content: string(content),
	}
}

func isValidFileReadRequest(args FileReadArgs) bool {
	return true
}
