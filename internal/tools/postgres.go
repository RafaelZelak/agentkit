package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/RafaelZelak/agentkit/internal/openai"

	_ "github.com/lib/pq"
)

func ExecPostgres(ctx context.Context, cfg ToolConfig, args ...any) (string, error) {
	db, err := sql.Open("postgres", cfg.Conn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, cfg.QueryTemplate, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	results := ""
	for rows.Next() {
		colsData := make([]any, len(cols))
		colsPtrs := make([]any, len(cols))
		for i := range cols {
			colsPtrs[i] = &colsData[i]
		}
		if err := rows.Scan(colsPtrs...); err != nil {
			return "", err
		}
		row := ""
		for i, c := range cols {
			row += fmt.Sprintf("%s=%v ", c, colsData[i])
		}
		results += row + "\n"
	}
	if results == "" {
		results = "Nenhum resultado encontrado."
	}
	return results, nil
}

func ExecPostgresEmbedding(ctx context.Context, cli *openai.Client, cfg ToolConfig, query string) (string, error) {
	if cfg.Table == "" || cfg.Column == "" || cfg.EmbeddingModel == "" {
		return "", fmt.Errorf("tool %s mal configurada: table/column/embedding_model obrigatórios", cfg.Name)
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}

	emb, err := cli.Embed(ctx, cfg.EmbeddingModel, query)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	vec := encodeVector(emb)

	db, err := sql.Open("postgres", cfg.Conn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	sqlQuery := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1::vector
		LIMIT %d
	`, cfg.Column, cfg.Table, cfg.TopK)

	rows, err := db.QueryContext(ctx, sqlQuery, vec)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return "", err
		}
		results = append(results, content)
	}

	if len(results) == 0 {
		return "Nenhum resultado encontrado.", nil
	}

	return strings.Join(results, "\n---\n"), nil
}

func encodeVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%f", x))
	}
	sb.WriteByte(']')
	return sb.String()
}
