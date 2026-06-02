package filesystem

import (
	"gode/internal/agent"
	"os"
	"path/filepath"
	"strings"
)

var FileEditTool = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_edit",
		Description: "Edits a file by replacing specific lines. This is safer and more efficient than file_write for modifications.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file to edit (e.g. `/home/scottie/file.txt`)",
				},
				"edits": {
					Type:        "array",
					Description: "A list of edits to apply to the file. Each edit specifies a range of lines to replace.",
					Items: &agent.FunctionParamProperty{
						Type: "object",
						Properties: map[string]agent.FunctionParamProperty{
							"start_line": {
								Type:        "integer",
								Description: "The 1-indexed line number where the edit starts (inclusive).",
							},
							"end_line": {
								Type:        "integer",
								Description: "The 1-indexed line number where the edit ends (inclusive).",
							},
							"new_text": {
								Type:        "string",
								Description: "The text to replace the specified line range with.",
							},
						},
					},
					Required: []string{
						"start_line",
						"end_line",
						"new_text",
					},
					AdditionalProperties: false,
				},
			},
		},
		Required: []string{
			"path",
			"edits",
		},
	},
}

type FileEditArgs struct {
	Path  string `json:"path"`
	Edits []Edit `json:"edits"`
}

type Edit struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	NewText   string `json:"new_text"`
}

type FileEditResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func FileEdit(args FileEditArgs) FileEditResult {
	// Resolve to absolute path to prevent directory traversal attacks
	absPath, err := filepath.Abs(args.Path)
	if err != nil {
		return FileEditResult{Success: false, Error: err.Error()}
	}

	// Read the file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return FileEditResult{Success: false, Error: err.Error()}
	}

	lines := strings.Split(string(content), "\n")

	// Apply edits in reverse order to avoid shifting line numbers
	for i := len(args.Edits) - 1; i >= 0; i-- {
		edit := args.Edits[i]

		// Validate line numbers
		if edit.StartLine < 1 || edit.EndLine < 1 || edit.StartLine > edit.EndLine {
			return FileEditResult{Success: false, Error: "Invalid line numbers in edit"}
		}

		if edit.StartLine > len(lines) || edit.EndLine > len(lines) {
			return FileEditResult{Success: false, Error: "Line numbers exceed file length"}
		}

		// Convert 1-indexed to 0-indexed
		startIdx := edit.StartLine - 1
		endIdx := edit.EndLine - 1

		// Replace the range
		newLines := append(lines[:startIdx], strings.Split(edit.NewText, "\n")...)
		if endIdx+1 < len(lines) {
			newLines = append(newLines, lines[endIdx+1:]...)
		}

		lines = newLines
	}

	// Join lines back
	newContent := strings.Join(lines, "\n")

	// Write the file
	err = os.WriteFile(absPath, []byte(newContent), 0644)
	if err != nil {
		return FileEditResult{Success: false, Error: err.Error()}
	}

	return FileEditResult{Success: true}
}
