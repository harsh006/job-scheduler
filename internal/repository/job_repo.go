package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

type JobRepository interface {
	Create(ctx context.Context, job domain.Job) error
	GetByID(ctx context.Context, id string) (domain.Job, error)
	Update(ctx context.Context, job domain.Job) error
	UpdateNextRunAt(ctx context.Context, jobID string, nextRunAt time.Time) error
	Delete(ctx context.Context, id string) error
	ListActive(ctx context.Context) ([]domain.Job, error)
	List(ctx context.Context) ([]domain.Job, error)
}

type MySQLJobRepository struct {
	db *sql.DB
}

func NewMySQLJobRepository(db *sql.DB) *MySQLJobRepository {
	return &MySQLJobRepository{db: db}
}

func (r *MySQLJobRepository) Create(ctx context.Context, job domain.Job) error {
	headers, err := json.Marshal(job.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO jobs
			(id, name, cron_expr, url, http_method, payload, headers, status,
			 max_retries, retry_delay_secs, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Name, job.CronExpr, job.URL, job.HTTPMethod,
		nullableString(job.Payload), headers, job.Status,
		job.MaxRetries, job.RetryDelaySecs, job.NextRunAt,
		job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (r *MySQLJobRepository) GetByID(ctx context.Context, id string) (domain.Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, cron_expr, url, http_method, payload, headers, status,
		       max_retries, retry_delay_secs, next_run_at, created_at, updated_at
		FROM jobs WHERE id = ?`, id)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrNotFound
	}
	return job, err
}

func (r *MySQLJobRepository) Update(ctx context.Context, job domain.Job) error {
	headers, err := json.Marshal(job.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE jobs SET
			name=?, cron_expr=?, url=?, http_method=?, payload=?, headers=?,
			status=?, max_retries=?, retry_delay_secs=?, next_run_at=?, updated_at=?
		WHERE id=?`,
		job.Name, job.CronExpr, job.URL, job.HTTPMethod,
		nullableString(job.Payload), headers, job.Status,
		job.MaxRetries, job.RetryDelaySecs, job.NextRunAt,
		time.Now().UTC(), job.ID,
	)
	return err
}

func (r *MySQLJobRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status=?, updated_at=? WHERE id=?`,
		domain.JobStatusDeleted, time.Now().UTC(), id,
	)
	return err
}

func (r *MySQLJobRepository) UpdateNextRunAt(ctx context.Context, jobID string, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET next_run_at=?, updated_at=? WHERE id=?`,
		nextRunAt.UTC(), time.Now().UTC(), jobID,
	)
	return err
}

func (r *MySQLJobRepository) ListActive(ctx context.Context) ([]domain.Job, error) {
	return r.listByStatus(ctx, domain.JobStatusActive)
}

func (r *MySQLJobRepository) List(ctx context.Context) ([]domain.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, cron_expr, url, http_method, payload, headers, status,
		       max_retries, retry_delay_secs, next_run_at, created_at, updated_at
		FROM jobs WHERE status != 'deleted' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (r *MySQLJobRepository) listByStatus(ctx context.Context, status domain.JobStatus) ([]domain.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, cron_expr, url, http_method, payload, headers, status,
		       max_retries, retry_delay_secs, next_run_at, created_at, updated_at
		FROM jobs WHERE status=? ORDER BY next_run_at ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(s rowScanner) (domain.Job, error) {
	var j domain.Job
	var payload sql.NullString
	var headersRaw []byte

	err := s.Scan(
		&j.ID, &j.Name, &j.CronExpr, &j.URL, &j.HTTPMethod,
		&payload, &headersRaw, &j.Status,
		&j.MaxRetries, &j.RetryDelaySecs, &j.NextRunAt,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return j, err
	}

	j.Payload = payload.String

	if len(headersRaw) > 0 {
		if err := json.Unmarshal(headersRaw, &j.Headers); err != nil {
			return j, fmt.Errorf("unmarshal headers: %w", err)
		}
	}

	return j, nil
}

func scanJobs(rows *sql.Rows) ([]domain.Job, error) {
	var jobs []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func nullableString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
