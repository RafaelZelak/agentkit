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

	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
)

func envIntR(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

/*
RouteAndRun:
  - Igual ao fluxo normal, mas o roteamento também usa memória híbrida (recentes + semântica + fatos).
*/
func RouteAndRun(
	ctx context.Context,
	cli *openai.Client,
	model string,
	embeddingModel string,
	sessionID string,
	basePromptPath string,
	userMessage string,
	routerPath string,
	verbose bool,
	opts ...Option,
) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	vr := routeVerbose{
		RouterEnabled: routerPath != "",
		RouterPath:    routerPath,
		BasePrompt:    basePromptPath,
		UserMessage:   userMessage,
	}

	if routerPath == "" {
		return Run(ctx, cli, model, embeddingModel, sessionID, basePromptPath, userMessage, verbose, opts...)
	}

	// Carregar router.md
	routerBytes, err := os.ReadFile(routerPath)
	if err != nil {
		return "", fmt.Errorf("read router: %w", err)
	}
	routerPrompt := string(routerBytes)

	dir := filepath.Dir(routerPath)
	cands, err := listPromptCandidates(dir)
	if err != nil {
		return "", err
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no candidates in %s", dir)
	}
	vr.Candidates = append(vr.Candidates, cands...)

	// Recuperar memória híbrida
	mem := memory.Get()
	semTopK := envIntR("MEM_SEM_TOPK", 5)
	memDepth := envIntR("MEM_DEPTH", 4)

	var (
		retrieved []memory.HistoryItem
		recent    []memory.HistoryItem
		faturas   map[string]string
	)

	// Similaridade
	if emb, errEmb := cli.Embed(ctx, embeddingModel, userMessage); errEmb == nil {
		if items, err := mem.RetrieveSimilar(ctx, sessionID, emb, semTopK); err == nil {
			retrieved = items
		}
	}

	// Últimas N mensagens
	if items, err := mem.RetrieveRecent(ctx, sessionID, memDepth); err == nil {
		recent = items
	}

	// Fatos estruturados (status boletos)
	if m, err := mem.LoadBoletoStatus(ctx, sessionID); err == nil {
		faturas = m
	}

	// Montar bloco de memória
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

	// Perguntar ao router qual prompt usar
	chosen, raw, err := askRouter(ctx, cli, model, routerPrompt, routerInput, cands)
	vr.RouterRaw = raw
	if err != nil {
		vr.RouterError = err.Error()
		chosen = fallbackCandidate(cands, "geral.md")
	}
	vr.Chosen = chosen
	vr.SpecialPrompt = filepath.Join(dir, chosen)

	specBytes, err := os.ReadFile(vr.SpecialPrompt)
	if err != nil {
		vr.RouterError = "chosen prompt not found: " + err.Error()
		chosen = fallbackCandidate(cands, "geral.md")
		vr.Chosen = chosen
		vr.SpecialPrompt = filepath.Join(dir, chosen)
		specBytes, err = os.ReadFile(vr.SpecialPrompt)
		if err != nil {
			return "", fmt.Errorf("read chosen prompt: %w", err)
		}
	}
	specPrompt := string(specBytes)

	// Rodar com o prompt especializado
	// Rodar com o prompt especializado
	runOut, err := Run(ctx, cli, model, embeddingModel, sessionID, basePromptPath, userMessage, verbose, append(opts, WithSystemPrompt(specPrompt))...)
	if err != nil {
		return "", err
	}
	vr.FinalText = runOut

	if verbose {
		// tentar decodificar o resultado do Run
		var rv runVerbose
		_ = json.Unmarshal([]byte(runOut), &rv)

		type merged struct {
			RouterEnabled bool     `json:"router_enabled"`
			RouterPath    string   `json:"router_path,omitempty"`
			BasePrompt    string   `json:"base_prompt"`
			UserMessage   string   `json:"user_message"`
			Candidates    []string `json:"candidates,omitempty"`
			RouterRaw     string   `json:"router_raw,omitempty"`
			RouterError   string   `json:"router_error,omitempty"`
			Chosen        string   `json:"chosen,omitempty"`
			SpecialPrompt string   `json:"special_prompt,omitempty"`
			ToolRequested string   `json:"tool_requested,omitempty"`
			ToolArgs      []string `json:"tool_args,omitempty"`
			ToolOutput    string   `json:"tool_output,omitempty"`
			FinalText     string   `json:"final_text"`
		}
		out := merged{
			RouterEnabled: vr.RouterEnabled,
			RouterPath:    vr.RouterPath,
			BasePrompt:    vr.BasePrompt,
			UserMessage:   vr.UserMessage,
			Candidates:    vr.Candidates,
			RouterRaw:     vr.RouterRaw,
			RouterError:   vr.RouterError,
			Chosen:        vr.Chosen,
			SpecialPrompt: vr.SpecialPrompt,
			FinalText:     vr.FinalText,
		}
		if rv.FinalText != "" || rv.ToolRequested != "" {
			out.ToolRequested = rv.ToolRequested
			out.ToolArgs = rv.ToolArgs
			out.ToolOutput = rv.ToolOutput
			out.FinalText = rv.FinalText
		}
		js, _ := json.MarshalIndent(out, "", "  ")
		return string(js), nil
	}
	return runOut, nil
}

type routeVerbose struct {
	RouterEnabled bool     `json:"router_enabled"`
	RouterPath    string   `json:"router_path,omitempty"`
	BasePrompt    string   `json:"base_prompt"`
	UserMessage   string   `json:"user_message"`
	Candidates    []string `json:"candidates,omitempty"`
	RouterRaw     string   `json:"router_raw,omitempty"`
	RouterError   string   `json:"router_error,omitempty"`
	Chosen        string   `json:"chosen,omitempty"`
	SpecialPrompt string   `json:"special_prompt,omitempty"`
	FinalText     string   `json:"final_text"`
}

func (r routeVerbose) JSON() string {
	js, _ := json.MarshalIndent(r, "", "  ")
	return string(js)
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
			{Type: "input_text", Text: sb.String()},
		},
	}
	user := openai.Message{
		Type: "message",
		Role: "user",
		Content: []openai.ContentItem{
			{Type: "input_text", Text: userMessage},
		},
	}

	req := &openai.ResponsesRequest{
		Model:           model,
		Input:           []openai.Message{sys, user},
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
