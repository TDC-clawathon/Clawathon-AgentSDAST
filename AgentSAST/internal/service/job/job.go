package job

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"agentsast/internal/ai"
	"agentsast/internal/db/model"
	"agentsast/internal/db/repo"
	"agentsast/internal/store"

	"github.com/google/uuid"
)

// AIParams holds the LLM connection + loop settings shared across jobs.
type AIParams struct {
	BaseURL   string
	APIKey    string
	MaxTurns  int
	SkillsDir string
}

// Service orchestrates a SAST job: pull <project>/raw -> run the AI tool loop ->
// push <project>/sast/{openapi.yaml,report.md}.
type Service struct {
	repo         *repo.SAST
	modelRepo    *repo.ModelConfig
	store        *store.Client
	ai           AIParams
	workRoot     string
	defaultModel string
	mgr          *manager
}

func New(r *repo.SAST, modelRepo *repo.ModelConfig, s *store.Client, aiParams AIParams, workRoot, defaultModel string) *Service {
	return &Service{
		repo:         r,
		modelRepo:    modelRepo,
		store:        s,
		ai:           aiParams,
		workRoot:     workRoot,
		defaultModel: strings.TrimSpace(defaultModel),
		mgr:          newManager(),
	}
}

// Start creates a job record and runs it in the background. Returns the job id.
// modelOverride may come from the Manager request; otherwise the DB assignment is used.
// modes selects which SAST skill sets execute (quickscan/deepscan/pgwscan).
func (s *Service) Start(projectID, swaggerBaseURL, modelOverride string, modes []string) (string, error) {
	id := uuid.NewString()
	rec := &model.SAST{ID: id, ProjectID: projectID, Status: model.ScanStatusNew, Phase: "initializing", Progress: 0}
	if err := s.repo.Create(rec); err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mgr.add(id, cancel)
	go func() {
		defer s.mgr.remove(id)
		defer cancel()
		s.run(ctx, id, projectID, swaggerBaseURL, modelOverride, modes)
	}()
	return id, nil
}

// Cancel signals a running job. Returns false if it is not currently running.
func (s *Service) Cancel(id string) bool { return s.mgr.cancel(id) }

func (s *Service) run(ctx context.Context, id, projectID, swaggerBaseURL, modelOverride string, modes []string) {
	work := filepath.Join(s.workRoot, id)
	defer os.RemoveAll(work)
	rawDir := filepath.Join(work, "raw")
	sastDir := filepath.Join(work, "sast")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		s.finish(ctx, id, err)
		return
	}

	_ = s.repo.Update(id, map[string]any{"status": model.ScanStatusProcess, "phase": "planning", "progress": 10, "last_message": "starting analysis…"})

	// 1) pull the raw upload (any layout/format)
	_ = s.repo.SetProgress(id, "running", 15)
	n, err := s.store.DownloadPrefix(ctx, projectID+"/raw", rawDir)
	if err != nil {
		s.finish(ctx, id, fmt.Errorf("download %s/raw: %w", projectID, err))
		return
	}
	if n == 0 {
		s.finish(ctx, id, fmt.Errorf("no objects found under %s/raw", projectID))
		return
	}

	modelName, err := s.resolveModel(modelOverride)
	if err != nil {
		s.finish(ctx, id, err)
		return
	}
	log.Printf("[%s] using model %s", short(id), modelName)

	// 2) run the AI tool loop: it extracts archives, reads source, and writes
	//    ./sast/{openapi.yaml,report.md,base_url.txt}.
	skills := ai.LoadSkills(s.ai.SkillsDir, modes)
	effective := skills.Modes()
	log.Printf("[%s] scan modes: %s", short(id), strings.Join(effective, "+"))
	_ = s.repo.SetProgress(id, "analyzing ("+strings.Join(effective, "+")+")", 40)
	exec := ai.NewExecutor(work, skills)
	orch := ai.NewOrchestrator(ai.Config{
		BaseURL:     s.ai.BaseURL,
		APIKey:      s.ai.APIKey,
		Model:       modelName,
		MaxTurns:    s.ai.MaxTurns,
		WorkDir:     work,
		BaseURLHint: swaggerBaseURL,
	}, exec)
	result, err := orch.Run(ctx, func(activity string) {
		_ = s.repo.SetMessage(id, activity)
		log.Printf("[%s] %s", short(id), activity)
	})
	if err != nil {
		s.finish(ctx, id, fmt.Errorf("analysis: %w", err))
		return
	}

	// 3) verify the two deliverables exist
	openapiLocal := filepath.Join(sastDir, "openapi.yaml")
	reportLocal := filepath.Join(sastDir, "report.md")
	if !fileExists(openapiLocal) || !fileExists(reportLocal) {
		s.finish(ctx, id, errors.New("analysis did not produce ./sast/openapi.yaml and ./sast/report.md"))
		return
	}
	// Server-side gate: never upload an invalid OpenAPI document.
	if verr := ai.ValidateOpenAPIFile(openapiLocal); verr != nil {
		s.finish(ctx, id, fmt.Errorf("openapi validation failed: %w", verr))
		return
	}

	// 4) push results to fixed object keys
	_ = s.repo.SetProgress(id, "generating report", 85)
	swKey := projectID + "/sast/openapi.yaml"
	repKey := projectID + "/sast/report.md"
	if err := s.store.UploadFile(ctx, openapiLocal, swKey, "application/yaml"); err != nil {
		s.finish(ctx, id, fmt.Errorf("upload openapi: %w", err))
		return
	}
	if err := s.store.UploadFile(ctx, reportLocal, repKey, "text/markdown; charset=utf-8"); err != nil {
		s.finish(ctx, id, fmt.Errorf("upload report: %w", err))
		return
	}

	_ = s.repo.Update(id, map[string]any{
		"status":                  model.ScanStatusDone,
		"progress":                100,
		"phase":                   "completed",
		"result_swagger_path":     swKey,
		"result_report_path":      repKey,
		"result_swagger_base_url": result.BaseURL,
		"last_message":            "completed",
	})
}

// finish records the terminal state: canceled vs failed.
func (s *Service) finish(ctx context.Context, id string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		_ = s.repo.Update(id, map[string]any{"status": model.ScanStatusCanceled, "phase": "canceled", "last_message": "canceled"})
		return
	}
	log.Printf("[%s] failed: %v", short(id), err)
	_ = s.repo.Update(id, map[string]any{"status": model.ScanStatusFailed, "phase": "failed", "last_message": err.Error()})
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func short(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

func (s *Service) resolveModel(override string) (string, error) {
	if model := strings.TrimSpace(override); model != "" {
		return model, nil
	}
	if s.modelRepo != nil {
		if model, err := s.modelRepo.EnabledModel("sast"); err == nil && model != "" {
			return model, nil
		}
	}
	if s.defaultModel != "" {
		return s.defaultModel, nil
	}
	return "", fmt.Errorf("no model configured: set AgentModelConfig for sast or LLM_MODEL")
}
