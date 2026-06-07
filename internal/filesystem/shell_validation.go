package filesystem

import (
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
)

//go:embed shell_policy.toml
var configFS embed.FS

// Policy represents the shell execution and path validation policy.
type Policy struct {
	Shell ShellPolicy `toml:"shell"`
	Paths PathPolicy  `toml:"paths"`
}

// ShellPolicy contains shell command validation rules.
type ShellPolicy struct {
	AllowedCommands  []string  `toml:"allowed_commands"`
	BlockedPaths     []string  `toml:"blocked_paths"`
	BlockedPatterns  []Pattern `toml:"blocked_patterns"`
	compiledPatterns []*regexp.Regexp
}

// PathPolicy contains path validation rules.
type PathPolicy struct {
	Blocked          []string `toml:"blocked"`
	compiledPatterns []*regexp.Regexp
}

// Pattern represents a single regex pattern with optional metadata.
type Pattern struct {
	Pattern     string `toml:"pattern"`
	Description string `toml:"description,omitempty"`
}

// InitPolicy loads the policy from the embedded config file.
func InitPolicy(filename string) (*Policy, error) {
	p := &Policy{}
	data, err := configFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	if _, err := toml.Decode(string(data), p); err != nil {
		return nil, fmt.Errorf("failed to decode policy file: %w", err)
	}

	// Compile blocked patterns for shell policy
	for i := range p.Shell.BlockedPatterns {
		re, err := regexp.Compile(p.Shell.BlockedPatterns[i].Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile shell blocked pattern %d: %w", i, err)
		}
		p.Shell.compiledPatterns = append(p.Shell.compiledPatterns, re)
	}

	// Compile blocked patterns for path policy
	for i := range p.Paths.Blocked {
		// Escape special regex characters in path strings
		escaped := regexp.QuoteMeta(p.Paths.Blocked[i])
		re, err := regexp.Compile(escaped)
		if err != nil {
			return nil, fmt.Errorf("failed to compile path blocked pattern %d: %w", i, err)
		}
		p.Paths.compiledPatterns = append(p.Paths.compiledPatterns, re)
	}

	return p, nil
}

// ValidateShellCommand checks if a shell command is allowed.
func (p *Policy) ValidateShellCommand(cmd string) error {
	// Parse the command to extract the command name
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	commandName := parts[0]

	// Check if command is in the whitelist
	allowed := false
	for _, allowedCmd := range p.Shell.AllowedCommands {
		if allowedCmd == commandName {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("command '%s' is not in the allowed list", commandName)
	}

	// Check if any arguments contain blocked paths
	args := strings.Join(parts[1:], " ")
	for _, blockedPath := range p.Shell.BlockedPaths {
		log.Logger.Debug().Msgf("checking if args:%s uses blocked path: %s", args, blockedPath)
		if strings.Contains(args, blockedPath) {
			return fmt.Errorf("command contains blocked path '%s'", blockedPath)
		}
	}

	// Check blocked patterns
	for _, pattern := range p.Shell.compiledPatterns {
		if pattern.MatchString(cmd) {
			return fmt.Errorf("command blocked: matches dangerous pattern '%s'", pattern.String())
		}
	}

	return nil
}

// ValidatePath checks if a file path is allowed.
func (p *Policy) ValidatePath(path string) error {
	// Resolve the path to an absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check against blocked paths
	for _, blockedPath := range p.Paths.compiledPatterns {
		log.Logger.Debug().Msgf("checking if absolute path: %s matches blocked path: %s", absPath, blockedPath)
		if blockedPath.MatchString(absPath) {
			return fmt.Errorf("path '%s' is blocked", absPath)
		}
	}

	return nil
}
