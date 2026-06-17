// Package server runs AgentDAST as an HTTP service: it accepts scan requests,
// reads the project's swagger + SAST report from fixed MinIO keys under the
// project prefix (<project>/sast/openapi.yaml, <project>/sast/report.md), runs
// the AI audit flow asynchronously, and stores the report and request logs back
// in MinIO under <project>/dast/.
package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Scan status values stored in the dast table.
const (
	StatusNew     = "new"
	StatusProcess = "process"
	StatusDone    = "done"
	StatusFail    = "fail"
	StatusCancel  = "cancel"
)

// dastSchema is the table this service owns. error_msg is added beyond the
// requested columns so a failed scan can surface why it failed via /api/status.
const dastSchema = `
CREATE TABLE IF NOT EXISTS dast (
	id          VARCHAR(36)  PRIMARY KEY,
	project_id  VARCHAR(255) NOT NULL,
	result_path VARCHAR(512) NULL,
	status      VARCHAR(16)  NOT NULL,
	error_msg   TEXT         NULL,
	last_update DATETIME     NOT NULL,
	INDEX idx_dast_project (project_id)
)`

// Record mirrors a row in the dast table.
type Record struct {
	ID         string    `json:"scan_id"`
	ProjectID  string    `json:"project_id"`
	ResultPath string    `json:"result_path,omitempty"`
	Status     string    `json:"status"`
	Progress   int       `json:"progress"`
	Phase      string    `json:"phase,omitempty"`
	Error      string    `json:"error,omitempty"`
	LastUpdate time.Time `json:"last_update"`
}

// Store wraps MySQL (state) and MinIO (swagger in / report + logs out).
type Store struct {
	db     *sql.DB
	minio  *minio.Client
	bucket string
}

// NewStore connects to MySQL and MinIO using the service config and ensures the
// dast table exists.
func NewStore(cfg Config) (*Store, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(dastSchema); err != nil {
		return nil, fmt.Errorf("ensure dast table: %w", err)
	}
	if err := ensureDastColumns(db); err != nil {
		return nil, fmt.Errorf("ensure dast columns: %w", err)
	}

	mc, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	return &Store{
		db:     db,
		minio:  mc,
		bucket: cfg.MinIOBucket,
	}, nil
}

// openDB opens MySQL with a bounded connection-ready retry (the DB container may
// still be starting when this service boots).
func openDB(cfg Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN())
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)

	var lastErr error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return db, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("mysql not reachable after retries: %w", lastErr)
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Ping reports DB reachability for the health check.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// CreateScan inserts a new dast row in the "new" state.
func (s *Store) CreateScan(ctx context.Context, scanID, projectID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dast (id, project_id, status, progress, last_update) VALUES (?, ?, ?, 0, ?)`,
		scanID, projectID, StatusNew, time.Now())
	return err
}

// EnsureScan reuses a Manager-precreated row or inserts one when id is client-supplied.
func (s *Store) EnsureScan(ctx context.Context, scanID, projectID string) error {
	rec, err := s.GetScan(ctx, scanID)
	if err != nil {
		return err
	}
	if rec == nil {
		return s.CreateScan(ctx, scanID, projectID)
	}
	if rec.ProjectID != projectID {
		return fmt.Errorf("scan %s belongs to project %s", scanID, rec.ProjectID)
	}
	if isActiveDastStatus(rec.Status) {
		return nil
	}
	return fmt.Errorf("scan %s already finished with status %s", scanID, rec.Status)
}

// SetStatus updates status (and optionally result_path / error_msg) for a scan.
func (s *Store) SetStatus(ctx context.Context, scanID, status, resultPath, errMsg string) error {
	progress := 0
	phase := ""
	switch status {
	case StatusDone:
		progress = 100
		phase = "done"
	case StatusProcess:
		progress = 10
		phase = "running"
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE dast SET status = ?, result_path = ?, error_msg = ?, progress = ?, phase = ?, last_update = ? WHERE id = ?`,
		status, nullable(resultPath), nullable(errMsg), progress, nullable(phase), time.Now(), scanID)
	return err
}

// SetProgress updates live progress while a scan is running.
func (s *Store) SetProgress(ctx context.Context, scanID, status, phase string, progress int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE dast SET status = ?, phase = ?, progress = ?, last_update = ? WHERE id = ?`,
		status, nullable(phase), progress, time.Now(), scanID)
	return err
}

// GetScan returns the dast row for a scan id.
func (s *Store) GetScan(ctx context.Context, scanID string) (*Record, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, result_path, status, COALESCE(progress, 0), phase, error_msg, last_update FROM dast WHERE id = ?`, scanID)
	var (
		r          Record
		resultPath sql.NullString
		phase      sql.NullString
		errMsg     sql.NullString
	)
	if err := row.Scan(&r.ID, &r.ProjectID, &resultPath, &r.Status, &r.Progress, &phase, &errMsg, &r.LastUpdate); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.ResultPath = resultPath.String
	r.Phase = phase.String
	r.Error = errMsg.String
	return &r, nil
}

// CancelScan transitions a scan to the "cancel" state, but only while it is still
// new or in progress, so a finished (done/fail) scan is never clobbered. It
// reports whether a row was actually cancelled.
func (s *Store) CancelScan(ctx context.Context, scanID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE dast SET status = ?, error_msg = NULL, last_update = ? WHERE id = ? AND status IN (?, ?)`,
		StatusCancel, time.Now(), scanID, StatusNew, StatusProcess)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// StatObject reports whether an object exists in the configured bucket. A
// not-found result is (false, nil); any other error is returned as-is.
func (s *Store) StatObject(ctx context.Context, key string) (bool, error) {
	_, err := s.minio.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetObject downloads an object's raw bytes from the configured bucket.
func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.minio.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("read object %q: %w", key, err)
	}
	return data, nil
}

// PutObject uploads raw bytes to the configured bucket and returns the key.
func (s *Store) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.minio.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

// EnsureBucket creates the result/swagger bucket if it does not yet exist.
func (s *Store) EnsureBucket(ctx context.Context) error {
	exists, err := s.minio.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := s.minio.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("minio make bucket: %w", err)
		}
	}
	return nil
}

// EnabledAgentModel returns the enabled model_name from Manager's AgentModelConfig table.
func (s *Store) EnabledAgentModel(ctx context.Context, agentType string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT model_name FROM AgentModelConfig WHERE agent_type = ? AND enabled = 1 LIMIT 1`,
		strings.ToLower(strings.TrimSpace(agentType)),
	).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// WriteTempFile writes object bytes (a swagger spec or a SAST report) to a temp
// file and returns its path, so consumers that take a path (the spec parser, the
// SAST report loader) can read it. The extension is derived from the object key
// so format detection downstream still works.
func WriteTempFile(key string, data []byte) (string, func(), error) {
	ext := filepath.Ext(key)
	if ext == "" {
		ext = ".yaml"
	}
	f, err := os.CreateTemp("", "agentdast-*"+ext)
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, err
	}
	f.Close()
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func isActiveDastStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusNew, StatusProcess, "progress", "processing":
		return true
	default:
		return false
	}
}

func ensureDastColumns(db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE dast ADD COLUMN progress TINYINT UNSIGNED NOT NULL DEFAULT 0`,
		`ALTER TABLE dast ADD COLUMN phase VARCHAR(255) NULL`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			if isDuplicateColumnErr(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}
