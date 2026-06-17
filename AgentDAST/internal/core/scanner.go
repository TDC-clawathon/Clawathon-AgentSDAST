package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"agentdast/internal/parser"
	"agentdast/pkg/types"
)

// Scanner orchestrates a full scan run over a parsed spec and a set of plugins.
type Scanner struct {
	registry *PluginRegistry
}

// NewScanner returns a Scanner backed by the given registry (defaults to GlobalRegistry).
func NewScanner(registry *PluginRegistry) *Scanner {
	if registry == nil {
		registry = GlobalRegistry
	}
	return &Scanner{registry: registry}
}

// Run resolves the scan source (single URL or spec), resolves plugins, and
// fans out (endpoint × plugin) tests.
func (s *Scanner) Run(ctx context.Context, cfg types.ScanConfig) (*types.ScanResult, error) {
	cfg = cfg.WithDefaults()
	result := &types.ScanResult{
		ID:         uuid.NewString(),
		ScanConfig: cfg,
		Status:     types.ScanStatusRunning,
		StartedAt:  time.Now(),
		Findings:   []types.Finding{},
	}

	plugins, err := s.registry.Resolve(cfg.Plugins)
	if err != nil {
		return s.fail(result, err), err
	}

	endpoints, baseURL, err := s.resolveTargets(cfg)
	if err != nil {
		return s.fail(result, err), err
	}

	executor := NewRequestExecutor(cfg)
	if cfg.OutputMode == types.OutputModeFull {
		executor.EnableLogCollection()
	}

	slog.Debug("scan started", "scan_id", result.ID, "base_url", baseURL,
		"endpoints", len(endpoints), "plugins", len(plugins), "concurrency", cfg.Concurrency)

	findings := s.scanEndpoints(ctx, executor, endpoints, baseURL, plugins, cfg)

	now := time.Now()
	result.CompletedAt = &now
	result.Status = types.ScanStatusCompleted
	result.Findings = findings
	result.Summary = types.Summarize(findings)
	result.Summary.TotalEndpoints = len(endpoints)
	result.Summary.TotalRequests = executor.RequestCount()
	if cfg.OutputMode == types.OutputModeFull {
		result.RequestLogs = executor.CollectedLogs()
	}

	slog.Debug("scan complete", "scan_id", result.ID,
		"requests", result.Summary.TotalRequests, "findings", result.Summary.TotalFindings,
		"duration", now.Sub(result.StartedAt).String())
	return result, nil
}

// resolveTargets returns the endpoints to scan and the base URL, choosing
// between a single-URL scan (TargetURL) and a spec scan (SwaggerSource).
func (s *Scanner) resolveTargets(cfg types.ScanConfig) ([]parser.Endpoint, string, error) {
	switch {
	case cfg.TargetURL != "":
		ep, base, err := BuildEndpointFromURL(cfg.TargetURL, cfg.Method, cfg.TargetBaseURL, cfg.Body, cfg.BodyParams)
		if err != nil {
			return nil, "", err
		}
		return []parser.Endpoint{ep}, base, nil
	case cfg.SwaggerSource != "":
		spec, err := parser.LoadSpec(cfg.SwaggerSource)
		if err != nil {
			return nil, "", err
		}
		base := cfg.TargetBaseURL
		if base == "" {
			base = spec.BaseURL
		}
		return spec.Endpoints, base, nil
	default:
		return nil, "", fmt.Errorf("nothing to scan: set target_url or swagger_source")
	}
}

// scanEndpoints runs every plugin against every endpoint, bounded by concurrency.
func (s *Scanner) scanEndpoints(
	ctx context.Context,
	executor *RequestExecutor,
	endpoints []parser.Endpoint,
	baseURL string,
	plugins []Plugin,
	cfg types.ScanConfig,
) []types.Finding {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		findings []types.Finding
	)
	sem := make(chan struct{}, cfg.Concurrency)

	for _, ep := range endpoints {
		for _, pl := range plugins {
			select {
			case <-ctx.Done():
				goto wait
			default:
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(ep parser.Endpoint, pl Plugin) {
				defer wg.Done()
				defer func() { <-sem }()
				// A panicking plugin must not take down the whole scan.
				defer func() {
					if r := recover(); r != nil {
						slog.Error("plugin panicked", "plugin", pl.Name(), "method", ep.Method, "path", ep.Path, "panic", r)
					}
				}()
				slog.Debug("testing", "plugin", pl.Name(), "method", ep.Method, "path", ep.Path)

				sc := &ScanContext{
					Ctx:          ctx,
					Endpoint:     ep,
					BaseURL:      baseURL,
					Config:       cfg,
					Executor:     executor,
					InsertPoints: cfg.InsertPoints,
				}
				found := pl.Test(sc)
				if len(found) == 0 {
					return
				}
				mu.Lock()
				findings = append(findings, decorate(found, pl)...)
				mu.Unlock()
			}(ep, pl)
		}
	}

wait:
	wg.Wait()
	return findings
}

// decorate fills in identity/metadata fields a plugin may have left blank.
func decorate(findings []types.Finding, pl Plugin) []types.Finding {
	for i := range findings {
		if findings[i].ID == "" {
			findings[i].ID = uuid.NewString()
		}
		if findings[i].Plugin == "" {
			findings[i].Plugin = pl.Name()
		}
		if findings[i].Category == "" {
			findings[i].Category = pl.Category()
		}
		if findings[i].Severity == "" {
			findings[i].Severity = pl.Severity()
		}
		if findings[i].Confidence == "" {
			findings[i].Confidence = types.ConfidenceProbable
		}
	}
	return findings
}

func (s *Scanner) fail(result *types.ScanResult, err error) *types.ScanResult {
	now := time.Now()
	result.Status = types.ScanStatusFailed
	result.CompletedAt = &now
	result.Error = err.Error()
	return result
}
