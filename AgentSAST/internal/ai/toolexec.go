package ai

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	maxExtractFiles = 20000
	maxExtractBytes = 1 << 30 // 1 GiB total uncompressed
	readHardCap     = 120_000
	readDefault     = 40_000
	listDefault     = 600
	searchDefault   = 100
	scanFileCap     = 2 << 20 // 2 MiB per file when searching
)

// Executor runs the model's tool calls against the job workspace (Root).
type Executor struct {
	Root   string
	skills *skillStore
}

// NewExecutor builds an executor sandboxed to an absolute, cleaned root.
func NewExecutor(root string, skills *skillStore) *Executor {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = filepath.Clean(root)
	}
	if skills == nil {
		skills = &skillStore{refs: map[string]string{}}
	}
	return &Executor{Root: abs, skills: skills}
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".svn": true, ".hg": true, "__pycache__": true,
}

var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".tgz": true,
	".7z": true, ".rar": true, ".jar": true, ".class": true, ".exe": true,
	".dll": true, ".so": true, ".o": true, ".a": true, ".bin": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".mp4": true, ".mp3": true, ".mov": true, ".wasm": true,
}

// ---- extract_archive ----

func (e *Executor) ExtractArchive(raw json.RawMessage) string {
	var in struct {
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "extract_archive error: invalid arguments"
	}
	src, err := e.resolve(in.Path)
	if err != nil {
		return "extract_archive error: " + err.Error()
	}
	dest := strings.TrimSpace(in.Dest)
	if dest == "" {
		dest = "raw/extracted"
	}
	destAbs, err := e.resolve(dest)
	if err != nil {
		return "extract_archive error: " + err.Error()
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return "extract_archive error: " + err.Error()
	}

	lower := strings.ToLower(src)
	var n int
	switch {
	case strings.HasSuffix(lower, ".zip"):
		n, err = extractZip(src, destAbs)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		n, err = extractTarGz(src, destAbs)
	case strings.HasSuffix(lower, ".tar"):
		n, err = extractTar(src, destAbs)
	default:
		return fmt.Sprintf("extract_archive: unsupported archive %q (supported: .zip, .tar, .tar.gz, .tgz). If it is already a source tree, use list_files/read_file directly.", in.Path)
	}
	if err != nil {
		return "extract_archive error: " + err.Error()
	}
	return fmt.Sprintf("extracted %d file(s) from %s into %s", n, in.Path, dest)
}

func extractZip(src, dest string) (int, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	var n int
	var total int64
	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return n, err
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		if n++; n > maxExtractFiles {
			return n, fmt.Errorf("archive has too many files (>%d)", maxExtractFiles)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return n, err
		}
		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		written, err := writeCapped(target, rc, &total)
		rc.Close()
		if err != nil {
			return n, err
		}
		_ = written
	}
	return n, nil
}

func extractTarGz(src, dest string) (int, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gz.Close()
	return readTar(tar.NewReader(gz), dest)
}

func extractTar(src, dest string) (int, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return readTar(tar.NewReader(f), dest)
}

func readTar(tr *tar.Reader, dest string) (int, error) {
	var n int
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return n, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if n++; n > maxExtractFiles {
				return n, fmt.Errorf("archive has too many files (>%d)", maxExtractFiles)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return n, err
			}
			if _, err := writeCapped(target, tr, &total); err != nil {
				return n, err
			}
		default:
			// skip symlinks/devices/etc. for safety
		}
	}
	return n, nil
}

// writeCapped copies r to target, enforcing a running total-bytes ceiling.
func writeCapped(target string, r io.Reader, total *int64) (int64, error) {
	out, err := os.Create(target)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	limit := maxExtractBytes - *total
	if limit <= 0 {
		return 0, fmt.Errorf("archive exceeds size limit (%d bytes)", maxExtractBytes)
	}
	w, err := io.Copy(out, io.LimitReader(r, limit+1))
	*total += w
	if err != nil {
		return w, err
	}
	if w > limit {
		return w, fmt.Errorf("archive exceeds size limit (%d bytes)", maxExtractBytes)
	}
	return w, nil
}

// ---- list_files ----

func (e *Executor) ListFiles(raw json.RawMessage) string {
	var in struct {
		Glob string `json:"glob"`
		Max  int    `json:"max"`
	}
	_ = json.Unmarshal(raw, &in)
	max := in.Max
	if max <= 0 || max > 5000 {
		max = listDefault
	}
	var out []string
	truncated := false
	_ = filepath.WalkDir(e.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(e.Root, p)
		rel = filepath.ToSlash(rel)
		if !matchGlob(in.Glob, rel) {
			return nil
		}
		if len(out) >= max {
			truncated = true
			return io.EOF
		}
		info, e2 := d.Info()
		size := int64(0)
		if e2 == nil {
			size = info.Size()
		}
		out = append(out, fmt.Sprintf("%s (%d)", rel, size))
		return nil
	})
	sort.Strings(out)
	res := strings.Join(out, "\n")
	if res == "" {
		res = "(no matching files)"
	}
	if truncated {
		res += fmt.Sprintf("\n… (truncated at %d entries; narrow with a glob)", max)
	}
	return res
}

func matchGlob(glob, rel string) bool {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return true
	}
	rel = filepath.ToSlash(rel)
	if ok, _ := path.Match(glob, rel); ok {
		return true
	}
	if ok, _ := path.Match(glob, path.Base(rel)); ok {
		return true
	}
	if strings.Contains(glob, "**") {
		suff := glob[strings.LastIndex(glob, "**")+2:]
		suff = strings.TrimPrefix(suff, "/")
		if suff == "" {
			return true
		}
		if ok, _ := path.Match(suff, path.Base(rel)); ok {
			return true
		}
	}
	return strings.Contains(rel, strings.Trim(glob, "*/"))
}

// ---- read_file ----

func (e *Executor) ReadFile(raw json.RawMessage) string {
	var in struct {
		Path     string `json:"path"`
		MaxBytes int    `json:"max_bytes"`
		Offset   int    `json:"offset"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "read_file error: invalid arguments"
	}
	abs, err := e.resolve(in.Path)
	if err != nil {
		return "read_file error: " + err.Error()
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "read_file error: " + err.Error()
	}
	if info.IsDir() {
		return "read_file error: path is a directory; use list_files"
	}
	max := in.MaxBytes
	if max <= 0 {
		max = readDefault
	}
	if max > readHardCap {
		max = readHardCap
	}
	f, err := os.Open(abs)
	if err != nil {
		return "read_file error: " + err.Error()
	}
	defer f.Close()
	if in.Offset > 0 {
		_, _ = f.Seek(int64(in.Offset), io.SeekStart)
	}
	buf := make([]byte, max)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]
	for i := 0; i < len(buf) && i < 8000; i++ {
		if buf[i] == 0 {
			return "read_file error: file appears to be binary"
		}
	}
	res := string(buf)
	if int64(in.Offset)+int64(n) < info.Size() {
		res += fmt.Sprintf("\n… (truncated; file is %d bytes, read %d from offset %d)", info.Size(), n, in.Offset)
	}
	return res
}

// ---- search_code ----

func (e *Executor) SearchCode(raw json.RawMessage) string {
	var in struct {
		Pattern    string `json:"pattern"`
		Glob       string `json:"glob"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "search_code error: invalid arguments"
	}
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return "search_code error: invalid regexp: " + err.Error()
	}
	max := in.MaxResults
	if max <= 0 || max > 1000 {
		max = searchDefault
	}
	var out []string
	done := fmt.Errorf("done")
	walkErr := filepath.WalkDir(e.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if binaryExt[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		rel, _ := filepath.Rel(e.Root, p)
		rel = filepath.ToSlash(rel)
		if !matchGlob(in.Glob, rel) {
			return nil
		}
		info, e2 := d.Info()
		if e2 == nil && info.Size() > scanFileCap {
			return nil
		}
		f, e3 := os.Open(p)
		if e3 != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 256*1024), 1024*1024)
		ln := 0
		for sc.Scan() {
			ln++
			line := sc.Text()
			if re.MatchString(line) {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 200 {
					trimmed = trimmed[:200] + "…"
				}
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, ln, trimmed))
				if len(out) >= max {
					return done
				}
			}
		}
		return nil
	})
	res := strings.Join(out, "\n")
	if res == "" {
		return "(no matches)"
	}
	if walkErr == done {
		res += fmt.Sprintf("\n… (truncated at %d matches)", max)
	}
	return res
}

// ---- get_knowledge ----

func (e *Executor) Knowledge(topic string) string {
	if c, ok := e.skills.get(topic); ok {
		return c
	}
	avail := strings.Join(e.skills.list(), ", ")
	if avail == "" {
		avail = "(none)"
	}
	return fmt.Sprintf("No reference document for %q. Available topics: %s. Rely on your own expertise.", topic, avail)
}

// ---- write_artifact ----

var allowedArtifacts = map[string]bool{"openapi.yaml": true, "report.md": true, "base_url.txt": true}

func (e *Executor) WriteArtifact(raw json.RawMessage) string {
	var in struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "write_artifact error: invalid arguments"
	}
	name := strings.TrimSpace(in.Name)
	if !allowedArtifacts[name] {
		return "write_artifact error: name must be one of openapi.yaml, report.md, base_url.txt"
	}
	content := in.Content
	if name == "base_url.txt" {
		content = strings.TrimSpace(firstLine(content))
	}
	if strings.TrimSpace(content) == "" {
		return "write_artifact error: content is empty"
	}
	sastDir := filepath.Join(e.Root, "sast")
	if err := os.MkdirAll(sastDir, 0o755); err != nil {
		return "write_artifact error: " + err.Error()
	}
	if err := os.WriteFile(filepath.Join(sastDir, name), []byte(content), 0o644); err != nil {
		return "write_artifact error: " + err.Error()
	}
	return fmt.Sprintf("wrote sast/%s (%d bytes)", name, len(content))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ---- validate_openapi ----

func (e *Executor) ValidateOpenAPI(raw json.RawMessage) string {
	var in struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &in)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "openapi.yaml"
	}
	if err := ValidateOpenAPIFile(filepath.Join(e.Root, "sast", name)); err != nil {
		return "INVALID: " + err.Error() + "\nFix sast/" + name + " and re-run validate_openapi."
	}
	return "valid"
}

// ValidateOpenAPIFile loads and validates an OpenAPI 3.x document with kin-openapi.
// It is also used as a server-side gate before upload.
func ValidateOpenAPIFile(p string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(b)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
