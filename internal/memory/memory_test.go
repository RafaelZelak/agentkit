package memory

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	pq "github.com/lib/pq"
)

func TestSaveEmbeddedMessagePersistsAgentName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := NewStoreWithDB(db, Config{Schema: "test_schema", AgentName: "support-agent"})
	mock.ExpectQuery(`INSERT INTO test_schema\.chat_memory \(session_id, role, text, agent_name, embedding\) VALUES \(\$1,\$2,\$3,\$4,NULL\) RETURNING id`).
		WithArgs("session-1", "user", "hello", "support-agent").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	id, err := store.SaveEmbeddedMessage(context.Background(), "session-1", "user", "hello", nil)
	if err != nil {
		t.Fatalf("SaveEmbeddedMessage returned error: %v", err)
	}
	if id != 7 {
		t.Fatalf("expected id 7, got %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSaveEmbeddedMessageFallsBackWhenAgentNameColumnMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := NewStoreWithDB(db, Config{Schema: "test_schema", AgentName: "support-agent"})
	mock.ExpectQuery(`INSERT INTO test_schema\.chat_memory \(session_id, role, text, agent_name, embedding\) VALUES \(\$1,\$2,\$3,\$4,NULL\) RETURNING id`).
		WithArgs("session-1", "assistant", "hello back", "support-agent").
		WillReturnError(&pq.Error{Code: "42703", Message: `column "agent_name" of relation "chat_memory" does not exist`})
	mock.ExpectQuery(`INSERT INTO test_schema\.chat_memory \(session_id, role, text, embedding\) VALUES \(\$1,\$2,\$3,NULL\) RETURNING id`).
		WithArgs("session-1", "assistant", "hello back").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))

	id, err := store.SaveEmbeddedMessage(context.Background(), "session-1", "assistant", "hello back", nil)
	if err != nil {
		t.Fatalf("SaveEmbeddedMessage returned error: %v", err)
	}
	if id != 8 {
		t.Fatalf("expected id 8, got %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
