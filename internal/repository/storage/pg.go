package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

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

// Health db health check (Ping)
func (s *PGStorage) Health() error {
	return s.db.Ping()
}

// Shutdown closes database connection pool gracefully.
// Call during application shutdown to release resources.
func (s *PGStorage) Shutdown() error {
	return s.db.Close()
}

func (s *PGStorage) CreateAvatar(ctx context.Context, userID, fileName, mimeType string, width, height int, sizeBytes int64) (*model.AvatarCreateInfo, error) {
	query := `
        INSERT INTO avatars (user_id, file_name, mime_type, size_bytes, width, height, s3_key)
        VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, user_id, processing_status, created_at`

	avatar := &model.AvatarCreateInfo{}

	if err := s.db.QueryRowContext(ctx, query, userID, fileName, mimeType, sizeBytes, width, height, "").Scan(
		&avatar.ID, &avatar.UserID, &avatar.ProcessingStatus, &avatar.CreatedAt,
	); err != nil {
		return nil, err
	}

	return avatar, nil
}

func (s *PGStorage) UpdateAvatarS3Key(ctx context.Context, id string, s3Key string) error {
	query := `UPDATE avatars SET s3_key = $1, upload_status = 'uploaded', updated_at = NOW() WHERE id = $2`

	_, err := s.db.ExecContext(ctx, query, s3Key, id)

	return err
}

func (s *PGStorage) GetAvatarMeta(ctx context.Context, avatarID, userID string) (*model.AvatarMeta, error) {
	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, width, height, thumbnail_s3_keys, created_at, updated_at
        FROM avatars WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	var (
		avatar          model.AvatarMeta
		widthPtr        *int
		heightPtr       *int
		thumbnailsBytes []byte // Временный буфер для JSONB данных
	)

	err := s.db.QueryRowContext(ctx, query, avatarID, userID).Scan(
		&avatar.ID,
		&avatar.UserID,
		&avatar.FileName,
		&avatar.MimeType,
		&avatar.SizeBytes,
		&widthPtr,
		&heightPtr,
		&thumbnailsBytes,
		&avatar.CreatedAt,
		&avatar.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// AvatarMetaDimensions
	if widthPtr != nil {
		avatar.Dimensions.Width = *widthPtr
	}
	if heightPtr != nil {
		avatar.Dimensions.Height = *heightPtr
	}

	// AvatarMetaThumbnails
	avatar.Thumbnails = make(model.AvatarMetaThumbnails, 0)
	var thumbnailsJSON map[string]string

	if len(thumbnailsBytes) > 0 {
		if err := json.Unmarshal(thumbnailsBytes, &thumbnailsJSON); err != nil {
			return nil, err
		}

		for k, v := range thumbnailsJSON {
			avatar.Thumbnails = append(avatar.Thumbnails, &model.AvatarMetaThumbnail{
				Size: k,
				Url:  v,
			})
		}
	}

	return &avatar, nil
}

func (s *PGStorage) GetAvatarByID(ctx context.Context, avatarID, userID string) (*model.Avatar, error) {
	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys, 
               upload_status, processing_status, created_at, updated_at, deleted_at
        FROM avatars 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	avatar := &model.Avatar{}
	var (
		thumbnailsBytes []byte
		deletedAtPtr    *time.Time
	)

	err := s.db.QueryRowContext(ctx, query, avatarID, userID).Scan(
		&avatar.ID,
		&avatar.UserID,
		&avatar.FileName,
		&avatar.MimeType,
		&avatar.SizeBytes,
		&avatar.S3Key,
		&thumbnailsBytes,
		&avatar.UploadStatus,
		&avatar.ProcessingStatus,
		&avatar.CreatedAt,
		&avatar.UpdatedAt,
		&deletedAtPtr,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Handle thumbnails JSONB
	if len(thumbnailsBytes) > 0 {
		if err := json.Unmarshal(thumbnailsBytes, &avatar.ThumbnailS3Keys); err != nil {
			return nil, err
		}
	}

	// Handle nullable deleted_at
	avatar.DeletedAt = deletedAtPtr

	return avatar, nil
}

func (s *PGStorage) SoftDeleteAvatar(ctx context.Context, avatarID, userID string) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE avatars 
        SET deleted_at = NOW(),
            updated_at = NOW()
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
    `, avatarID, userID)

	return err
}
