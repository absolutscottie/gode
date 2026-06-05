package app

import (
	"strings"
	"testing"
)

func TestResolveFileWords(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		fileList []string
		want     string
	}{
		{
			name:     "no matching files",
			msg:      "open file.txt and read it",
			fileList: []string{"src/main.go", "src/utils.go"},
			want:     "open file.txt and read it",
		},
		{
			name:     "exact match with file path",
			msg:      "read src/main.go",
			fileList: []string{"src/main.go", "src/utils.go"},
			want:     "read src/main.go",
		},
		{
			name:     "substring match with file path",
			msg:      "edit main.go",
			fileList: []string{"src/main.go", "src/utils.go"},
			want:     "edit src/main.go",
		},
		{
			name:     "multiple matches",
			msg:      "read main.go and utils.go",
			fileList: []string{"src/main.go", "src/utils.go"},
			want:     "read src/main.go and src/utils.go",
		},
		{
			name:     "word with dot but no match",
			msg:      "check version 1.0.0",
			fileList: []string{"src/main.go"},
			want:     "check version 1.0.0",
		},
		{
			name:     "word with slash but no match",
			msg:      "go to path/to/something",
			fileList: []string{"src/main.go"},
			want:     "go to path/to/something",
		},
		{
			name:     "empty message",
			msg:      "",
			fileList: []string{"src/main.go"},
			want:     "",
		},
		{
			name:     "empty file list",
			msg:      "read main.go",
			fileList: []string{},
			want:     "read main.go",
		},
		{
			name:     "word without dot or slash",
			msg:      "hello world",
			fileList: []string{"src/main.go"},
			want:     "hello world",
		},
		{
			name:     "multiple words, some match some don't",
			msg:      "open main.go and hello world",
			fileList: []string{"src/main.go"},
			want:     "open src/main.go and hello world",
		},
		{
			name:     "first match wins",
			msg:      "read main.go",
			fileList: []string{"src/main.go", "backup/main.go"},
			want:     "read src/main.go",
		},
		{
			name:     "word contains dot but is not a file path",
			msg:      "use foo.bar",
			fileList: []string{"src/main.go"},
			want:     "use foo.bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				fileList: tt.fileList,
			}
			got := a.resolveFileWords(tt.msg)
			if got != tt.want {
				t.Errorf("resolveFileWords() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveFileWords_EdgeCases(t *testing.T) {
	// Test that only the first matching file is used (break on first match)
	a := &App{
		fileList: []string{"a/b/c.go", "x/y/c.go"},
	}
	got := a.resolveFileWords("open c.go")
	want := "open a/b/c.go"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	// Test that a word containing both '.' and '/' is resolved
	a2 := &App{
		fileList: []string{"src/main.go"},
	}
	got2 := a2.resolveFileWords("open src/main.go")
	want2 := "open src/main.go"
	if got2 != want2 {
		t.Errorf("expected %q, got %q", want2, got2)
	}

	// Test that the original word is preserved if no match is found
	a3 := &App{
		fileList: []string{"src/main.go"},
	}
	got3 := a3.resolveFileWords("open unknown.go")
	want3 := "open unknown.go"
	if got3 != want3 {
		t.Errorf("expected %q, got %q", want3, got3)
	}

	// Verify that words without '.' or '/' are never matched
	a4 := &App{
		fileList: []string{"src/main.go"},
	}
	got4 := a4.resolveFileWords("main")
	want4 := "main"
	if got4 != want4 {
		t.Errorf("expected %q, got %q", want4, got4)
	}

	// Verify that the function handles multiple spaces correctly
	a5 := &App{
		fileList: []string{"src/main.go"},
	}
	got5 := a5.resolveFileWords("open  main.go")
	want5 := "open  src/main.go"
	if got5 != want5 {
		t.Errorf("expected %q, got %q", want5, got5)
	}
}

func TestResolveFileWords_Integration(t *testing.T) {
	// Simulate a realistic user message
	a := &App{
		fileList: []string{
			"internal/app/app.go",
			"internal/app/handler.go",
			"internal/filesystem/tool.go",
		},
	}

	msg := "please fix the bug in app.go and update handler.go"
	got := a.resolveFileWords(msg)
	want := "please fix the bug in internal/app/app.go and update internal/app/handler.go"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	// Verify that words like "the" or "in" are not matched even though they
	// appear in file paths
	if strings.Contains(got, "the") == false {
		t.Error("expected 'the' to be preserved in output")
	}
}
