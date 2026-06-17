package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// SSTIPlugin detects Server-Side Template Injection by injecting template
// expressions that evaluate a distinctive arithmetic product and checking the
// computed result appears in the response (not the literal expression).
type SSTIPlugin struct{}

func (p *SSTIPlugin) Name() string             { return "ssti" }
func (p *SSTIPlugin) Category() string         { return "injection" }
func (p *SSTIPlugin) Severity() types.Severity { return types.SeverityCritical }
func (p *SSTIPlugin) Description() string {
	return "Detects Server-Side Template Injection by evaluating a distinctive arithmetic expression across template engines"
}

// The factors and their product are distinctive enough that the product
// appearing in the response is strong evidence of evaluation (not reflection).
const (
	sstiExpr    = "1337*1337"
	sstiProduct = "1787569"
)

func (p *SSTIPlugin) DefaultPayloads() []string {
	return []string{
		"{{" + sstiExpr + "}}",    // Jinja2, Twig, Nunjucks
		"${" + sstiExpr + "}",     // FreeMarker, JSP EL, Thymeleaf
		"#{" + sstiExpr + "}",     // Ruby, Thymeleaf
		"<%= " + sstiExpr + " %>", // ERB, EJS
		"*{" + sstiExpr + "}",     // Thymeleaf selection
		"@(" + sstiExpr + ")",     // Razor
	}
}

func (p *SSTIPlugin) Test(ctx *core.ScanContext) []types.Finding {
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
			// The product must appear in a response that does NOT merely echo the
			// expression — stripPayload removes the reflected payload first.
			if strings.Contains(stripPayload(log.ResponseBody, payload), sstiProduct) {
				findings = append(findings, types.Finding{
					Title:       "Server-Side Template Injection in parameter " + param.Name,
					Endpoint:    ctx.Endpoint.Path,
					Method:      ctx.Endpoint.Method,
					ParamName:   param.Name,
					ParamIn:     param.In,
					Payload:     payload,
					Evidence:    "template expression evaluated to " + sstiProduct + " in the response\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "User input is evaluated by a server-side template engine, allowing template expression execution that often escalates to RCE.",
					Remediation: "Never pass user input into template source; use a logic-less/sandboxed template and pass data as bound variables only.",
					RequestLog:  logForMode(ctx, log),
				})
				break
			}
		}
	}
	return findings
}
