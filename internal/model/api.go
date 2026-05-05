package model

import (
	"time"
)

type UploadAvatarFormatError struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}

type UploadAvatarSizeError struct {
	Error   string `json:"error"`
	MaxSize int64  `json:"max_size"`
}

type HealthResponse struct {
	Postgresql bool `json:"postgresql"`
	Minio      bool `json:"minio"`
	RabbitMQ   bool `json:"rabbitMQ"`
}

type UploadAvatarResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
