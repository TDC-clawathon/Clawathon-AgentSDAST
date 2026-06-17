package plugins

import (
	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// CommandInjectionPlugin detects OS command injection via output-signature matching.
type CommandInjectionPlugin struct{}

func (p *CommandInjectionPlugin) Name() string             { return "cmdi" }
func (p *CommandInjectionPlugin) Category() string         { return "injection" }
func (p *CommandInjectionPlugin) Severity() types.Severity { return types.SeverityCritical }
func (p *CommandInjectionPlugin) Description() string {
	return "Detects OS command injection by injecting shell separators and matching command-output signatures"
}

func (p *CommandInjectionPlugin) DefaultPayloads() []string {
	return []string{
		// separator variations (Unix)
		"; id", "| id", "|| id", "& id", "&& id",
		// substitution / quoted contexts
		"`id`", "$(id)", "'; id; '", "\"; id; \"",
		// newline / carriage-return injection (bypasses naive ; & | filters)
		"\nid", "%0aid", "\rid",
		// file read fallbacks
		"; cat /etc/passwd", "| cat /etc/passwd",
		// Windows variants
		"& type C:\\Windows\\win.ini", "| type C:\\Windows\\win.ini", "&& whoami",
	}
}

// cmdiSignatures appear in the output of the injected commands above.
var cmdiSignatures = []string{
	"uid=",
	"gid=",
	"groups=",
	"root:x:0:0:",
	"daemon:x:",
	"[fonts]",
	"[extensions]",
	"; for 16-bit app support",
}

func (p *CommandInjectionPlugin) Test(ctx *core.ScanContext) []types.Finding {
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
			if sig, ok := matchSignal(log.ResponseBody, payload, cmdiSignatures); ok {
				findings = append(findings, types.Finding{
					Title:       "OS command injection in parameter " + param.Name,
					Endpoint:    ctx.Endpoint.Path,
					Method:      ctx.Endpoint.Method,
					ParamName:   param.Name,
					ParamIn:     param.In,
					Payload:     payload,
					Evidence:    "matched command-output signature: " + sig + "\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "Injected shell command output appeared in the response, indicating user input reaches an OS command.",
					Remediation: "Avoid shelling out; if unavoidable, use argument arrays (no shell), strict allow-lists, and input validation.",
					RequestLog:  logForMode(ctx, log),
				})
				break
			}
		}
	}
	return findings
}
