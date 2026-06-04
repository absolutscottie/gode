package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileEdit_SuccessSingleByteRange(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("hello world\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 6, EndOffset: 11, NewText: "moon"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "hello moon\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessMultiByteReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("abc\ndef\nghi\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Replace bytes 0-4 ("abc\n") with "xyz\n"
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 4, NewText: "xyz\n"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "xyz\ndef\nghi\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessMultipleEdits(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("one\ntwo\nthree\nfour\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// "one\ntwo\nthree\nfour\n"
	// byte 0-3: "one\n" -> replace with "ONE\n"
	// byte 8-12: "three\n" -> replace with "THREE\n"
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 4, NewText: "ONE\n"},
			{StartOffset: 8, EndOffset: 14, NewText: "THREE\n"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "ONE\ntwo\nTHREE\nfour\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessReplaceAllContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("hello\nworld\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 12, NewText: "replaced"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "replaced"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessAppendByReplacingEnd(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("line1\nline2\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Replace the trailing newline with "\nline3\n"
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 11, EndOffset: 12, NewText: "\nline3\n"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "line1\nline2\nline3\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessUTF8MultiByte(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	// "héllo" — 'é' is 2 bytes in UTF-8
	err := os.WriteFile(filePath, []byte("héllo\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Replace bytes 1-3 (the 'é' character) with 'e'
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 1, EndOffset: 3, NewText: "e"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "hello\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_InvalidOffsets(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("abc\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name    string
		edit    Edit
		wantErr string
	}{
		{
			name:    "negative start offset",
			edit:    Edit{StartOffset: -1, EndOffset: 2, NewText: "x"},
			wantErr: "Invalid byte offsets in edit",
		},
		{
			name:    "end offset less than start offset",
			edit:    Edit{StartOffset: 3, EndOffset: 1, NewText: "x"},
			wantErr: "Invalid byte offsets in edit",
		},
		{
			name:    "end offset exceeds file length",
			edit:    Edit{StartOffset: 0, EndOffset: 100, NewText: "x"},
			wantErr: "End offset exceeds file length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FileEdit(FileEditArgs{
				Path:  filePath,
				Edits: []Edit{tc.edit},
			})
			if result.Success {
				t.Fatalf("expected failure, got success")
			}
			if result.Error != tc.wantErr {
				t.Fatalf("got error %q, want %q", result.Error, tc.wantErr)
			}
		})
	}
}

func TestFileEdit_FileNotFound(t *testing.T) {
	result := FileEdit(FileEditArgs{
		Path: "/nonexistent/path/file.txt",
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 1, NewText: "x"},
		},
	})

	if result.Success {
		t.Fatal("expected failure for nonexistent file, got success")
	}
	if result.Error == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestFileEdit_EmptyEdits(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("a\nb\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	result := FileEdit(FileEditArgs{
		Path:  filePath,
		Edits: []Edit{},
	})

	if !result.Success {
		t.Fatalf("expected success with empty edits, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "a\nb\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_EdgeCaseSingleByteFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("x"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 1, NewText: "y"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "y"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_EdgeCaseEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Insert "hello" at offset 0 (start == end == 0 is valid for insertion)
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 0, NewText: "hello"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "hello"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessInsertAtBeginning(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("world\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Insert "hello " at offset 0
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 0, EndOffset: 0, NewText: "hello "},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "hello world\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}

func TestFileEdit_SuccessInsertAtEnd(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("hello\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Insert " world" at offset 6 (end of file)
	result := FileEdit(FileEditArgs{
		Path: filePath,
		Edits: []Edit{
			{StartOffset: 6, EndOffset: 6, NewText: " world"},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	content, _ := os.ReadFile(filePath)
	want := "hello\n world"
	if string(content) != want {
		t.Fatalf("got %q, want %q", string(content), want)
	}
}
