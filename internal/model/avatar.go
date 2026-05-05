package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type UploadStatus string

const (
	UploadStatusUploading UploadStatus = "uploading"
	UploadStatusUploaded  UploadStatus = "uploaded"
	UploadStatusFailed    UploadStatus = "failed"
)

type ProcessingOp string

const (
	ProcessingOpPending   ProcessingOp = "pending"
	ProcessingOpRunning   ProcessingOp = "processing"
	ProcessingOpCompleted ProcessingOp = "completed"
	ProcessingOpFailed    ProcessingOp = "failed"
)

type Avatar struct {
	ID               uuid.UUID        `json:"id"`
	UserID           string           `json:"user_id"`
	FileName         string           `json:"file_name"`
	MimeType         string           `json:"mime_type"`
	SizeBytes        int64            `json:"size_bytes"`
	S3Key            string           `json:"s3_key"`
	ThumbnailS3Keys  *json.RawMessage `json:"thumbnail_s3_keys"`
	UploadStatus     UploadStatus     `json:"upload_status"`
	ProcessingStatus ProcessingOp     `json:"processing_status"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	DeletedAt        *time.Time       `json:"deleted_at"`
}

type AvatarCreateInfo struct {
	ID               uuid.UUID `json:"id"`
	UserID           string    `json:"user_id"`
	ProcessingStatus string    `json:"processing_status"`
	CreatedAt        time.Time `json:"created_at"`
}
