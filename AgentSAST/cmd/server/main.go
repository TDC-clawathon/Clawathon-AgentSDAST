// Command server is the AgentSAST HTTP service.
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"agentsast/internal/ai"
	"agentsast/internal/config"
	"agentsast/internal/db"
	"agentsast/internal/db/repo"
	"agentsast/internal/handler"
	"agentsast/internal/service/job"
	"agentsast/internal/store"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if cfg.LLM.BaseURL == "" || cfg.LLM.APIKey == "" || cfg.LLM.Model == "" {
		log.Fatalf("LLM not configured — set LLM_BASE_URL, LLM_API_KEY and LLM_MODEL (.env)")
	}

	gdb, err := db.Open(cfg.MySQL.DSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}

	mc, err := store.New(cfg.MinIO.Endpoint, cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, cfg.MinIO.Bucket, cfg.MinIO.UseSSL)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := mc.EnsureBucket(ctx); err != nil {
		cancel()
		log.Fatalf("minio bucket: %v", err)
	}
	cancel()

	skillsDir := ai.ResolveSastSkillsDir()
	if skillsDir == "" {
		log.Printf("warning: skills/sast not found (set SKILLS_SAST_DIR); running on model expertise only")
	} else {
		log.Printf("sast skills dir: %s", skillsDir)
	}

	aiParams := job.AIParams{
		BaseURL:   cfg.LLM.BaseURL,
		APIKey:    cfg.LLM.APIKey,
		MaxTurns:  envInt("AI_MAX_TURNS", 100),
		SkillsDir: skillsDir,
	}
	jobs := job.New(repo.NewSAST(gdb), repo.NewModelConfig(gdb), mc, aiParams, cfg.Server.WorkRoot, cfg.LLM.Model)

	r := handler.Router(&handler.Deps{
		DB:         gdb,
		Repo:       repo.NewSAST(gdb),
		Store:      mc,
		Jobs:       jobs,
		LLMBaseURL: cfg.LLM.BaseURL,
		LLMAPIKey:  cfg.LLM.APIKey,
	})

	addr := ":" + cfg.Server.Port
	log.Printf("AgentSAST listening on %s (model=%s)", addr, cfg.LLM.Model)
	if err := r.Run(addr); err != nil {
		log.Fatalf("http: %v", err)
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
