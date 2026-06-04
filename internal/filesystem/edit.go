package filesystem

import (
	"encoding/json"
	"fmt"
	"gode/internal/agent"
	"os"
	"path/filepath"
)

var FileEditToolDescription = agent.ToolDescription{
	Type: "function",
	Function: agent.Function{
		Name:        "file_edit",
		Description: "Edits a file by replacing specific byte ranges. This is safer and more efficient than file_write for modifications.",
		Params: agent.FunctionParam{
			Type: "object",
			Properties: map[string]agent.FunctionParamProperty{
				"path": {
					Type:        "string",
					Description: "The absolute path of the file to edit (e.g. `/home/scottie/file.txt`)",
				},
				"edits": {
					Type:        "array",
					Description: "A list of edits to apply to the file. Each edit specifies a byte range to replace.",
					Items: &agent.FunctionParamProperty{
						Type: "object",
						Properties: map[string]agent.FunctionParamProperty{
							"start_offset": {
								Type:        "integer",
								Description: "The 0-indexed byte offset where the edit starts.",
							},
							"end_offset": {
								Type:        "integer",
								Description: "The 0-indexed byte offset where the edit ends (exclusive).",
							},
							"new_text": {
								Type:        "string",
								Description: "The text to replace the specified byte range with.",
							},
						},
					},
					Required: []string{
						"start_offset",
						"end_offset",
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
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	NewText     string `json:"new_text"`
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

	// Read the file content as bytes
	content, err := os.ReadFile(absPath)
	if err != nil {
		return FileEditResult{Success: false, Error: err.Error()}
	}

	// Apply edits in reverse order to preserve byte offsets
	for i := len(args.Edits) - 1; i >= 0; i-- {
		edit := args.Edits[i]

		// Validate byte offsets: start must be >= 0, end must be > start (zero-length range allowed for insertion)
		if edit.StartOffset < 0 || edit.EndOffset < edit.StartOffset {
			return FileEditResult{Success: false, Error: "Invalid byte offsets in edit"}
		}

		if edit.EndOffset > len(content) {
			return FileEditResult{Success: false, Error: "End offset exceeds file length"}
		}

		// Replace the byte range: [startOffset, endOffset)
		newContent := append(content[:edit.StartOffset], append([]byte(edit.NewText), content[edit.EndOffset:]...)...)
		content = newContent
	}

	// Write the modified content back
	err = os.WriteFile(absPath, content, 0644)
	if err != nil {
		return FileEditResult{Success: false, Error: err.Error()}
	}

	return FileEditResult{Success: true}
}

type FileEditTool struct {
	enabled bool
}

func (tool *FileEditTool) Execute(call string) (string, error) {
	var args FileEditArgs
	err := json.Unmarshal([]byte(call), &args)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %s", err)
	}
	result := FileEdit(args)
	if !result.Success {
		return "", fmt.Errorf("file read error: %s", result.Error)
	}
	return "", nil
}

func (t *FileEditTool) GetName() string {
	return FileEditToolDescription.Function.Name
}

func (t *FileEditTool) Enabled() bool {
	return t.enabled
}

func (t *FileEditTool) SetEnabled(enabled bool) {
	t.enabled = enabled
}

func (t *FileEditTool) GetDescription() agent.ToolDescription {
	return FileEditToolDescription
}
