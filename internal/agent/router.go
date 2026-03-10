package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RafaelZelak/agentkit/internal/functions"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"
)

func envIntR(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type PromptInfo struct {
	RouterEnabled bool     `json:"router_enabled"`
	BasePrompt    string   `json:"base_prompt"`
	UserMessage   string   `json:"user_message"`
	SessionID     string   `json:"session_id"`
	Candidates    []string `json:"candidates,omitempty"`
	ChosenPrompt  string   `json:"chosen_prompt,omitempty"`
	RouterError   string   `json:"router_error,omitempty"`
}

type ToolInfo struct {
	ToolRequested string   `json:"tool_requested"`
	ToolArgs      []string `json:"tool_args"`
	ToolOutput    string   `json:"tool_output"`
}

type Response struct {
	Prompt    PromptInfo     `json:"prompt"`
	Tools     []ToolInfo     `json:"tools,omitempty"`
	Functions []FunctionCall `json:"functions,omitempty"`
	Usage     *openai.Usage  `json:"usage,omitempty"`
	FinalText string         `json:"final_text"`
}

func (r Response) JSON() string {
	js, _ := json.MarshalIndent(r, "", "  ")
	return string(js)
}

func RouteAndRun(
	ctx context.Context,
	cli *openai.Client,
	model string,
	embeddingModel string,
	sessionID string,
	promptsDir string,
	basePromptPath string,
	userMessage string,
	routerPath string,
	verbose bool,
	mem *memory.Store,
	toolReg *tools.Registry,
	opts ...Option,
) (string, *openai.Usage, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	resp := Response{
		Prompt: PromptInfo{
			RouterEnabled: routerPath != "",
			BasePrompt:    basePromptPath,
			UserMessage:   userMessage,
			SessionID:     sessionID,
		},
	}

	if routerPath == "" {

		runOut, usage, err := Run(ctx, cli, model, embeddingModel, sessionID, basePromptPath, userMessage, true, mem, toolReg, opts...)
		if err != nil {
			return "", nil, err
		}

		var rv runVerbose
		if json.Unmarshal([]byte(runOut), &rv) == nil {
			resp.FinalText = rv.FinalText
			resp.Usage = rv.Usage
			if rv.ToolRequested != "" {
				resp.Tools = []ToolInfo{{
					ToolRequested: rv.ToolRequested,
					ToolArgs:      rv.ToolArgs,
					ToolOutput:    rv.ToolOutput,
				}}
			}
			if len(rv.Functions) > 0 {
				resp.Functions = rv.Functions
			}
		} else {
			resp.FinalText = runOut
			resp.Usage = usage
		}

		if verbose {
			return resp.JSON(), resp.Usage, nil
		}
		return resp.FinalText, resp.Usage, nil
	}

	routerPromptRaw, err := loadPrompt(ctx, mem, routerPath)
	if err != nil {
		return "", nil, fmt.Errorf("read router: %w", err)
	}

	// Process template functions in the router prompt
	routerPrompt, err := functions.ProcessTemplate(routerPromptRaw)
	if err != nil {
		// On error, use original prompt
		routerPrompt = routerPromptRaw
	}

	var cands []string
	var dbPrompts map[string]string

	if strings.HasPrefix(promptsDir, "db:") {
		query := strings.TrimSpace(strings.TrimPrefix(promptsDir, "db:"))
		dbPrompts, err = mem.QueryMap(ctx, query)
		if err != nil {
			return "", nil, fmt.Errorf("failed to load candidates from db: %w", err)
		}
		if len(dbPrompts) == 0 {
			return "", nil, fmt.Errorf("no candidates returned by query: %s", query)
		}
		for k := range dbPrompts {
			cands = append(cands, k)
		}
		sort.Strings(cands)
	} else {
		if promptsDir != "" {
			cands, err = listPromptCandidates(promptsDir)
			if err != nil {
				return "", nil, err
			}
		}
	}

	if len(cands) == 0 {
		return "", nil, errors.New("no prompt candidates available for routing")
	}
	resp.Prompt.Candidates = append(resp.Prompt.Candidates, cands...)

	semTopK := envIntR("MEM_SEM_TOPK", 5)
	memDepth := envIntR("MEM_DEPTH", 4)

	var (
		retrieved []memory.HistoryItem
		recent    []memory.HistoryItem
		faturas   map[string]string
	)

	if emb, errEmb := cli.Embed(ctx, embeddingModel, userMessage); errEmb == nil {
		if items, err := mem.RetrieveSimilar(ctx, sessionID, emb, semTopK); err == nil {
			retrieved = items
		}
	}

	if items, err := mem.RetrieveRecent(ctx, sessionID, memDepth); err == nil {
		recent = items
	}

	if m, err := mem.LoadBoletoStatus(ctx, sessionID); err == nil {
		faturas = m
	}

	var sb strings.Builder
	if len(recent) > 0 {
		sb.WriteString("== Memória curta ==\n")
		for _, h := range recent {
			sb.WriteString(h.Role + ": " + h.Text + "\n")
		}
	}
	if len(faturas) > 0 {
		sb.WriteString("\n== Fatos: faturas ==\n")
		for id, st := range faturas {
			sb.WriteString("- " + id + ": " + st + "\n")
		}
	}
	if len(retrieved) > 0 {
		sb.WriteString("\n== Semântica relevante ==\n")
		for _, h := range retrieved {
			sb.WriteString(h.Role + ": " + h.Text + "\n")
		}
	}
	memBlock := sb.String()

	routerInput := userMessage
	if memBlock != "" {
		routerInput = memBlock + "\nUsuário agora: " + userMessage
	}

	chosen, _, err := askRouter(ctx, cli, model, routerPrompt, routerInput, cands)
	if err != nil {
		resp.Prompt.RouterError = err.Error()
		chosen = fallbackCandidate(cands, "geral.md")
	}
	resp.Prompt.ChosenPrompt = chosen

	var specPrompt string

	if len(dbPrompts) > 0 {
		// Loaded from DB map
		if val, exists := dbPrompts[chosen]; exists {
			specPrompt = val
		} else {
			resp.Prompt.RouterError = "chosen prompt not found in db result"
			chosen = fallbackCandidate(cands, "geral.md")
			resp.Prompt.ChosenPrompt = chosen
			specPrompt = dbPrompts[chosen]
		}
	} else {
		// Loaded from Disk
		pathPrefix := filepath.Join(promptsDir, chosen)
		specPrompt, err = loadPrompt(ctx, mem, pathPrefix)
		if err != nil {
			resp.Prompt.RouterError = "chosen prompt not found: " + err.Error()
			chosen = fallbackCandidate(cands, "geral.md")
			resp.Prompt.ChosenPrompt = chosen

			pathPrefix = filepath.Join(promptsDir, chosen)
			specPrompt, err = loadPrompt(ctx, mem, pathPrefix)
			if err != nil {
				return "", nil, fmt.Errorf("read chosen prompt: %w", err)
			}
		}
	}

	runOut, usage, err := Run(ctx, cli, model, embeddingModel, sessionID, basePromptPath, userMessage, true, mem, toolReg, append(opts, WithSystemPrompt(specPrompt))...)
	if err != nil {
		return "", nil, err
	}

	var rv runVerbose
	if json.Unmarshal([]byte(runOut), &rv) == nil {
		resp.FinalText = rv.FinalText
		resp.Usage = rv.Usage
		if rv.ToolRequested != "" {
			resp.Tools = []ToolInfo{{
				ToolRequested: rv.ToolRequested,
				ToolArgs:      rv.ToolArgs,
				ToolOutput:    rv.ToolOutput,
			}}
		}
		if len(rv.Functions) > 0 {
			resp.Functions = rv.Functions
		}
	} else {
		resp.FinalText = runOut
		resp.Usage = usage
	}

	if verbose {
		return resp.JSON(), resp.Usage, nil
	}
	return resp.FinalText, resp.Usage, nil
}

func listPromptCandidates(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list router dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasSuffix(low, ".md") {
			continue
		}
		if low == "router.md" {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}

func normalizeChoice(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '\r'); i >= 0 {
		s = s[:i]
	}
	parts := strings.Fields(s)
	if len(parts) > 0 {
		s = parts[0]
	}
	s = strings.Trim(s, ".,;:!?)('\"`”’“‘")
	return s
}

func matchCandidate(raw string, candidates []string) (string, bool) {
	raw = normalizeChoice(raw)
	withMD := raw
	if !strings.HasSuffix(withMD, ".md") {
		withMD = raw + ".md"
	}
	for _, c := range candidates {
		if strings.EqualFold(c, withMD) {
			return c, true
		}
	}
	baseRaw := strings.TrimSuffix(withMD, ".md")
	for _, c := range candidates {
		if strings.EqualFold(strings.TrimSuffix(c, ".md"), baseRaw) {
			return c, true
		}
	}
	return "", false
}

func fallbackCandidate(candidates []string, prefer string) string {
	for _, c := range candidates {
		if strings.EqualFold(c, prefer) {
			return c
		}
	}
	return candidates[0]
}

func askRouter(
	ctx context.Context,
	cli *openai.Client,
	model string,
	routerPrompt string,
	userMessage string,
	candidates []string,
) (chosen string, raw string, err error) {
	var sb strings.Builder
	sb.WriteString(routerPrompt)
	sb.WriteString("\n\n== Regras de roteamento ==\n")
	sb.WriteString("Escolha exatamente UM dos seguintes arquivos de prompt e responda SOMENTE com o nome do arquivo.\n")
	sb.WriteString("Opções permitidas:\n")
	for _, c := range candidates {
		sb.WriteString("- ")
		sb.WriteString(c)
		sb.WriteByte('\n')
	}
	sb.WriteString("\nFormato de saída: apenas o nome do arquivo (ex.: tecnico.md). Não inclua explicações.\n")

	sys := openai.Message{
		Type: "message",
		Role: "system",
		Content: []openai.ContentItem{
			{Type: "text", Text: sb.String()},
		},
	}
	user := openai.Message{
		Type: "message",
		Role: "user",
		Content: []openai.ContentItem{
			{Type: "text", Text: userMessage},
		},
	}

	req := &openai.ChatCompletionRequest{
		Model:           model,
		Messages:        []openai.Message{sys, user},
		MaxOutputTokens: 32,
	}

	resp, e := cli.Respond(ctx, req)
	if e != nil {
		return "", "", e
	}
	raw = resp.OutputText
	if sel, ok := matchCandidate(resp.OutputText, candidates); ok {
		return sel, raw, nil
	}
	return "", raw, errors.New("router returned an invalid option")
}
