package plugins

import (
	"net/http"
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// IDORPlugin probes for Broken Object Level Authorization (IDOR/BOLA) on
// identifier path parameters.
//
// True IDOR requires two identities, which a single-pass scanner cannot supply,
// so this plugin only flags a *candidate* when ALL of the following hold:
//   - the endpoint declares a security requirement (authorization is expected)
//   - an id-like path parameter exists
//   - several distinct identifiers each return 200 with DIFFERENT object bodies
//
// Requiring "secured" avoids flagging public catalogs (e.g. product listings),
// and requiring distinct bodies avoids matching uniform error/empty responses.
type IDORPlugin struct{}

func (p *IDORPlugin) Name() string             { return "idor" }
func (p *IDORPlugin) Category() string         { return "auth" }
func (p *IDORPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *IDORPlugin) Description() string {
	return "Flags secured endpoints whose object identifiers are enumerable (IDOR/BOLA candidates needing two-account verification)"
}

func (p *IDORPlugin) DefaultPayloads() []string {
	return []string{"1", "2", "3"}
}

func looksLikeID(name string) bool {
	n := strings.ToLower(name)
	return n == "id" || strings.HasSuffix(n, "id") || strings.Contains(n, "uuid") || strings.Contains(n, "guid")
}

func (p *IDORPlugin) Test(ctx *core.ScanContext) []types.Finding {
	// Authorization only matters where the spec says the endpoint is protected.
	if !ctx.Endpoint.Secured {
		return nil
	}
	var findings []types.Finding
	for _, param := range ctx.Params() {
		if param.In != "path" || !looksLikeID(param.Name) {
			continue
		}
		var bodies []string
		var sample types.RequestLog
		for _, id := range p.DefaultPayloads() {
			if ctx.Ctx.Err() != nil {
				return findings
			}
			log := ctx.Inject(param, id)
			if log.Error != "" || log.StatusCode != http.StatusOK {
				continue
			}
			body := strings.TrimSpace(log.ResponseBody)
			if body == "" {
				continue
			}
			bodies = append(bodies, body)
			sample = log
		}
		// Need at least two accessible objects that are genuinely different.
		if len(bodies) >= 2 && lengthDelta(bodies[0], bodies[len(bodies)-1]) > 0.05 {
			findings = append(findings, types.Finding{
				Title:       "IDOR/BOLA candidate on path parameter " + param.Name,
				Endpoint:    ctx.Endpoint.Path,
				Method:      ctx.Endpoint.Method,
				ParamName:   param.Name,
				ParamIn:     param.In,
				Evidence:    "secured endpoint returned distinct objects for sequential identifiers without per-object authorization context\n" + evidence(sample),
				Confidence:  types.ConfidencePossible,
				Description: "A protected endpoint exposes enumerable object identifiers. Confirm with two separate accounts that one can read another's object.",
				Remediation: "Enforce per-object ownership checks on every request; scope queries to the authenticated principal.",
				RequestLog:  logForMode(ctx, sample),
			})
		}
	}
	return findings
}
