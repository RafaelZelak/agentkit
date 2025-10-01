package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	_ "github.com/lib/pq"
)

type Config struct {
	DSN          string
	Schema       string
	EmbeddingDim int
}

type Store struct {
	db           *sql.DB
	schema       string
	embeddingDim int
}

type HistoryItem struct {
	Role string
	Text string
}

var (
	storeOnce sync.Once
	storeInst *Store
	storeErr  error
)

func Init(cfg Config) (*Store, error) {
	storeOnce.Do(func() {
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			storeErr = err
			return
		}
		s := &Store{
			db:           db,
			schema:       cfg.Schema,
			embeddingDim: cfg.EmbeddingDim,
		}
		if err := s.migrate(); err != nil {
			storeErr = err
			return
		}
		storeInst = s
	})
	return storeInst, storeErr
}

func Get() *Store {
	if storeInst == nil {
		panic("memory store not initialized: call memory.Init first")
	}
	return storeInst
}

func (s *Store) migrate() error {
	_, _ = s.db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)

	if _, err := s.db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, pqIdent(s.schema))); err != nil {
		return err
	}
	if _, err := s.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.chat_memory (
			id BIGSERIAL PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			text TEXT NOT NULL,
			embedding vector(%d),
			created_at TIMESTAMPTZ DEFAULT now()
		);`, pqIdent(s.schema), s.embeddingDim)); err != nil {
		return err
	}
	_, _ = s.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS chat_memory_session_idx ON %s.chat_memory (session_id)`, pqIdent(s.schema)))
	_, _ = s.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS chat_memory_embedding_idx ON %s.chat_memory USING ivfflat (embedding vector_cosine_ops) WITH (lists=100)`, pqIdent(s.schema)))

	if _, err := s.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.metadata (
			id BIGSERIAL PRIMARY KEY,
			message_id BIGINT NOT NULL REFERENCES %s.chat_memory(id) ON DELETE CASCADE,
			key TEXT NOT NULL,
			value JSONB NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		);`, pqIdent(s.schema), pqIdent(s.schema))); err != nil {
		return err
	}
	_, _ = s.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS metadata_message_idx ON %s.metadata(message_id)`, pqIdent(s.schema)))
	return nil
}

func (s *Store) SaveEmbeddedMessage(ctx context.Context, sessionID, role, text string, embedding []float32) (int64, error) {
	var id int64
	if len(embedding) == 0 {
		err := s.db.QueryRowContext(ctx,
			fmt.Sprintf(`INSERT INTO %s.chat_memory (session_id, role, text, embedding)
			 VALUES ($1,$2,$3,NULL) RETURNING id`, pqIdent(s.schema)),
			sessionID, role, text,
		).Scan(&id)
		return id, err
	}
	vec := encodeVector(embedding)
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`INSERT INTO %s.chat_memory (session_id, role, text, embedding)
		 VALUES ($1,$2,$3,$4::vector) RETURNING id`, pqIdent(s.schema)),
		sessionID, role, text, vec,
	).Scan(&id)
	return id, err
}

func (s *Store) SaveMetadata(ctx context.Context, messageID int64, key string, value any) error {
	js, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s.metadata (message_id, key, value) VALUES ($1,$2,$3::jsonb)`, pqIdent(s.schema)),
		messageID, key, string(js),
	)
	return err
}

func (s *Store) RetrieveSimilar(ctx context.Context, sessionID string, queryEmbedding []float32, topK int) ([]HistoryItem, error) {
	if topK <= 0 {
		topK = 5
	}
	vec := encodeVector(queryEmbedding)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT role, text
		FROM %s.chat_memory
		WHERE session_id=$1 AND embedding IS NOT NULL
		ORDER BY embedding <=> $2::vector
		LIMIT $3
	`, pqIdent(s.schema)), sessionID, vec, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryItem
	for rows.Next() {
		var h HistoryItem
		if err := rows.Scan(&h.Role, &h.Text); err == nil {
			out = append(out, h)
		}
	}
	return out, nil
}

func (s *Store) RetrieveRecent(ctx context.Context, sessionID string, depth int) ([]HistoryItem, error) {
	if depth <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT role, text
		FROM %s.chat_memory
		WHERE session_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, pqIdent(s.schema)), sessionID, depth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rev []HistoryItem
	for rows.Next() {
		var h HistoryItem
		if err := rows.Scan(&h.Role, &h.Text); err == nil {
			rev = append(rev, h)
		}
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}

type toolUsed struct {
	ToolRequested string   `json:"tool_requested"`
	ToolArgs      []string `json:"tool_args"`
	ToolOutput    string   `json:"tool_output"`
}

func (s *Store) LoadBoletoStatus(ctx context.Context, sessionID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT m.value
		FROM %s.metadata m
		JOIN %s.chat_memory c ON c.id = m.message_id
		WHERE c.session_id = $1
		  AND m.key = 'tool_used'
		ORDER BY c.created_at ASC, c.id ASC
	`, pqIdent(s.schema), pqIdent(s.schema)), sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statusByID := make(map[string]string)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var tu toolUsed
		if err := json.Unmarshal([]byte(raw), &tu); err != nil {
			continue
		}
		if strings.ToLower(tu.ToolRequested) != "db_boleto" || len(tu.ToolArgs) == 0 {
			continue
		}
		id := strings.TrimSpace(tu.ToolArgs[0])
		if id == "" {
			continue
		}
		if st := parseStatusFromToolOutput(tu.ToolOutput); st != "" {
			statusByID[id] = st
		}
	}
	return statusByID, nil
}

func parseStatusFromToolOutput(out string) string {
	out = strings.TrimSpace(out)
	idx := strings.Index(strings.ToLower(out), "status=")
	if idx < 0 {
		return ""
	}
	rest := out[idx+len("status="):]
	rest = strings.TrimSpace(rest)
	for i, r := range rest {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return strings.TrimSpace(rest[:i])
		}
	}
	return strings.TrimSpace(rest)
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
		sb.WriteString(trimFloat(float64(x)))
	}
	sb.WriteByte(']')
	return sb.String()
}

func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 32)
	if s == "NaN" || s == "+Inf" || s == "-Inf" {
		return "0"
	}
	return s
}

func pqIdent(s string) string {
	return s
}
