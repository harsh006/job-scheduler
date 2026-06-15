package repository

import (
	"context"
	"database/sql"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

type RunRepository interface {
	Create(ctx context.Context, run domain.Run) error
	UpdateStatus(ctx context.Context, run domain.Run) error
	ListByJobID(ctx context.Context, jobID string, limit int) ([]domain.Run, error)
	ListRecent(ctx context.Context, limit int) ([]domain.Run, error)
}

type MySQLRunRepository struct {
	db *sql.DB
}

func NewMySQLRunRepository(db *sql.DB) *MySQLRunRepository {
	return &MySQLRunRepository{db: db}
}

func (r *MySQLRunRepository) Create(ctx context.Context, run domain.Run) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO runs (id, job_id, status, attempt, started_at)
		VALUES (?, ?, ?, ?, ?)`,
		run.ID, run.JobID, run.Status, run.Attempt, run.StartedAt,
	)
	return err
}

func (r *MySQLRunRepository) UpdateStatus(ctx context.Context, run domain.Run) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE runs SET status=?, finished_at=?, duration_ms=?, response_code=?, error_message=?
		WHERE id=?`,
		run.Status, run.FinishedAt, run.DurationMs, run.ResponseCode,
		run.ErrorMessage, run.ID,
	)
	return err
}

func (r *MySQLRunRepository) ListByJobID(ctx context.Context, jobID string, limit int) ([]domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, status, attempt, started_at, finished_at,
		       duration_ms, response_code, error_message
		FROM runs WHERE job_id=? ORDER BY started_at DESC LIMIT ?`,
		jobID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (r *MySQLRunRepository) ListRecent(ctx context.Context, limit int) ([]domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, status, attempt, started_at, finished_at,
		       duration_ms, response_code, error_message
		FROM runs ORDER BY started_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

func scanRun(s rowScanner) (domain.Run, error) {
	var run domain.Run
	var finishedAt sql.NullTime
	var durationMs sql.NullInt64
	var responseCode sql.NullInt32
	var errorMessage sql.NullString

	err := s.Scan(
		&run.ID, &run.JobID, &run.Status, &run.Attempt, &run.StartedAt,
		&finishedAt, &durationMs, &responseCode, &errorMessage,
	)
	if err != nil {
		return run, err
	}

	if finishedAt.Valid {
		t := finishedAt.Time
		run.FinishedAt = &t
	}
	if durationMs.Valid {
		v := durationMs.Int64
		run.DurationMs = &v
	}
	if responseCode.Valid {
		v := int(responseCode.Int32)
		run.ResponseCode = &v
	}
	if errorMessage.Valid {
		run.ErrorMessage = &errorMessage.String
	}

	return run, nil
}

func scanRuns(rows *sql.Rows) ([]domain.Run, error) {
	var runs []domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	// return empty slice, not nil, for clean JSON encoding
	if runs == nil {
		runs = []domain.Run{}
	}
	return runs, rows.Err()
}

