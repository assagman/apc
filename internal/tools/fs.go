package tools

import (
	// "fmt"
	// "io"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FS struct {
	WD string
}

// ToolGetCurrentWorkingDirectory returns the current working directory(or so called project directory).
// It's typically use for when it's asked to do operations regarding local filesystem, file operations,
// statistics, etc.
func (fs *FS) ToolGetCurrentWorkingDirectory() (string, error) {
	if fs.WD != "" {
		return fs.WD, nil
	}
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
// caseSensitive: bool flag to determine if search is performed dace sensitive or not. rg option `-sis used when enabled
// dir: relative directory path to CWD, to perform ripgrep in `CWD/dir`.
func (fs *FS) ToolGrepText(text string, includeHiddenFiles bool, caseSensitive bool, dir string) (string, error) {
	var args []string

	// flags
	if includeHiddenFiles {
		args = append(args, "-.")
	}
	if caseSensitive {
		args = append(args, "-s")
	}

	if text == "" {
		return "", fmt.Errorf("Cannot perform rg for empty string")
	}
	if text == "*" {
		return "", fmt.Errorf("Cannot perform rg for everything")
	}
	args = append(args, text)

	// final arg is the directory to search
	cwd, cwdErr := fs.ToolGetCurrentWorkingDirectory()
	if cwdErr != nil {
		return "", fmt.Errorf("[ToolGrepText] Failed to get ToolGetCurrentWorkingDirectory output: %v", cwdErr)
	}
	dir = strings.TrimSpace(dir)
	dir = filepath.Clean(dir)
	if filepath.IsAbs(dir) {
		return "", fmt.Errorf("[ToolGrepText] dir must be relative to CWD.")
	}
	if dir == "" || dir == "." {
		dir = cwd
	}
	args = append(args, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rg", args...) // TODO: CommandContext with timeout

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// https://manpages.debian.org/unstable/ripgrep/rg.1.en.html#EXIT_STATUS
			if exitErr.ExitCode() == 1 {
				return "No match", nil
			}
			return "", err
		}
		return "", err
	}

	return string(output), nil
}

// ToolReadFile returns the entire contents of the file at the given path.
// The path is treated as relative to the current working directory.
//
// filePath: relative path to the CWD
func (fs *FS) ToolReadFile(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	filePath = filepath.Clean(filePath)
	if filepath.Base(filePath) == ".env" {
		return "", fmt.Errorf("Reading .env file is forbidden")
	}
	cwd, err := fs.ToolGetCurrentWorkingDirectory()
	if err != nil {
		return "", fmt.Errorf("[ToolReadFile] failed to get cwd: %w", err)
	}
	fullPath := filepath.Join(cwd, filePath)
	if !strings.HasPrefix(fullPath, cwd) {
		return "", fmt.Errorf("reading outside project directory is not allowed")
	}

	// TODO: check file size

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToolTree returns an ASCII tree representation of the directory tree
// rooted at dir (relative to CWD).
//
// dir: relative path to the CWD.
// maxDepth: positive integer value represents how many level of nested directories included in tree
func (fs *FS) ToolTree(dir string, maxDepth int) (string, error) {
	if maxDepth < 1 {
		return "", fmt.Errorf("[ToolTree] maxDepth must be a positive integer")
	}
	cwd, err := fs.ToolGetCurrentWorkingDirectory()
	if err != nil {
		return "", fmt.Errorf("[ToolTree] failed to get cwd: %w", err)
	}
	if filepath.IsAbs(dir) {
		return "", fmt.Errorf("[ToolTree] dir must be relative to CWD.")
	}
	root := filepath.Join(cwd, filepath.Clean(dir))

	var sb strings.Builder
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			rel = ""
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() && rel != "" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "" {
			sb.WriteString(d.Name() + "/\n")
			return nil
		}
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s/\n", indent, d.Name()))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", indent, d.Name()))
		}
		return nil
	})
	return sb.String(), err
}
