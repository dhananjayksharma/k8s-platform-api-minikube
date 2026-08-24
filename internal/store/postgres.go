package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Job struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	s := &PostgresStore{db: db}

	var lastErr error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("postgres not ready: %w", lastErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS jobs (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			priority INTEGER NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) CreateJob(ctx context.Context, name string, priority int) (Job, error) {
	var j Job
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO jobs(name, priority, status)
		VALUES ($1, $2, 'queued')
		RETURNING id, name, priority, status, created_at, updated_at
	`, name, priority).Scan(&j.ID, &j.Name, &j.Priority, &j.Status, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (s *PostgresStore) GetJob(ctx context.Context, id int64) (Job, error) {
	var j Job
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, priority, status, created_at, updated_at
		FROM jobs
		WHERE id = $1
	`, id).Scan(&j.ID, &j.Name, &j.Priority, &j.Status, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (s *PostgresStore) UpdateJobStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status=$1, updated_at=now()
		WHERE id=$2
	`, status, id)
	return err
}
