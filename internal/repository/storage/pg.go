package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	ErrIsExistCode = "23505"

	migrationsTable = "schema_migrations"
	schemaName      = "public"
	migrationsPath  = "./migrations"
)

// PGStorage wraps sql.DB with PostgreSQL connection pool and migration management.
type PGStorage struct {
	db *sql.DB
}

// New creates PostgreSQL storage with automatic schema migrations.
// 1. Establishes pgx connection pool
// 2. Initializes migration driver with schema_migrations table
// 3. Applies all pending migrations from ./migrations directory
// 4. Returns sql.DB compatible storage instance
func New(connStr string) (*PGStorage, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, err
	}

	db := stdlib.OpenDBFromPool(pool)

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: migrationsTable,
		SchemaName:      schemaName,
	})
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "postgres", driver)
	if err != nil {
		return nil, err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}

	return &PGStorage{
		db: db,
	}, nil
}

// Ping
func (s *PGStorage) Health() error {
	return s.db.Ping()
}

// Shutdown closes database connection pool gracefully.
// Call during application shutdown to release resources.
func (s *PGStorage) Shutdown() error {
	return s.db.Close()
}

func (s *PGStorage) CreateAvatar(ctx context.Context, userID, fileName, mimeType string, sizeBytes int64) (*model.AvatarCreateInfo, error) {
	query := `
        INSERT INTO avatars (user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys)
        VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, user_id, s3_key, processing_status, created_at`

	avatar := &model.AvatarCreateInfo{}

	if err := s.db.QueryRowContext(ctx, query, userID, fileName, mimeType, sizeBytes, "{}").Scan(
		&avatar.ID, &avatar.UserID, &avatar.ProcessingStatus, &avatar.CreatedAt,
	); err != nil {
		return nil, err
	}

	return avatar, nil
}

func (s *PGStorage) UpdateAvatarS3Key(ctx context.Context, id string, s3Key string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE avatars
		SET s3_key = $1,
			upload_status = 'uploaded',
			updated_at = NOW()
		WHERE id = $2
	`, s3Key, id)

	return err
}
