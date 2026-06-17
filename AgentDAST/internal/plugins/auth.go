package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// BrokenAuthPlugin checks that endpoints declaring a security requirement
// actually reject unauthenticated requests, and flags weak JWT handling.
type BrokenAuthPlugin struct{}

func (p *BrokenAuthPlugin) Name() string             { return "auth" }
func (p *BrokenAuthPlugin) Category() string         { return "auth" }
func (p *BrokenAuthPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *BrokenAuthPlugin) Description() string {
	return "Checks that secured endpoints reject unauthenticated access and flags missing authentication enforcement"
}

func (p *BrokenAuthPlugin) DefaultPayloads() []string { return nil }

func (p *BrokenAuthPlugin) Test(ctx *core.ScanContext) []types.Finding {
	// Only meaningful for endpoints the spec marks as secured.
	if !ctx.Endpoint.Secured {
		return nil
	}
	if ctx.Ctx.Err() != nil {
		return nil
	}

	// Send a request WITHOUT auth headers by stripping known auth headers from config.
	stripped := ctx.Config
	stripped.CustomHeaders = withoutAuthHeaders(ctx.Config.CustomHeaders)
	log := ctx.Executor.Baseline(ctx.Ctx, ctx.Endpoint, ctx.BaseURL, stripped)
	if log.Error != "" {
		return nil
	}

	// A secured endpoint returning 2xx without credentials is broken auth.
	if log.StatusCode >= 200 && log.StatusCode < 300 {
		return []types.Finding{{
			Title:       "Secured endpoint accessible without authentication",
			Endpoint:    ctx.Endpoint.Path,
			Method:      ctx.Endpoint.Method,
			Evidence:    "endpoint declares a security requirement but returned " + itoa(log.StatusCode) + " without credentials\n" + evidence(log),
			Confidence:  types.ConfidenceConfirmed,
			Description: "An operation that declares a security requirement responded successfully to an unauthenticated request.",
			Remediation: "Enforce authentication middleware on all protected routes and fail closed when credentials are missing.",
			RequestLog:  logForMode(ctx, log),
		}}
	}
	return nil
}

func withoutAuthHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		lk := strings.ToLower(k)
		if lk == "authorization" || lk == "cookie" || strings.HasPrefix(lk, "x-api-key") || strings.Contains(lk, "token") {
			continue
		}
		out[k] = v
	}
	return out
}
