// Package knowledge serves per-vulnerability reference material that the AI
// auditor can pull on demand. Canonical files live in repo root skills/dast/
// (SKILLS_DAST_DIR or auto-detect from repo root).
package knowledge

import "strings"

// aliases map human phrasings (after normalization) to a knowledge file base.
var aliases = map[string]string{
	"bola":                              "idor",
	"broken_object_level_authorization": "idor",
	"insecure_direct_object_reference":  "idor",
	"sql":                               "sqli",
	"sql_injection":                     "sqli",
	"sqlinjection":                      "sqli",
	"cross_site_scripting":              "xss",
	"command_injection":                 "cmdi",
	"os_command_injection":              "cmdi",
	"rce":                               "cmdi",
	"directory_traversal":               "path_traversal",
	"lfi":                               "path_traversal",
	"local_file_inclusion":              "path_traversal",
	"server_side_request_forgery":       "ssrf",
	"broken_authentication":             "auth",
	"authentication":                    "auth",
	"broken_object_property_level_auth": "mass_assignment",
	"bopla":                             "mass_assignment",
	"excessive_data_exposure":           "sensitive_data",
	"sensitive_data_exposure":           "sensitive_data",
	"data_exposure":                     "sensitive_data",
	"xml_external_entity":               "xxe",
	"open_redirect":                     "open_redirect",
	"unvalidated_redirect":              "open_redirect",
	"template_injection":                "ssti",
	"server_side_template_injection":    "ssti",
	"security_misconfiguration":         "cors",
}

// normalize lower-cases and canonicalizes a vulnerability name to a file base.
func normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.NewReplacer("-", "_", " ", "_", "/", "_").Replace(n)
	n = strings.TrimSuffix(n, ".md")
	if a, ok := aliases[n]; ok {
		return a
	}
	return n
}

// Get returns the knowledge document for a vulnerability name (accepting common
// aliases). ok is false when no document exists, signalling the caller to fall
// back to its own knowledge.
func Get(name string) (content string, ok bool) {
	base := normalize(name)
	if base == "" {
		return "", false
	}
	return currentSource.get(base)
}

// List returns the available knowledge topic names (file bases), sorted.
func List() []string {
	return currentSource.list()
}
