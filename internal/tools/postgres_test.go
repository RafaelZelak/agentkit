package tools

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func noResultsMessage() string {
	return string([]byte{
		78, 101, 110, 104, 117, 109, 32, 114, 101, 115, 117, 108, 116, 97, 100, 111, 32,
		101, 110, 99, 111, 110, 116, 114, 97, 100, 111, 46,
	})
}

func toAnyArgs(vals []driver.Value) []any {
	if vals == nil {
		return nil
	}
	out := make([]any, len(vals))
	for i := range vals {
		out[i] = vals[i]
	}
	return out
}

func TestExecPostgresWithDB_TableDriven(t *testing.T) {
	ctx := context.Background()

	type testCase struct {
		name    string
		cfg     ToolConfig
		args    []driver.Value
		rows    *sqlmock.Rows
		retErr  error
		wantOut string
		wantErr bool
	}

	cases := []testCase{
		{
			name: "single_row_format",
			cfg: ToolConfig{
				QueryTemplate: "SELECT id, name, status FROM users WHERE id = $1",
			},
			args: []driver.Value{int64(1)},
			rows: sqlmock.NewRows([]string{"id", "name", "status"}).
				AddRow(1, "Item 1", "active"),
			wantOut: "id=1 name=Item 1 status=active \n",
		},
		{
			name: "multiple_rows_format",
			cfg: ToolConfig{
				QueryTemplate: "SELECT id, name FROM items",
			},
			rows: sqlmock.NewRows([]string{"id", "name"}).
				AddRow(1, "Item 1").
				AddRow(2, "Item 2").
				AddRow(3, "Item 3"),
			wantOut: "" +
				"id=1 name=Item 1 \n" +
				"id=2 name=Item 2 \n" +
				"id=3 name=Item 3 \n",
		},
		{
			name: "no_results_default_message",
			cfg: ToolConfig{
				QueryTemplate: "SELECT id, name FROM items WHERE id = $1",
			},
			args:    []driver.Value{int64(999)},
			rows:    sqlmock.NewRows([]string{"id", "name"}),
			wantOut: noResultsMessage(),
		},
		{
			name: "with_multiple_args",
			cfg: ToolConfig{
				QueryTemplate: "SELECT COUNT(*) as count FROM orders WHERE user_id = $1 AND status = $2",
			},
			args: []driver.Value{int64(123), "completed"},
			rows: sqlmock.NewRows([]string{"count"}).
				AddRow(5),
			wantOut: "count=5 \n",
		},
		{
			name: "query_error",
			cfg: ToolConfig{
				QueryTemplate: "SELECT * FROM invalid_table",
			},
			retErr:  errors.New("query failed"),
			wantErr: true,
		},
		{
			name: "rows_err_is_returned",
			cfg: ToolConfig{
				QueryTemplate: "SELECT id FROM items",
			},
			rows: sqlmock.NewRows([]string{"id"}).
				AddRow(1).
				RowError(0, errors.New("row error")),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New error: %v", err)
			}
			defer db.Close()

			exp := mock.ExpectQuery(regexp.QuoteMeta(tc.cfg.QueryTemplate))
			if tc.args != nil {
				exp = exp.WithArgs(tc.args...)
			}

			if tc.retErr != nil {
				exp.WillReturnError(tc.retErr)
			} else {
				exp.WillReturnRows(tc.rows)
			}

			got, err := execPostgresWithDB(ctx, db, tc.cfg, toAnyArgs(tc.args)...)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if got != "" {
					t.Fatalf("expected empty output on error, got %q", got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.wantOut {
					t.Fatalf("unexpected output\nGot:  %q\nWant: %q", got, tc.wantOut)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestExecPostgres_UsesSqlOpenAndForwardsArgs(t *testing.T) {
	origOpen := sqlOpen
	defer func() { sqlOpen = origOpen }()

	var gotDriver string
	var gotDSN string

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		gotDriver = driverName
		gotDSN = dataSourceName
		return db, nil
	}

	cfg := ToolConfig{
		Conn:          "dsn-placeholder",
		QueryTemplate: "SELECT id FROM users WHERE id = $1",
	}

	rows := sqlmock.NewRows([]string{"id"}).AddRow(7)

	mock.ExpectQuery(regexp.QuoteMeta(cfg.QueryTemplate)).
		WithArgs(int64(7)).
		WillReturnRows(rows)

	out, err := ExecPostgres(context.Background(), cfg, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotDriver != "postgres" {
		t.Fatalf("unexpected driver: got %q want %q", gotDriver, "postgres")
	}
	if gotDSN != cfg.Conn {
		t.Fatalf("unexpected DSN: got %q want %q", gotDSN, cfg.Conn)
	}

	want := "id=7 \n"
	if out != want {
		t.Fatalf("unexpected output\nGot:  %q\nWant: %q", out, want)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestExecPostgres_SqlOpenError(t *testing.T) {
	origOpen := sqlOpen
	defer func() { sqlOpen = origOpen }()

	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("open failed")
	}

	cfg := ToolConfig{
		Conn:          "dsn-placeholder",
		QueryTemplate: "SELECT 1",
	}

	out, err := ExecPostgres(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}
