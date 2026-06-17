package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Orchestrator drives the SAST chat-completions tool loop.
type Orchestrator struct {
	cfg       Config
	client    *openai.Client
	exec      *Executor
	skillText string
}

// NewOrchestrator builds an orchestrator from a config and tool executor.
func NewOrchestrator(cfg Config, exec *Executor) *Orchestrator {
	cfg = cfg.WithDefaults()
	skillText := ""
	if exec != nil && exec.skills != nil {
		skillText = exec.skills.skillText()
	}
	return &Orchestrator{
		cfg:       cfg,
		client:    newClient(cfg.BaseURL, cfg.APIKey),
		exec:      exec,
		skillText: skillText,
	}
}

// Result carries the outcome of a run.
type Result struct {
	BaseURL string
	Turns   int
}

// Run executes the agentic loop, writing sast/{openapi.yaml,report.md,base_url.txt}
// into the workdir. onEmit is called per tool call so the caller can heartbeat.
func (o *Orchestrator) Run(ctx context.Context, onEmit func(string)) (*Result, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt(o.skillText)},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt(o.cfg)},
	}
	tools := toolDefinitions()
	turns := 0
	nudged := false
	validateNudged := false

	for turn := 0; turn < o.cfg.MaxTurns; turn++ {
		turns = turn + 1
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    o.cfg.Model,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			if turn == 0 {
				return nil, fmt.Errorf("chat completion failed (check LLM_BASE_URL/LLM_MODEL/LLM_API_KEY and tool-calling support): %w", err)
			}
			log.Printf("[ai] model error on turn %d; finalizing with backstop: %v", turn+1, err)
			o.finalizeBackstop(ctx, messages)
			return o.result(turns), nil
		}
		if len(resp.Choices) == 0 {
			if turn == 0 {
				return nil, fmt.Errorf("model returned no choices")
			}
			break
		}
		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			// Model is concluding. Gate on the three artifacts + a valid OpenAPI.
			if !o.artifactsComplete() {
				if !nudged {
					nudged = true
					messages = append(messages, userMsg("You have not written all three deliverables yet (sast/openapi.yaml, sast/report.md, sast/base_url.txt). Use write_artifact to finish them now, then validate_openapi."))
					continue
				}
				o.finalizeBackstop(ctx, messages)
				return o.result(turns), nil
			}
			if verr := ValidateOpenAPIFile(o.artifactPath("openapi.yaml")); verr != nil && !validateNudged {
				validateNudged = true
				messages = append(messages, userMsg("sast/openapi.yaml is invalid: "+verr.Error()+"\nFix it with write_artifact and call validate_openapi until it returns valid."))
				continue
			}
			return o.result(turns), nil
		}

		for _, tc := range msg.ToolCalls {
			if onEmit != nil {
				onEmit(tc.Function.Name + " " + truncate(tc.Function.Arguments, 160))
			}
			content := o.dispatch(ctx, tc)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    content,
			})
		}
	}

	// Turn cap reached (or empty-choice break): backstop anything still missing.
	if !o.artifactsComplete() {
		log.Printf("[ai] reached max turns (%d); writing backstop artifacts", o.cfg.MaxTurns)
		o.finalizeBackstop(ctx, messages)
	}
	return o.result(turns), nil
}

// dispatch runs a single tool call and returns its textual result for the model.
func (o *Orchestrator) dispatch(ctx context.Context, tc openai.ToolCall) string {
	args := json.RawMessage(tc.Function.Arguments)
	switch tc.Function.Name {
	case fnExtractArchive:
		return o.exec.ExtractArchive(args)
	case fnListFiles:
		return o.exec.ListFiles(args)
	case fnReadFile:
		return o.exec.ReadFile(args)
	case fnSearchCode:
		return o.exec.SearchCode(args)
	case fnGetKnowledge:
		var in struct {
			Topic string `json:"topic"`
		}
		_ = json.Unmarshal(args, &in)
		return o.exec.Knowledge(in.Topic)
	case fnWriteArtifact:
		return o.exec.WriteArtifact(args)
	case fnValidateOpenAPI:
		return o.exec.ValidateOpenAPI(args)
	default:
		return "unknown tool: " + tc.Function.Name
	}
}

// synthesize asks the model, tools disabled, to emit the final report.md body.
func (o *Orchestrator) synthesize(ctx context.Context, messages []openai.ChatCompletionMessage) string {
	messages = append(messages, userMsg("Stop using tools. Using everything you have read above, output the FINAL sast/report.md content in markdown now: per-endpoint attack surface, candidate vulnerabilities with file:line evidence, OWASP/CWE mappings, concrete test payloads, and a prioritized summary table. Output ONLY the markdown body."))
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: o.cfg.Model, Messages: messages})
	if err != nil || len(resp.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

// finalizeBackstop guarantees the three artifacts exist so the job never fails
// for lack of output, even if the model degrades.
func (o *Orchestrator) finalizeBackstop(ctx context.Context, messages []openai.ChatCompletionMessage) {
	sast := filepath.Join(o.cfg.WorkDir, "sast")
	_ = os.MkdirAll(sast, 0o755)
	if !nonEmptyFile(o.artifactPath("report.md")) {
		narrative := o.synthesize(ctx, messages)
		if strings.TrimSpace(narrative) == "" {
			narrative = "# SAST Report\n\n_The model did not produce a narrative report._\n"
		}
		_ = os.WriteFile(o.artifactPath("report.md"), []byte(narrative), 0o644)
	}
	if !nonEmptyFile(o.artifactPath("openapi.yaml")) {
		_ = os.WriteFile(o.artifactPath("openapi.yaml"), []byte(stubOpenAPI(o.cfg.BaseURLHint)), 0o644)
	}
	if !nonEmptyFile(o.artifactPath("base_url.txt")) {
		_ = os.WriteFile(o.artifactPath("base_url.txt"), []byte(strings.TrimSpace(o.cfg.BaseURLHint)), 0o644)
	}
}

func (o *Orchestrator) artifactPath(name string) string {
	return filepath.Join(o.cfg.WorkDir, "sast", name)
}

func (o *Orchestrator) artifactsComplete() bool {
	return nonEmptyFile(o.artifactPath("openapi.yaml")) &&
		nonEmptyFile(o.artifactPath("report.md")) &&
		nonEmptyFile(o.artifactPath("base_url.txt"))
}

func (o *Orchestrator) result(turns int) *Result {
	base := strings.TrimSpace(o.cfg.BaseURLHint)
	if b, err := os.ReadFile(o.artifactPath("base_url.txt")); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			base = v
		}
	}
	return &Result{BaseURL: base, Turns: turns}
}

func userMsg(content string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: content}
}

func nonEmptyFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// stubOpenAPI returns a minimal valid OpenAPI 3 doc used only as a backstop.
func stubOpenAPI(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "/"
	}
	return fmt.Sprintf(`openapi: 3.0.3
info:
  title: SAST (backstop)
  version: 0.0.0
  description: Automated analysis did not produce a full spec; this is a placeholder.
servers:
  - url: %q
paths: {}
`, baseURL)
}
