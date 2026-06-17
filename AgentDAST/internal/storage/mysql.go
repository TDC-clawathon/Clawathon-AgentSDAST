package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"agentdast/pkg/types"
)

// MySQLStore persists scan metadata and findings in MySQL. When a BlobStore is
// provided, full request logs are offloaded there and referenced by object key.
type MySQLStore struct {
	db    *sql.DB
	blobs BlobStore
}

const schemaDDL = `
CREATE TABLE IF NOT EXISTS scan_results (
	id              VARCHAR(36)  PRIMARY KEY,
	status          VARCHAR(16)  NOT NULL,
	started_at      DATETIME     NOT NULL,
	completed_at    DATETIME     NULL,
	config_json     TEXT         NOT NULL,
	summary_json    TEXT         NOT NULL,
	findings_json   LONGTEXT     NULL,
	logs_object_key VARCHAR(256) NULL,
	error_msg       TEXT         NULL,
	created_at      DATETIME     DEFAULT CURRENT_TIMESTAMP
)`

// NewMySQLStore connects using MYSQL_* env vars and ensures the schema exists.
func NewMySQLStore(blobs BlobStore) (*MySQLStore, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		envOr("MYSQL_USER", "root"),
		os.Getenv("MYSQL_PASSWORD"),
		envOr("MYSQL_HOST", "127.0.0.1"),
		envOr("MYSQL_PORT", "3306"),
		envOr("MYSQL_DB", "agentdast"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		return nil, fmt.Errorf("mysql schema: %w", err)
	}
	return &MySQLStore{db: db, blobs: blobs}, nil
}

func (s *MySQLStore) SaveResult(ctx context.Context, result *types.ScanResult) error {
	configJSON, _ := json.Marshal(result.ScanConfig)
	summaryJSON, _ := json.Marshal(result.Summary)
	findingsJSON, _ := json.Marshal(result.Findings)

	var logsKey sql.NullString
	if s.blobs != nil && len(result.RequestLogs) > 0 {
		key := result.ID + "/request_logs.json"
		data, _ := json.Marshal(result.RequestLogs)
		if err := s.blobs.PutBlob(ctx, key, data); err != nil {
			return fmt.Errorf("store logs blob: %w", err)
		}
		logsKey = sql.NullString{String: key, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scan_results
			(id, status, started_at, completed_at, config_json, summary_json, findings_json, logs_object_key, error_msg)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status=VALUES(status), completed_at=VALUES(completed_at),
			summary_json=VALUES(summary_json), findings_json=VALUES(findings_json),
			logs_object_key=VALUES(logs_object_key), error_msg=VALUES(error_msg)`,
		result.ID, result.Status, result.StartedAt, nullableTime(result.CompletedAt),
		string(configJSON), string(summaryJSON), string(findingsJSON), logsKey, result.Error)
	return err
}

func (s *MySQLStore) GetResult(ctx context.Context, scanID string) (*types.ScanResult, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, status, started_at, completed_at, config_json, summary_json, findings_json, logs_object_key, error_msg
		FROM scan_results WHERE id = ?`, scanID)

	var (
		r            types.ScanResult
		completed    sql.NullTime
		configJSON   string
		summaryJSON  string
		findingsJSON sql.NullString
		logsKey      sql.NullString
		errMsg       sql.NullString
	)
	if err := row.Scan(&r.ID, &r.Status, &r.StartedAt, &completed, &configJSON,
		&summaryJSON, &findingsJSON, &logsKey, &errMsg); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scan %q not found", scanID)
		}
		return nil, err
	}
	if completed.Valid {
		r.CompletedAt = &completed.Time
	}
	r.Error = errMsg.String
	_ = json.Unmarshal([]byte(configJSON), &r.ScanConfig)
	_ = json.Unmarshal([]byte(summaryJSON), &r.Summary)
	if findingsJSON.Valid {
		_ = json.Unmarshal([]byte(findingsJSON.String), &r.Findings)
	}
	if logsKey.Valid && s.blobs != nil {
		if data, err := s.blobs.GetBlob(ctx, logsKey.String); err == nil {
			_ = json.Unmarshal(data, &r.RequestLogs)
		}
	}
	return &r, nil
}

func (s *MySQLStore) ListResults(ctx context.Context, limit, offset int) ([]*types.ScanResult, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, started_at, completed_at, summary_json, error_msg
		FROM scan_results ORDER BY started_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*types.ScanResult
	for rows.Next() {
		var (
			r           types.ScanResult
			completed   sql.NullTime
			summaryJSON string
			errMsg      sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Status, &r.StartedAt, &completed, &summaryJSON, &errMsg); err != nil {
			return nil, err
		}
		if completed.Valid {
			r.CompletedAt = &completed.Time
		}
		r.Error = errMsg.String
		_ = json.Unmarshal([]byte(summaryJSON), &r.Summary)
		out = append(out, &r)
	}
	return out, rows.Err()
}

func (s *MySQLStore) DeleteResult(ctx context.Context, scanID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scan_results WHERE id = ?`, scanID)
	return err
}

func (s *MySQLStore) Close() error { return s.db.Close() }

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
