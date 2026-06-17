package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"agentdast/internal/ai"
	"agentdast/internal/core"
	"agentdast/internal/storage"
	"agentdast/internal/toolexec"
	"agentdast/pkg/types"
)

// Config holds all service configuration, sourced from environment variables.
type Config struct {
	Addr string

	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDB       string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOUseSSL    bool
	MinIOBucket    string

	// AI provider (OpenAI-compatible).
	AIBaseURL  string
	AIAPIKey   string
	AIModel    string
	AIMaxTurns int

	MaxConcurrentScans int
}

// MySQLDSN builds a go-sql-driver DSN from the config.
func (c Config) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&multiStatements=false",
		c.MySQLUser, c.MySQLPassword, c.MySQLHost, c.MySQLPort, c.MySQLDB)
}

// LoadConfig reads configuration from the environment, applying sensible defaults.
func LoadConfig() Config {
	return Config{
		Addr: ":" + envOr("PORT", "8002"),

		MySQLHost:     envOr("MYSQL_HOST", "mysql"),
		MySQLPort:     envOr("MYSQL_PORT", "3306"),
		MySQLUser:     envOr("MYSQL_USER", "root"),
		MySQLPassword: os.Getenv("MYSQL_PASSWORD"),
		MySQLDB:       firstNonEmpty(os.Getenv("MYSQL_DATABASE"), os.Getenv("MYSQL_DB"), "agentsdast"),

		MinIOEndpoint:  envOr("MINIO_ENDPOINT", "minio:9000"),
		MinIOAccessKey: firstNonEmpty(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_ROOT_USER")),
		MinIOSecretKey: firstNonEmpty(os.Getenv("MINIO_SECRET_KEY"), os.Getenv("MINIO_ROOT_PASSWORD")),
		MinIOUseSSL:    os.Getenv("MINIO_USE_SSL") == "true",
		MinIOBucket:    envOr("MINIO_BUCKET", "agentsdast"),

		AIBaseURL:  firstNonEmpty(os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1"),
		AIAPIKey:   os.Getenv("OPENAI_API_KEY"),
		AIModel:    os.Getenv("OPENAI_MODEL"),
		AIMaxTurns: envInt("AI_MAX_TURNS", 300),

		MaxConcurrentScans: envInt("MAX_CONCURRENT_SCANS", 2),
	}
}

// MinIO object keys are fixed under the per-project prefix.
const (
	swaggerObjectFmt = "%s/sast/openapi.yaml" // <project>/sast/openapi.yaml (required)
	sastObjectFmt    = "%s/sast/report.md"    // <project>/sast/report.md   (optional)
	reportObjectFmt  = "%s/dast/report.md"    // <project>/dast/report.md
	logsObjectFmt    = "%s/dast/logs/%s.json" // <project>/dast/logs/<scan_id>.json
)

// Server is the AgentDAST HTTP service.
type Server struct {
	cfg   Config
	store *Store
	sem   chan struct{} // bounds concurrent scans

	mu      sync.Mutex                    // guards cancels
	cancels map[string]context.CancelFunc // running scan id -> cancel
}

// New builds a Server, connecting to its backends.
func New(cfg Config) (*Server, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureBucket(context.Background()); err != nil {
		slog.Warn("could not ensure bucket", "bucket", cfg.MinIOBucket, "error", err)
	}
	if cfg.MaxConcurrentScans < 1 {
		cfg.MaxConcurrentScans = 1
	}
	return &Server{
		cfg:     cfg,
		store:   store,
		sem:     make(chan struct{}, cfg.MaxConcurrentScans),
		cancels: make(map[string]context.CancelFunc),
	}, nil
}

// registerCancel stores a scan's cancel func so /api/dast/cancel can reach it.
func (s *Server) registerCancel(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	s.cancels[id] = cancel
	s.mu.Unlock()
}

// unregisterCancel drops a finished scan's cancel func.
func (s *Server) unregisterCancel(id string) {
	s.mu.Lock()
	delete(s.cancels, id)
	s.mu.Unlock()
}

// cancelRunning interrupts a scan running on this instance, if present.
func (s *Server) cancelRunning(id string) {
	s.mu.Lock()
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Close releases resources.
func (s *Server) Close() error { return s.store.Close() }

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/dast/health", s.handleHealth)
	mux.HandleFunc("/api/dast/run", s.handleScan)
	mux.HandleFunc("/api/dast/status", s.handleStatus)
	mux.HandleFunc("/api/dast/cancel", s.handleCancel)
	return logRequests(mux)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("AgentDAST server listening", "addr", s.cfg.Addr, "max_concurrent_scans", s.cfg.MaxConcurrentScans)
	return srv.ListenAndServe()
}

// --- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	resp := map[string]string{"status": "ok"}
	code := http.StatusOK
	if err := s.store.Ping(ctx); err != nil {
		resp["status"] = "degraded"
		resp["db"] = err.Error()
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}

type scanRequest struct {
	// ID is the Manager-assigned job id (optional; a new uuid is generated when empty).
	ID string `json:"id"`
	// ProjectID selects the per-project MinIO prefix where the swagger and SAST
	// report live (<project>/sast/openapi.yaml, <project>/sast/report.md).
	ProjectID string `json:"project_id"`
	// BaseURL is the live API target the scanner hits (required).
	BaseURL string `json:"base_url"`
	// Model overrides the LLM model for this scan (falls back to AgentModelConfig / env).
	Model string `json:"model"`
	// Prompt is optional free-form guidance for the auditor.
	Prompt string `json:"prompt"`
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	req := parseScanRequest(r)
	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if req.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "base_url is required")
		return
	}
	if u, err := url.ParseRequestURI(req.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		writeError(w, http.StatusBadRequest, "base_url must be a valid http(s) URL")
		return
	}
	modelName, err := s.resolveModel(r.Context(), req.Model)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	req.Model = modelName
	if s.cfg.AIAPIKey == "" {
		writeError(w, http.StatusServiceUnavailable, "AI is not configured (set OPENAI_API_KEY)")
		return
	}

	// The swagger is required and read from a fixed MinIO key. Reject up front if
	// it is missing so the caller gets immediate "missing" feedback. (The SAST
	// report is optional and resolved later, in the worker.)
	swaggerKey := fmt.Sprintf(swaggerObjectFmt, req.ProjectID)
	switch ok, err := s.store.StatObject(r.Context(), swaggerKey); {
	case err != nil:
		writeError(w, http.StatusInternalServerError, "check swagger: "+err.Error())
		return
	case !ok:
		writeError(w, http.StatusBadRequest, "missing: "+swaggerKey)
		return
	}

	scanID := strings.TrimSpace(req.ID)
	if scanID == "" {
		scanID = uuid.NewString()
	}
	if err := s.store.EnsureScan(r.Context(), scanID, req.ProjectID); err != nil {
		if strings.Contains(err.Error(), "already finished") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Run asynchronously; the client polls /api/dast/status.
	go s.runScan(scanID, req)

	writeJSON(w, http.StatusAccepted, map[string]string{"id": scanID})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	scanID := r.URL.Query().Get("id")
	if scanID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	rec, err := s.store.GetScan(r.Context(), scanID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rec == nil {
		writeError(w, http.StatusNotFound, "scan not found")
		return
	}
	resp := map[string]interface{}{
		"id":          rec.ID,
		"project_id":  rec.ProjectID,
		"status":      rec.Status,
		"progress":    rec.Progress,
		"phase":       rec.Phase,
		"last_update": rec.LastUpdate.UTC().Format(time.RFC3339),
	}
	if rec.Status == StatusDone {
		resp["result_path"] = rec.ResultPath
	}
	if rec.Status == StatusFail {
		resp["error"] = rec.Error
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	scanID := strings.TrimSpace(r.URL.Query().Get("id"))
	if scanID == "" {
		scanID = strings.TrimSpace(r.FormValue("id"))
	}
	if scanID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	// Flip the row to "cancel" only while it is still new/in-progress.
	cancelled, err := s.store.CancelScan(r.Context(), scanID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !cancelled {
		// Either the scan does not exist or it already finished.
		rec, gerr := s.store.GetScan(r.Context(), scanID)
		if gerr != nil {
			writeError(w, http.StatusInternalServerError, gerr.Error())
			return
		}
		if rec == nil {
			writeError(w, http.StatusNotFound, "scan not found")
			return
		}
		writeError(w, http.StatusConflict, "cannot cancel scan in status "+rec.Status)
		return
	}

	// Interrupt the running goroutine (if it is on this instance).
	s.cancelRunning(scanID)
	writeJSON(w, http.StatusOK, map[string]string{"id": scanID, "status": StatusCancel})
}

// runScan executes the full AI audit flow for one scan, off the request path. It
// reads the swagger (required) and SAST report (optional) from fixed MinIO keys
// under the project prefix, scans the live API at req.BaseURL, then writes the
// report to <project>/dast/report.md and request logs to <project>/dast/logs/.
func (s *Server) runScan(scanID string, req scanRequest) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	// ctx drives the scan work and is cancellable via /api/dast/cancel. State
	// writes use a separate background context so they still succeed after cancel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.registerCancel(scanID, cancel)
	defer s.unregisterCancel(scanID)
	bg := context.Background()

	lg := slog.With("component", "scan", "id", scanID, "project_id", req.ProjectID)
	started := time.Now()
	lg.Info("scan accepted", "base_url", req.BaseURL, "model", req.Model)

	// fail routes cancellation to the "cancel" state (set by handleCancel) and
	// everything else to "fail".
	fail := func(stage string, err error) {
		if ctx.Err() != nil {
			lg.Info("scan canceled", "stage", stage, "elapsed", time.Since(started).String())
			return
		}
		lg.Error("scan failed", "stage", stage, "error", err, "elapsed", time.Since(started).String())
		_ = s.store.SetStatus(bg, scanID, StatusFail, "", fmt.Sprintf("%s: %v", stage, err))
	}

	if err := s.store.SetStatus(bg, scanID, StatusProcess, "", ""); err != nil {
		lg.Warn("could not mark process", "error", err)
	}
	_ = s.store.SetProgress(bg, scanID, StatusProcess, "loading swagger", 15)

	// 1. Fixed MinIO keys under the project prefix; base URL from the request.
	swaggerKey := fmt.Sprintf(swaggerObjectFmt, req.ProjectID)
	sastKey := fmt.Sprintf(sastObjectFmt, req.ProjectID)
	baseURL := req.BaseURL

	// 2. Download the swagger from MinIO to a temp file (existence was checked
	// synchronously at request time).
	data, err := s.store.GetObject(ctx, swaggerKey)
	if err != nil {
		fail("download swagger", err)
		return
	}
	swaggerFile, cleanup, err := WriteTempFile(swaggerKey, data)
	if err != nil {
		fail("stage swagger", err)
		return
	}
	defer cleanup()

	// 3. Download the SAST report (optional). A missing report is not fatal — the
	// audit proceeds dynamically without it.
	sastFile := ""
	if sastData, derr := s.store.GetObject(ctx, sastKey); derr != nil {
		if ctx.Err() != nil {
			fail("download SAST report", derr)
			return
		}
		lg.Warn("no SAST report found; auditing dynamically without it", "key", sastKey, "error", derr)
	} else {
		var sastCleanup func()
		sastFile, sastCleanup, err = WriteTempFile(sastKey, sastData)
		if err != nil {
			fail("stage SAST report", err)
			return
		}
		defer sastCleanup()
		lg.Info("loaded SAST report", "key", sastKey, "bytes", len(sastData))
	}

	_ = s.store.SetProgress(bg, scanID, StatusProcess, "running AI audit", 40)

	// 4. Run the AI audit flow (baseline scan + LLM-driven verification).
	aiCfg := types.AIConfig{
		BaseURL:       s.cfg.AIBaseURL,
		APIKey:        s.cfg.AIAPIKey,
		Model:         req.Model,
		MaxTurns:      s.cfg.AIMaxTurns,
		SASTReport:    sastFile,
		Context:       req.Prompt,
		TargetBaseURL: baseURL,
		CollectLogs:   true, // retain request logs so we can persist them
	}.WithDefaults()

	exec := toolexec.New(core.GlobalRegistry, storage.NewInMemoryStore())
	orch := ai.NewOrchestrator(aiCfg, exec)
	orch.Logger = lg.With("component", "ai")

	audit, err := orch.Run(ctx, swaggerFile)
	if err != nil {
		fail("ai audit", err)
		return
	}
	if ctx.Err() != nil { // cancelled between finishing and persisting
		fail("ai audit", ctx.Err())
		return
	}

	// 5. Persist request logs (best-effort) to <project>/dast/logs/<scan_id>.json.
	s.persistLogs(ctx, req.ProjectID, scanID, audit, lg)

	_ = s.store.SetProgress(bg, scanID, StatusProcess, "writing report", 90)

	// 6. Store the report at <project>/dast/report.md and record its key.
	resultKey := fmt.Sprintf(reportObjectFmt, req.ProjectID)
	if err := s.store.PutObject(ctx, resultKey, []byte(audit.Report), "text/markdown; charset=utf-8"); err != nil {
		fail("upload report", err)
		return
	}
	if err := s.store.SetStatus(bg, scanID, StatusDone, resultKey, ""); err != nil {
		lg.Warn("could not mark done", "error", err)
	}
	lg.Info("scan done",
		"result_path", resultKey,
		"findings", audit.ConfirmedFindings(),
		"scans", len(audit.Scans),
		"elapsed", time.Since(started).String())
}

// persistLogs writes the aggregated request/response logs from every scan in the
// audit to <project>/dast/logs/<scan_id>.json. It is best-effort: a failure is
// logged but does not fail the scan.
func (s *Server) persistLogs(ctx context.Context, projectID, scanID string, audit *ai.AuditResult, lg *slog.Logger) {
	var logs []types.RequestLog
	for _, sc := range audit.Scans {
		if sc != nil {
			logs = append(logs, sc.RequestLogs...)
		}
	}
	if len(logs) == 0 {
		return
	}
	data, err := json.Marshal(logs)
	if err != nil {
		lg.Warn("could not marshal scan logs", "error", err)
		return
	}
	key := fmt.Sprintf(logsObjectFmt, projectID, scanID)
	if err := s.store.PutObject(ctx, key, data, "application/json"); err != nil {
		lg.Warn("could not upload scan logs", "key", key, "error", err)
		return
	}
	lg.Info("uploaded scan logs", "key", key, "requests", len(logs))
}

// --- helpers ---

func parseScanRequest(r *http.Request) scanRequest {
	var req scanRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	// Fall back to / allow override via query or form values.
	if req.ID == "" {
		req.ID = r.FormValue("id")
	}
	if req.ProjectID == "" {
		req.ProjectID = r.FormValue("project_id")
	}
	if req.BaseURL == "" {
		req.BaseURL = r.FormValue("base_url")
	}
	if req.Prompt == "" {
		req.Prompt = r.FormValue("prompt")
	}
	if req.Model == "" {
		req.Model = r.FormValue("model")
	}
	req.ID = strings.TrimSpace(req.ID)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	return req
}

func (s *Server) resolveModel(ctx context.Context, override string) (string, error) {
	if model := strings.TrimSpace(override); model != "" {
		return model, nil
	}
	if model, err := s.store.EnabledAgentModel(ctx, "dast"); err == nil && model != "" {
		return model, nil
	}
	if model := strings.TrimSpace(s.cfg.AIModel); model != "" {
		return model, nil
	}
	return "", fmt.Errorf("no model configured: set AgentModelConfig for dast or OPENAI_MODEL")
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// statusRecorder captures the response status code for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		} else if rec.status >= 400 {
			level = slog.LevelWarn
		}
		slog.LogAttrs(r.Context(), level, "http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.String("duration", time.Since(start).String()),
			slog.String("remote", r.RemoteAddr),
		)
	})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
