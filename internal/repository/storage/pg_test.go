package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGStorage_CreateAvatar_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	userID := "1"
	fileName := "avatar.png"
	mimeType := "image/png"
	sizeBytes := int64(1024)
	width := 512
	height := 512
	s3Key := ""
	avatarID := uuid.New()

	store := &PGStorage{db: db}

	rows := sqlmock.NewRows([]string{"id", "user_id", "processing_status", "created_at"}).
		AddRow(avatarID, userID, "processing", time.Now())

	query := `INSERT INTO avatars (user_id, file_name, mime_type, size_bytes, width, height, s3_key) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, user_id, processing_status, created_at`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(userID, fileName, mimeType, sizeBytes, width, height, s3Key).
		WillReturnRows(rows)

	avatar, err := store.CreateAvatar(context.Background(), userID, fileName, mimeType, width, height, sizeBytes)
	assert.NoError(t, err)
	require.NotNil(t, avatar)

	assert.Equal(t, avatarID, avatar.ID)
	assert.Equal(t, userID, avatar.UserID)
	assert.Equal(t, "processing", avatar.ProcessingStatus)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_CreateAvatar_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	userID := "1"
	fileName := "avatar.png"
	mimeType := "image/png"
	sizeBytes := int64(1024)
	width := 512
	height := 512
	s3Key := ""

	store := &PGStorage{db: db}

	query := `INSERT INTO avatars (user_id, file_name, mime_type, size_bytes, width, height, s3_key) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, user_id, processing_status, created_at`

	dbErr := errors.New("db error")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(userID, fileName, mimeType, sizeBytes, width, height, s3Key).
		WillReturnError(dbErr)

	avatar, err := store.CreateAvatar(context.Background(), userID, fileName, mimeType, width, height, sizeBytes)

	assert.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	require.Nil(t, avatar)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_UpdateAvatarS3Key_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	id := "avatar-1"
	s3Key := "avatars/user-1/avatar.png"

	query := `UPDATE avatars SET s3_key = $1, upload_status = 'uploaded', updated_at = NOW() WHERE id = $2`

	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(s3Key, id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.UpdateAvatarS3Key(context.Background(), id, s3Key)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarMeta_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := uuid.New()
	userID := "user-1"

	width := 512
	height := 256
	thumbnails := `{"100x100":"url_100","300x300":"url_300"}`
	createdAt := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 8, 10, 5, 0, 0, time.UTC)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, width, height, thumbnail_s3_keys, created_at, updated_at
        FROM avatars WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"id", "user_id", "file_name", "mime_type", "size_bytes", "width", "height",
		"thumbnail_s3_keys", "created_at", "updated_at"}).
		AddRow(avatarID, userID, "avatar.png", "image/png", int64(1024), &width, &height,
			[]byte(thumbnails), createdAt, updatedAt)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnRows(rows)

	avatar, err := store.GetAvatarMeta(ctx, avatarID.String(), userID)
	require.NoError(t, err)
	require.NotNil(t, avatar)

	assert.Equal(t, avatarID, avatar.ID)
	assert.Equal(t, userID, avatar.UserID)
	assert.Equal(t, "avatar.png", avatar.FileName)
	assert.Equal(t, "image/png", avatar.MimeType)
	assert.Equal(t, int64(1024), avatar.SizeBytes)
	assert.Equal(t, width, avatar.Dimensions.Width)
	assert.Equal(t, height, avatar.Dimensions.Height)
	assert.Len(t, avatar.Thumbnails, 2)
	assert.ElementsMatch(t, []*model.AvatarMetaThumbnail{
		{Size: "100x100", Url: "url_100"},
		{Size: "300x300", Url: "url_300"},
	}, avatar.Thumbnails)
	assert.Equal(t, createdAt, avatar.CreatedAt)
	assert.Equal(t, updatedAt, avatar.UpdatedAt)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarMeta_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := uuid.New()
	userID := "user-1"

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, width, height, thumbnail_s3_keys, created_at, updated_at
        FROM avatars WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnError(sql.ErrNoRows)

	avatar, err := store.GetAvatarMeta(ctx, avatarID.String(), userID)
	require.NoError(t, err)
	require.Nil(t, avatar)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarMeta_BadThumbnailsJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	userID := "user-1"
	createdAt := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 8, 10, 5, 0, 0, time.UTC)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, width, height, thumbnail_s3_keys, created_at, updated_at
        FROM avatars WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"id", "user_id", "file_name", "mime_type", "size_bytes", "width", "height",
		"thumbnail_s3_keys", "created_at", "updated_at"}).
		AddRow(avatarID, userID, "avatar.png", "image/png", int64(1024), nil, nil,
			[]byte(`{bad json}`), createdAt, updatedAt)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnRows(rows)

	avatar, err := store.GetAvatarMeta(ctx, avatarID, userID)
	require.Error(t, err)
	require.Nil(t, avatar)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarByID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := uuid.New()
	userID := "user-1"

	thumbnails := `{"100x100":"url_100","300x300":"url_300"}`
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys, 
               upload_status, processing_status, created_at, updated_at, deleted_at
        FROM avatars 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"id", "user_id", "file_name", "mime_type", "size_bytes", "s3_key",
		"thumbnail_s3_keys", "upload_status", "processing_status", "created_at", "updated_at", "deleted_at"}).
		AddRow(avatarID, userID, "avatar.png", "image/png", int64(1024), "avatars/avatar.png", []byte(thumbnails),
			"uploaded", "completed", now, now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnRows(rows)

	avatar, err := store.GetAvatarByID(ctx, avatarID.String(), userID)
	require.NoError(t, err)
	require.NotNil(t, avatar)

	assert.Equal(t, avatarID, avatar.ID)
	assert.Equal(t, userID, avatar.UserID)
	assert.Equal(t, "avatar.png", avatar.FileName)
	assert.Equal(t, "image/png", avatar.MimeType)
	assert.Equal(t, int64(1024), avatar.SizeBytes)
	assert.Equal(t, "avatars/avatar.png", avatar.S3Key)
	assert.Equal(t, model.UploadStatusUploaded, avatar.UploadStatus)
	assert.Equal(t, model.ProcessingOpCompleted, avatar.ProcessingStatus)
	assert.Equal(t, now, avatar.CreatedAt)
	assert.Equal(t, now, avatar.UpdatedAt)
	require.Nil(t, avatar.DeletedAt)

	require.NotNil(t, avatar.ThumbnailS3Keys)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := uuid.New()
	userID := "user-1"

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys, 
               upload_status, processing_status, created_at, updated_at, deleted_at
        FROM avatars 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnError(sql.ErrNoRows)

	avatar, err := store.GetAvatarByID(ctx, avatarID.String(), userID)
	require.NoError(t, err)
	require.Nil(t, avatar)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_GetAvatarByID_BadThumbnailsJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := uuid.New()
	userID := "user-1"
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)

	query := `
        SELECT id, user_id, file_name, mime_type, size_bytes, s3_key, thumbnail_s3_keys, 
               upload_status, processing_status, created_at, updated_at, deleted_at
        FROM avatars 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"id", "user_id", "file_name", "mime_type", "size_bytes", "s3_key",
		"thumbnail_s3_keys", "upload_status", "processing_status", "created_at", "updated_at", "deleted_at"}).
		AddRow(avatarID, userID, "avatar.png", "image/png", int64(1024), "avatars/avatar.png",
			[]byte(`{bad json}`), "uploaded", "completed", now, now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnRows(rows)

	avatar, err := store.GetAvatarByID(ctx, avatarID.String(), userID)
	require.Error(t, err)
	require.Nil(t, avatar)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_SoftDeleteAvatar_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	userID := "user-1"

	query := `
        UPDATE avatars 
        SET deleted_at = NOW(),
            updated_at = NOW()
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
    `

	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.SoftDeleteAvatar(ctx, avatarID, userID)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_SoftDeleteAvatar_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	userID := "user-1"

	query := `
        UPDATE avatars 
        SET deleted_at = NOW(),
            updated_at = NOW()
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
    `

	dbErr := errors.New("db error")

	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(avatarID, userID).
		WillReturnError(dbErr)

	err = store.SoftDeleteAvatar(ctx, avatarID, userID)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_UpdateProcessingStatus_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	status := model.ProcessingOp("processing")

	query := `
        UPDATE avatars SET processing_status = $1, updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	rows := sqlmock.NewRows([]string{"id"}).AddRow(avatarID)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(status, avatarID).
		WillReturnRows(rows)

	err = store.UpdateProcessingStatus(ctx, avatarID, status)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_UpdateProcessingStatus_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	status := model.ProcessingOp("processing")

	query := `
        UPDATE avatars SET processing_status = $1, updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	dbErr := errors.New("db error")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(status, avatarID).
		WillReturnError(dbErr)

	err = store.UpdateProcessingStatus(ctx, avatarID, status)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_UpdateProcessingStatus_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-404"
	status := model.ProcessingOp("processing")

	query := `
        UPDATE avatars SET processing_status = $1, updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(status, avatarID).
		WillReturnError(sql.ErrNoRows)

	err = store.UpdateProcessingStatus(ctx, avatarID, status)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_SetThumbnailsData_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	avatarThumbnails := []byte(`{"100x100":"url_100","300x300":"url_300"}`)

	query := `
        UPDATE avatars SET thumbnail_s3_keys = $1::jsonb, processing_status = 'completed', updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	rows := sqlmock.NewRows([]string{"id"}).AddRow(avatarID)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarThumbnails, avatarID).
		WillReturnRows(rows)

	err = store.SetThumbnailsData(ctx, avatarID, avatarThumbnails)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_SetThumbnailsData_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	avatarThumbnails := []byte(`{"100x100":"url_100","300x300":"url_300"}`)

	query := `
        UPDATE avatars SET thumbnail_s3_keys = $1::jsonb, processing_status = 'completed', updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	dbErr := errors.New("db error")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarThumbnails, avatarID).
		WillReturnError(dbErr)

	err = store.SetThumbnailsData(ctx, avatarID, avatarThumbnails)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_SetThumbnailsData_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-404"
	avatarThumbnails := []byte(`{"100x100":"url_100"}`)

	query := `
        UPDATE avatars SET thumbnail_s3_keys = $1::jsonb, processing_status = 'completed', updated_at = NOW()
        WHERE id = $2 AND deleted_at IS NULL RETURNING id`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarThumbnails, avatarID).
		WillReturnError(sql.ErrNoRows)

	err = store.SetThumbnailsData(ctx, avatarID, avatarThumbnails)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_AvatarResizeIsProcessed_True(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `SELECT processing_status FROM avatars WHERE id = $1 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"processing_status"}).
		AddRow(model.ProcessingOpProcessing)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnRows(rows)

	got, err := store.AvatarResizeIsProcessed(ctx, avatarID)
	require.NoError(t, err)
	require.True(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_AvatarResizeIsProcessed_False(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `SELECT processing_status FROM avatars WHERE id = $1 AND deleted_at IS NULL`

	rows := sqlmock.NewRows([]string{"processing_status"}).
		AddRow(model.ProcessingOpCompleted)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnRows(rows)

	got, err := store.AvatarResizeIsProcessed(ctx, avatarID)
	require.NoError(t, err)
	require.False(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_AvatarResizeIsProcessed_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-404"

	query := `SELECT processing_status FROM avatars WHERE id = $1 AND deleted_at IS NULL`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnError(sql.ErrNoRows)

	got, err := store.AvatarResizeIsProcessed(ctx, avatarID)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.False(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_AvatarResizeIsProcessed_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `SELECT processing_status FROM avatars WHERE id = $1 AND deleted_at IS NULL`
	dbErr := errors.New("db error")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnError(dbErr)

	got, err := store.AvatarResizeIsProcessed(ctx, avatarID)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
	require.False(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_CheckAvatarThumbnailKeysIsDeleted_BothDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"
	deletedAt := time.Now()

	query := `SELECT thumbnail_s3_keys, deleted_at FROM avatars WHERE id = $1`

	rows := sqlmock.NewRows([]string{"thumbnail_s3_keys", "deleted_at"}).
		AddRow(nil, deletedAt)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnRows(rows)

	isDeleted, err := store.CheckAvatarThumbnailKeysIsDeleted(ctx, avatarID)
	require.NoError(t, err)
	assert.True(t, isDeleted)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_CheckAvatarThumbnailKeysIsDeleted_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `SELECT thumbnail_s3_keys, deleted_at FROM avatars WHERE id = $1`
	dbErr := errors.New("database connection failed")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnError(dbErr)

	isDeleted, err := store.CheckAvatarThumbnailKeysIsDeleted(ctx, avatarID)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
	assert.False(t, isDeleted)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_DeleteAvatarThumbnailsData_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `UPDATE avatars SET thumbnail_s3_keys = NULL, updated_at = NOW() WHERE id = $1 RETURNING id`

	rows := sqlmock.NewRows([]string{"id"}).AddRow(avatarID)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnRows(rows)

	err = store.DeleteAvatarThumbnailsData(ctx, avatarID)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_DeleteAvatarThumbnailsData_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-1"

	query := `UPDATE avatars SET thumbnail_s3_keys = NULL, updated_at = NOW() WHERE id = $1 RETURNING id`

	dbErr := errors.New("database error")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnError(dbErr)

	err = store.DeleteAvatarThumbnailsData(ctx, avatarID)
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPGStorage_DeleteAvatarThumbnailsData_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PGStorage{db: db}

	ctx := context.Background()
	avatarID := "avatar-404"

	query := `UPDATE avatars SET thumbnail_s3_keys = NULL, updated_at = NOW() WHERE id = $1 RETURNING id`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(avatarID).
		WillReturnError(sql.ErrNoRows)

	err = store.DeleteAvatarThumbnailsData(ctx, avatarID)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)

	require.NoError(t, mock.ExpectationsWereMet())
}
