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
	migrationsTable = "schema_migrations"
	schemaName      = "public"
	migrationsPath  = "./migrations"
)

type PGStorage struct {
	db *sql.DB
}

// New initializes PostgreSQL storage with automatic migrations.
// 1. Creates pgx connection pool from DSN
// 2. Sets up migration driver with custom migrations table
// 3. Resolves absolute path to ./migrations directory
// 4. Applies all pending migrations (ignores ErrNoChange)
// 5. Returns sql.DB wrapper for standard queries
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

// Health performs database ping to verify connection.
func (s *PGStorage) Health() error {
	return s.db.Ping()
}

// Shutdown closes underlying connection pool gracefully.
func (s *PGStorage) Shutdown() error {
	return s.db.Close()
}

// CreateAvatar inserts new avatar record with original file metadata.
// Returns created avatar info with generated UUID and timestamps.
// S3 key initially empty (set after upload).
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

// UpdateAvatarS3Key updates avatar with S3 object key after successful upload.
// Sets upload_status='uploaded' and updates timestamp.
func (s *PGStorage) UpdateAvatarS3Key(ctx context.Context, id string, s3Key string) error {
	query := `UPDATE avatars SET s3_key = $1, upload_status = 'uploaded', updated_at = NOW() WHERE id = $2`

	_, err := s.db.ExecContext(ctx, query, s3Key, id)

	return err
}

// GetAvatarMeta retrieves avatar metadata by ID and user.
// Joins thumbnails JSONB, handles nullable dimensions, filters soft-deleted.
// Returns populated AvatarMeta with thumbnails slice.
func (s *PGStorage) GetAvatarMeta(ctx context.Context, avatarID, userID string) (*model.AvatarMeta, error) {
	var (
		avatar          model.AvatarMeta
		widthPtr        *int
		heightPtr       *int
		thumbnailsBytes []byte
	)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, width, height, thumbnail_s3_keys, created_at, updated_at
        FROM avatars WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

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

// GetAvatarByID fetches complete avatar record by ID and user.
// Includes all fields including nullable deleted_at and thumbnails JSONB.
func (s *PGStorage) GetAvatarByID(ctx context.Context, avatarID, userID string) (*model.Avatar, error) {
	var (
		thumbnailsBytes []byte
		deletedAtPtr    *time.Time
	)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys, 
               upload_status, processing_status, created_at, updated_at, deleted_at
        FROM avatars 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	avatar := &model.Avatar{}

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

	avatar.DeletedAt = deletedAtPtr

	return avatar, nil
}

// SoftDeleteAvatar marks avatar as deleted (soft delete).
// Clears thumbnails, sets deleted_at timestamp.
func (s *PGStorage) SoftDeleteAvatar(ctx context.Context, avatarID, userID string) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE avatars 
        SET thumbnail_s3_keys = NULL,
            deleted_at = NOW(),
            updated_at = NOW()
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
    `, avatarID, userID)

	return err
}

// UpdateProcessingStatus updates avatar processing operation status.
func (s *PGStorage) UpdateProcessingStatus(ctx context.Context, avatarID string, status model.ProcessingOp) error {
	var id string

	query := `
        UPDATE avatars SET processing_status = $1, updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	err := s.db.QueryRowContext(ctx, query, status, avatarID).Scan(&id)
	if err != nil {
		return err
	}

	return nil
}

// SetThumbnailsData stores thumbnails metadata as JSONB and marks processing complete.
func (s *PGStorage) SetThumbnailsData(ctx context.Context, avatarID string, avatarThumbnails []byte) error {
	var id string

	query := `
        UPDATE avatars SET thumbnail_s3_keys = $1::jsonb, processing_status = 'completed', updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	err := s.db.QueryRowContext(ctx, query, avatarThumbnails, avatarID).Scan(&id)

	if err != nil {
		return err
	}

	return nil
}
