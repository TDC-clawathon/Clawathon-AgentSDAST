package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolve maps a model-supplied relative path to an absolute path strictly
// inside the executor Root, rejecting absolute paths and traversal (`..`) that
// would escape the sandbox.
func (e *Executor) resolve(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}
	abs := filepath.Clean(filepath.Join(e.Root, rel))
	if !withinRoot(e.Root, abs) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return abs, nil
}

// withinRoot reports whether abs is root itself or nested under it.
func withinRoot(root, abs string) bool {
	if abs == root {
		return true
	}
	return strings.HasPrefix(abs, root+string(os.PathSeparator))
}

// safeJoin is the extraction-time guard against zip-slip / tar traversal: it
// joins an archive entry name to dest and verifies the result stays under dest.
func safeJoin(dest, name string) (string, error) {
	name = filepath.FromSlash(name)
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute entry not allowed: %s", name)
	}
	target := filepath.Clean(filepath.Join(dest, name))
	if !withinRoot(dest, target) {
		return "", fmt.Errorf("entry escapes destination: %s", name)
	}
	return target, nil
}
