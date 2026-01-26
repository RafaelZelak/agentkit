package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RafaelZelak/agentkit/internal/functions"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"

	"golang.org/x/sync/errgroup"
)

type FunctionCall struct {
	Template string `json:"template"`
	Result   string `json:"result"`
}

type runVerbose struct {
	ToolRequested string        `json:"tool_requested,omitempty"`
	ToolArgs      []string      `json:"tool_args,omitempty"`
	ToolOutput    string        `json:"tool_output,omitempty"`
	Functions     []FunctionCall `json:"functions,omitempty"`
	Usage         *openai.Usage `json:"usage,omitempty"`
	FinalText     string        `json:"final_text"`
}

func (rv runVerbose) JSON() string {
	js, _ := json.MarshalIndent(rv, "", "  ")
	return string(js)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func buildMemBlock(recent, similar []memory.HistoryItem, boleto map[string]string) string {
	var sb strings.Builder

	if len(recent) > 0 {
		sb.WriteString("== Memória curta (últimas mensagens) ==\n")
		for _, h := range recent {
			sb.WriteString(h.Role)
			sb.WriteString(": ")
			sb.WriteString(h.Text)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}

	if len(boleto) > 0 {
		sb.WriteString("== Estado estruturado (faturas) ==\n")
		sb.WriteString("Estes são estados conhecidos anteriormente. Se o usuário perguntar sobre o status atual, você DEVE consultar a tool novamente para garantir dados atualizados.\n")
		for id, st := range boleto {
			sb.WriteString("- Fatura ")
			sb.WriteString(id)
			sb.WriteString(": ")
			sb.WriteString(st)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}

	if len(similar) > 0 {
		sb.WriteString("== Memória semântica relevante ==\n")
		for _, h := range similar {
			sb.WriteString(h.Role)
			sb.WriteString(": ")
			sb.WriteString(h.Text)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

func extractToolCommand(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.Trim(s, "\"")

	if strings.HasPrefix(s, "TOOL:") {
		return s, true
	}

	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		t = strings.Trim(t, "`")
		t = strings.Trim(t, "\"")
		if strings.HasPrefix(t, "TOOL:") {
			return t, true
		}
	}
	return "", false
}

func Run(
	ctx context.Context,
	cli *openai.Client,
	model string,
	embeddingModel string,
	sessionID string,
	promptPath string,
	userMessage string,
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

	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		return "", nil, fmt.Errorf("read prompt: %w", err)
	}
	longPromptRaw := string(promptBytes)
	
	// Process template functions in the prompt
	longPrompt, baseFunctions, err := functions.ProcessTemplateWithTracking(longPromptRaw)
	if err != nil {
		// Log error but continue with original prompt
		longPrompt = longPromptRaw
		baseFunctions = make(map[string]string)
	}

	semTopK := envInt("MEM_SEM_TOPK", 5)
	memDepth := envInt("MEM_DEPTH", 4)

	var (
		userEmb []float32
		similar []memory.HistoryItem
		recent  []memory.HistoryItem
		faturas map[string]string
	)
	eg, egctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		emb, err := cli.Embed(egctx, embeddingModel, userMessage)
		if err != nil {
			return err
		}
		userEmb = emb
		return nil
	})
	eg.Go(func() error {
		items, err := mem.RetrieveRecent(egctx, sessionID, memDepth)
		if err == nil {
			recent = items
		}
		return nil
	})
	eg.Go(func() error {
		m, err := mem.LoadBoletoStatus(egctx, sessionID)
		if err == nil {
			faturas = m
		}
		return nil
	})

	_ = eg.Wait()

	if len(userEmb) > 0 {
		if items, err := mem.RetrieveSimilar(ctx, sessionID, userEmb, semTopK); err == nil {
			similar = items
		}
	}

	memBlock := buildMemBlock(recent, similar, faturas)

	b := newBuilder()
	// Merge base prompt functions into builder's tracking
	for k, v := range baseFunctions {
		b.functionsUsed[k] = v
	}
	
	WithCachedContext(longPrompt)(b)
	if memBlock != "" {
		WithSystemPrompt(memBlock)(b)
	}
	for _, opt := range opts {
		opt(b)
	}
	b.user = openai.ContentItem{Type: "text", Text: userMessage}

	req := b.req(model)
	resp, err := cli.Respond(ctx, req)
	if err != nil {
		return "", nil, err
	}

	originalOut := strings.TrimSpace(resp.OutputText)

	toolLine, hasTool := extractToolCommand(originalOut)

	// Convert map to slice of FunctionCall
	functionsList := make([]FunctionCall, 0, len(b.functionsUsed))
	for template, result := range b.functionsUsed {
		functionsList = append(functionsList, FunctionCall{
			Template: template,
			Result:   result,
		})
	}
	
	rv := runVerbose{
		FinalText: originalOut,
		Usage:     &resp.Usage,
		Functions: functionsList,
	}
	
	// Only include functions in verbose mode if there are any
	if len(rv.Functions) == 0 {
		rv.Functions = nil
	}

	if hasTool {
		parts := strings.Fields(toolLine)
		if len(parts) >= 1 {
			var toolName string
			var args []string

			// Handle "TOOL:toolname" vs "TOOL: toolname"
			if parts[0] == "TOOL:" {
				if len(parts) >= 2 {
					toolName = parts[1]
					args = parts[2:]
				}
			} else {
				toolName = strings.TrimPrefix(parts[0], "TOOL:")
				args = parts[1:]
			}

			rv.ToolRequested = toolName
			rv.ToolArgs = args

			if toolName != "" {
				if tc := toolReg.Get(toolName); tc != nil {
					var toolOut string

					switch tc.Type {
					case "postgres":
						anyArgs := make([]any, len(args))
						for i, v := range args {
							anyArgs[i] = v
						}
						toolOut, err = tools.ExecPostgres(ctx, *tc, anyArgs...)

					case "postgres_embedding":
						var query string
						if len(args) == 0 {
							query = userMessage
						} else {
							query = strings.Join(args, " ")
						}
						toolOut, err = tools.ExecPostgresEmbedding(ctx, cli, *tc, query)

					case "script":
						toolOut, err = tools.ExecScript(*tc, args...)

					default:
						toolOut = "Tool type não suportado ainda"
					}

					if err != nil {
						toolOut = "Erro ao executar tool " + toolName + ": " + err.Error()
					}
					rv.ToolOutput = toolOut

					b2 := newBuilder()
					// Merge base prompt functions into builder's tracking
					for k, v := range baseFunctions {
						b2.functionsUsed[k] = v
					}
					WithCachedContext(longPrompt)(b2)
					if memBlock != "" {
						WithSystemPrompt(memBlock)(b2)
					}
					for _, opt := range opts {
						opt(b2)
					}
					WithSystemPrompt("O resultado da tool '" + toolName + "' foi:\n" + toolOut + "\nVocê DEVE usar essa informação para responder o usuário.")(b2)
					b2.user = openai.ContentItem{Type: "text", Text: userMessage}
					
					// Update functions tracking from b2 - merge into existing map
					for k, v := range b2.functionsUsed {
						b.functionsUsed[k] = v
					}

					req2 := b2.req(model)
					resp, err = cli.Respond(ctx, req2)
					if err != nil {
						return "", nil, err
					}
					
					// Convert updated map to slice of FunctionCall
					functionsList = make([]FunctionCall, 0, len(b.functionsUsed))
					for template, result := range b.functionsUsed {
						functionsList = append(functionsList, FunctionCall{
							Template: template,
							Result:   result,
						})
					}
					
					rv.FinalText = resp.OutputText
					rv.Usage = &resp.Usage
					rv.Functions = functionsList
					if len(rv.Functions) == 0 {
						rv.Functions = nil
					}
				} else {
					rv.ToolOutput = "Tool não encontrada: " + toolName
				}
			}
		}
	}

	saveEg, saveCtx := errgroup.WithContext(context.Background())
	saveEg.Go(func() error {
		_, err := mem.SaveEmbeddedMessage(saveCtx, sessionID, "user", userMessage, userEmb)
		return err
	})
	saveEg.Go(func() error {
		var assistEmb []float32
		if emb, err := cli.Embed(saveCtx, embeddingModel, originalOut); err == nil {
			assistEmb = emb
		}
		id, err := mem.SaveEmbeddedMessage(saveCtx, sessionID, "assistant", originalOut, assistEmb)
		if err != nil {
			return err
		}
		if resp.Raw != nil {
			_ = mem.SaveMetadata(saveCtx, id, "response_raw", resp.Raw)
		}
		if rv.ToolRequested != "" {
			_ = mem.SaveMetadata(saveCtx, id, "tool_used", map[string]any{
				"tool":   rv.ToolRequested,
				"args":   rv.ToolArgs,
				"output": rv.ToolOutput,
			})
		}
		return nil
	})
	if err := saveEg.Wait(); err != nil {
		return "", nil, fmt.Errorf("persist failed: %w", err)
	}

	if verbose {
		return rv.JSON(), rv.Usage, nil
	}
	return rv.FinalText, rv.Usage, nil
}
