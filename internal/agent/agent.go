package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"

	"golang.org/x/sync/errgroup"
)

type runVerbose struct {
	ToolRequested string   `json:"tool_requested,omitempty"`
	ToolArgs      []string `json:"tool_args,omitempty"`
	ToolOutput    string   `json:"tool_output,omitempty"`
	FinalText     string   `json:"final_text"`
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
		sb.WriteString("Use estes fatos como verdade a menos que o usuário informe atualização.\n")
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

/*
Run:
- Injeta o contexto fixo (arquivo) como system.
- Recupera memória HÍBRIDA (recentes + semântica) e fatos estruturados (boletos).
- Chama Responses API.
- Se resposta vier como TOOL: executa a tool e roda de novo.
- Se verbose==true, retorna JSON detalhado.
- Salva memória e metadata.
*/
func Run(
	ctx context.Context,
	cli *openai.Client,
	model string,
	embeddingModel string,
	sessionID string,
	promptPath string,
	userMessage string,
	verbose bool,
	opts ...Option,
) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("read prompt: %w", err)
	}
	longPrompt := string(promptBytes)

	mem := memory.Get()

	// Parâmetros de memória via ENV
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
		// fatos de boleto
		m, err := mem.LoadBoletoStatus(egctx, sessionID)
		if err == nil {
			faturas = m
		}
		return nil
	})

	_ = eg.Wait()

	// Agora que temos userEmb, buscar similar
	if len(userEmb) > 0 {
		if items, err := mem.RetrieveSimilar(ctx, sessionID, userEmb, semTopK); err == nil {
			similar = items
		}
	}

	memBlock := buildMemBlock(recent, similar, faturas)

	b := newBuilder()
	WithCachedContext(longPrompt)(b)
	if memBlock != "" {
		WithSystemPrompt(memBlock)(b)
	}
	for _, opt := range opts {
		opt(b)
	}
	b.user = openai.ContentItem{Type: "input_text", Text: userMessage}

	req := b.req(model)
	resp, err := cli.Respond(ctx, req)
	if err != nil {
		return "", err
	}

	rv := runVerbose{FinalText: resp.OutputText}
	if strings.HasPrefix(strings.TrimSpace(resp.OutputText), "TOOL:") {
		parts := strings.Fields(resp.OutputText)
		if len(parts) >= 1 {
			toolName := strings.TrimPrefix(parts[0], "TOOL:")
			args := parts[1:]
			rv.ToolRequested = toolName
			rv.ToolArgs = args

			if tc := tools.GetTool(toolName); tc != nil {
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

				default:
					toolOut = "Tool type não suportado ainda"
				}

				if err != nil {
					toolOut = "Erro ao executar tool " + toolName + ": " + err.Error()
				}
				rv.ToolOutput = toolOut

				// Quando reexecutar após tool:
				b2 := newBuilder()
				WithCachedContext(longPrompt)(b2)
				if memBlock != "" {
					WithSystemPrompt(memBlock)(b2)
				}
				// reaplica prompts extras (ex: financeiro.md)
				for _, opt := range opts {
					opt(b2)
				}
				WithSystemPrompt("O resultado da tool '" + toolName + "' foi:\n" + toolOut + "\nVocê DEVE usar essa informação para responder o usuário.")(b2)
				b2.user = openai.ContentItem{Type: "input_text", Text: userMessage}

				req2 := b2.req(model)
				resp, err = cli.Respond(ctx, req2)
				if err != nil {
					return "", err
				}
				rv.FinalText = resp.OutputText
			} else {
				rv.ToolOutput = "Tool não encontrada: " + toolName
			}
		}
	}

	// Persistir mensagens + metadata
	saveEg, saveCtx := errgroup.WithContext(context.Background())
	saveEg.Go(func() error {
		_, err := mem.SaveEmbeddedMessage(saveCtx, sessionID, "user", userMessage, userEmb)
		return err
	})
	saveEg.Go(func() error {
		var assistEmb []float32
		if emb, err := cli.Embed(saveCtx, embeddingModel, rv.FinalText); err == nil {
			assistEmb = emb
		}
		id, err := mem.SaveEmbeddedMessage(saveCtx, sessionID, "assistant", rv.FinalText, assistEmb)
		if err != nil {
			return err
		}
		if resp.Raw != nil {
			_ = mem.SaveMetadata(saveCtx, id, "response_raw", resp.Raw)
		}
		if rv.ToolRequested != "" {
			_ = mem.SaveMetadata(saveCtx, id, "tool_used", rv)
		}
		return nil
	})
	if err := saveEg.Wait(); err != nil {
		return "", fmt.Errorf("persist failed: %w", err)
	}

	if verbose {
		return rv.JSON(), nil
	}
	return rv.FinalText, nil
}
