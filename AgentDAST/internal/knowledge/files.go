package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// fileSource loads markdown knowledge from a directory (repo root skills/dast/).
type fileSource struct {
	cache  map[string]string
	topics []string
}

// NewFileSource reads all *.md files from dir.
func NewFileSource(dir string) (*fileSource, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("knowledge directory is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		cache[base] = string(data)
	}
	if len(cache) == 0 {
		return nil, fmt.Errorf("no knowledge files found in %s", dir)
	}

	topics := make([]string, 0, len(cache))
	for topic := range cache {
		topics = append(topics, topic)
	}
	sort.Strings(topics)

	return &fileSource{cache: cache, topics: topics}, nil
}

func (f *fileSource) get(base string) (string, bool) {
	content, ok := f.cache[base]
	return content, ok
}

func (f *fileSource) list() []string {
	out := make([]string, len(f.topics))
	copy(out, f.topics)
	return out
}

// ResolveDastSkillsDir locates repo root skills/dast/ for CLI and local development.
func ResolveDastSkillsDir() string {
	if p := strings.TrimSpace(os.Getenv("SKILLS_DAST_DIR")); p != "" {
		return p
	}

	if _, file, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "skills", "dast"))
		if isDir(candidate) {
			return candidate
		}
	}

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			candidate := filepath.Join(dir, "skills", "dast")
			if isDir(candidate) {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Init loads markdown knowledge from dir. When dir is empty, ResolveDastSkillsDir is used.
// Returns an error when the directory is missing or has no *.md files.
func Init(dir string) error {
	if strings.TrimSpace(dir) == "" {
		dir = ResolveDastSkillsDir()
	}
	if dir == "" {
		return fmt.Errorf("skills directory not found; set SKILLS_DAST_DIR")
	}
	src, err := NewFileSource(dir)
	if err != nil {
		return err
	}
	currentSource = src
	return nil
}
