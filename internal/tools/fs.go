package tools

import (
	// "fmt"
	// "io"
	"fmt"
	"os"
	"os/exec"
)

// ToolGetCurrentWorkingDirectory returns the current working directory(or so called project directory).
// It's typically use for when it's asked to do operations regarding local filesystem, file operations,
// statistics, etc.
func ToolGetCurrentWorkingDirectory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// ToolGrepText returns ripgrep output for given text in the given dir
//
// text: pattern to search
// includeHiddenFiles: bool flag to determine if hidden files are included or not. rg option `-.` is used when enabled
// caseSensitive: bool flag to determine if search is performed dace sensitive or not. rg option `-s` is used when enabled
// dir: full directory path to perform ripgrep in it
func ToolGrepText(text string, includeHiddenFiles bool, caseSensitive bool, dir string) (string, error) {
	var args []string
	if text == "" {
		return "", fmt.Errorf("Cannot perform rg for empty string")
	}
	if text == "*" {
		return "", fmt.Errorf("Cannot perform rg for everything")
	}

	args = append(args, text)
	// flags
	if includeHiddenFiles {
		args = append(args, "-.")
	}
	if caseSensitive {
		args = append(args, "-s")
	}
	// final arg is the directory to search
	if dir == "" {
		cwd, err := ToolGetCurrentWorkingDirectory()
		if err != nil {
			return "", err
		}
		dir = cwd
	}
	args = append(args, dir)
	cmd := exec.Command("rg", args...)

	stdout, stderr := cmd.CombinedOutput()
	if stderr != nil {
		if exitErr, ok := stderr.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "", nil // nothing matched
			}
			return "", stderr
		}
		return "", stderr
	}

	return string(stdout), nil
}
