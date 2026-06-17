package core

import (
	"context"

	"agentdast/internal/parser"
	"agentdast/pkg/types"
)

// ScanContext carries everything a plugin needs during a single Test() call.
type ScanContext struct {
	Ctx      context.Context
	Endpoint parser.Endpoint
	BaseURL  string // resolved target base URL for this scan
	Config   types.ScanConfig
	Executor *RequestExecutor
	// InsertPoints restricts where payloads are injected; empty means all params.
	InsertPoints []string
}

// Params returns the injection targets this run should fuzz, honoring the
// configured insert points (across query/header/path/cookie/body).
func (s *ScanContext) Params() []parser.ParamInfo {
	return s.Endpoint.ResolveInsertPoints(s.InsertPoints)
}

// Inject is a convenience wrapper that injects a payload into a parameter and
// records the resulting request through the shared executor.
func (s *ScanContext) Inject(param parser.ParamInfo, payload string) types.RequestLog {
	return s.Executor.Inject(s.Ctx, s.Endpoint, s.BaseURL, param, payload, s.Config)
}

// Baseline performs an unmodified request against the endpoint (no injection),
// useful for comparison-based plugins (CORS, auth, blind-injection baselines).
func (s *ScanContext) Baseline() types.RequestLog {
	return s.Executor.Baseline(s.Ctx, s.Endpoint, s.BaseURL, s.Config)
}
