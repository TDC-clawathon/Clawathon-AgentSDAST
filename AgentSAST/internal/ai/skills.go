package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// SASTModes are the supported scan modes, in canonical execution order.
// quickscan/deepscan are independent toggles; pgwscan is an optional add-on.
var SASTModes = []string{"quickscan", "deepscan", "pgwscan"}

// NormalizeModes resolves the requested modes into the skill dirs to load.
// Quick and Deep are mutually exclusive choices, but Deep INCLUDES all Quick
// skills — so deepscan expands to [quickscan, deepscan]. quickscan (or anything
// without deepscan) yields [quickscan]. pgwscan is an independent add-on. The
// effective base is always present so core deliverables are produced.
func NormalizeModes(in []string) []string {
	want := map[string]bool{}
	for _, m := range in {
		want[strings.ToLower(strings.TrimSpace(m))] = true
	}
	var out []string
	if want["deepscan"] {
		out = []string{"quickscan", "deepscan"} // Deep includes Quick
	} else {
		out = []string{"quickscan"}
	}
	if want["pgwscan"] {
		out = append(out, "pgwscan")
	}
	return out
}

// skillStore holds the SAST skill text (SKILL.md bodies) and reference docs for
// the selected scan modes, loaded from skills/sast/<mode>/. It is the SAST
// analogue of AgentDAST's internal/knowledge package.
type skillStore struct {
	skill  string            // concatenated SKILL.md bodies (frontmatter stripped)
	refs   map[string]string // topic -> markdown
	topics []string
	modes  []string // effective modes loaded
}

// Modes returns the effective scan modes whose skills were loaded.
func (s *skillStore) Modes() []string { return s.modes }

// refAliases map human phrasings to a reference file base.
var refAliases = map[string]string{
	"bola":                              "idor",
	"broken_object_level_authorization": "idor",
	"insecure_direct_object_reference":  "idor",
	"sql":                               "sqli",
	"sql_injection":                     "sqli",
	"sqlinjection":                      "sqli",
	"cross_site_scripting":              "xss",
	"price_tampering":                   "payment",
	"payment_logic":                     "payment",
}

// LoadSkills loads the union of the selected scan modes' skills from baseDir
// (skills/sast). Each mode is a subdir with SKILL.md + references/*.md. Missing
// files yield an empty store rather than an error.
func LoadSkills(baseDir string, modes []string) *skillStore {
	st := &skillStore{refs: map[string]string{}}
	baseDir = strings.TrimSpace(baseDir)
	st.modes = NormalizeModes(modes)
	if baseDir == "" {
		return st
	}
	for _, mode := range st.modes {
		dir := filepath.Join(baseDir, mode)
		if b, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil {
			body := stripFrontmatter(string(b))
			if strings.TrimSpace(body) != "" {
				if st.skill != "" {
					st.skill += "\n\n"
				}
				st.skill += "===== SCAN MODE: " + mode + " =====\n" + body
			}
		}
		refDir := filepath.Join(dir, "references")
		entries, err := os.ReadDir(refDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			key := base
			if _, exists := st.refs[key]; exists {
				key = mode + "/" + base // disambiguate cross-mode collisions
			}
			if b, rerr := os.ReadFile(filepath.Join(refDir, e.Name())); rerr == nil {
				st.refs[key] = string(b)
				st.topics = append(st.topics, key)
			}
		}
	}
	sort.Strings(st.topics)
	return st
}

// skillText returns the concatenated SKILL.md bodies to inline into the system prompt.
func (s *skillStore) skillText() string { return s.skill }

func (s *skillStore) list() []string {
	out := make([]string, len(s.topics))
	copy(out, s.topics)
	return out
}

// get returns the reference document for a vulnerability topic (with aliases).
func (s *skillStore) get(topic string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(topic))
	n = strings.NewReplacer("-", "_", " ", "_", "/", "_").Replace(n)
	n = strings.TrimSuffix(n, ".md")
	if a, ok := refAliases[n]; ok {
		n = a
	}
	c, ok := s.refs[n]
	return c, ok
}

// stripFrontmatter removes a leading YAML frontmatter block (--- ... ---).
func stripFrontmatter(s string) string {
	t := strings.TrimLeft(s, " \t\r\n")
	if !strings.HasPrefix(t, "---") {
		return s
	}
	rest := t[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return s
	}
	after := rest[idx+4:]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		after = after[nl+1:]
	}
	return strings.TrimLeft(after, "\r\n")
}

// ResolveSastSkillsDir locates skills/sast/ (env SKILLS_SAST_DIR, then repo-root
// autodetect for local dev), mirroring AgentDAST's ResolveDastSkillsDir.
func ResolveSastSkillsDir() string {
	if p := strings.TrimSpace(os.Getenv("SKILLS_SAST_DIR")); p != "" {
		return p
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "skills", "sast"))
		if isDir(candidate) {
			return candidate
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			candidate := filepath.Join(dir, "skills", "sast")
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
