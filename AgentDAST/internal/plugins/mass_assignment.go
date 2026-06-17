package plugins

import (
	"encoding/json"
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// MassAssignmentPlugin tests whether unexpected privileged fields are accepted
// and persisted. It injects privileged fields with DISTINCTIVE sentinel values
// and only flags when the response echoes the field together with that exact
// sentinel — proving the server bound an attacker-controlled property rather
// than merely echoing arbitrary input.
type MassAssignmentPlugin struct{}

func (p *MassAssignmentPlugin) Name() string             { return "mass_assignment" }
func (p *MassAssignmentPlugin) Category() string         { return "injection" }
func (p *MassAssignmentPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *MassAssignmentPlugin) Description() string {
	return "Tests for mass assignment by injecting privileged fields with sentinel values and confirming they are bound and reflected"
}

func (p *MassAssignmentPlugin) DefaultPayloads() []string {
	return []string{"role", "isAdmin", "admin", "verified", "isPremium"}
}

// privileged maps each injected field to a distinctive sentinel value. The
// sentinels are unlikely to appear naturally, so a reflected sentinel is strong
// evidence the field was actually bound.
var privileged = []struct {
	field    string
	value    interface{}
	sentinel string
}{
	{"role", "zz_admin_zz", "zz_admin_zz"},
	{"isAdmin", true, "true"},
	{"admin", true, "true"},
	{"verified", true, "true"},
	{"isPremium", true, "true"},
	{"accountBalance", 1337421, "1337421"},
}

func (p *MassAssignmentPlugin) Test(ctx *core.ScanContext) []types.Finding {
	body := ctx.Endpoint.RequestBody
	if body == nil || !strings.Contains(strings.ToLower(body.ContentType), "json") {
		return nil
	}
	if ctx.Ctx.Err() != nil {
		return nil
	}

	existing := make(map[string]bool, len(body.Fields))
	for _, f := range body.Fields {
		existing[strings.ToLower(f)] = true
	}

	// Build a base object from declared fields, then add only fields that are
	// NOT already part of the documented schema (truly unexpected properties).
	payload := map[string]interface{}{}
	for _, f := range body.Fields {
		payload[f] = "test"
	}
	var injected []string
	for _, pv := range privileged {
		if existing[strings.ToLower(pv.field)] {
			continue
		}
		payload[pv.field] = pv.value
		injected = append(injected, pv.field)
	}
	if len(injected) == 0 {
		return nil
	}

	raw, _ := json.Marshal(payload)
	log := ctx.Executor.SendRawBody(ctx.Ctx, ctx.Endpoint, ctx.BaseURL, "application/json", string(raw), ctx.Config)
	if log.Error != "" {
		return nil
	}

	var findings []types.Finding
	respLower := strings.ToLower(log.ResponseBody)
	for _, pv := range privileged {
		if existing[strings.ToLower(pv.field)] {
			continue
		}
		// Require BOTH the field name AND its distinctive sentinel value present.
		if strings.Contains(respLower, strings.ToLower(pv.field)) && strings.Contains(respLower, strings.ToLower(pv.sentinel)) {
			findings = append(findings, types.Finding{
				Title:       "Mass assignment via field " + pv.field,
				Endpoint:    ctx.Endpoint.Path,
				Method:      ctx.Endpoint.Method,
				ParamName:   pv.field,
				ParamIn:     "body",
				Payload:     pv.field + "=" + pv.sentinel,
				Evidence:    "injected undocumented field reflected with its sentinel value\n" + evidence(log),
				Confidence:  types.ConfidenceProbable,
				Description: "An undocumented privileged field was accepted and reflected with its injected value, indicating mass assignment / auto-binding.",
				Remediation: "Bind requests to explicit allow-listed DTOs; never auto-bind client JSON onto internal models.",
				RequestLog:  logForMode(ctx, log),
			})
		}
	}
	return findings
}
