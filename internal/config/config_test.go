package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMustReadConfig_Success(t *testing.T) {
	// Set test environment variables
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("MINIO_ENDPOINT", "localhost:9000")
	os.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	os.Setenv("MINIO_SECRET_KEY", "minioadmin123")
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	defer func() {
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("MINIO_ENDPOINT")
		os.Unsetenv("MINIO_ACCESS_KEY")
		os.Unsetenv("MINIO_SECRET_KEY")
		os.Unsetenv("RABBITMQ_URL")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("HTTP_ADDR")
	}()

	cfg := MustReadConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable", cfg.PGConnString)
	assert.Equal(t, "localhost:9000", cfg.MinIOEndpoint)
	assert.Equal(t, "minioadmin", cfg.MinIOAccessKey)
	assert.Equal(t, "minioadmin123", cfg.MinIOSecretKey)
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", cfg.RabbitURL)
}

func TestMustReadConfig_WithCustomHTTPAddr(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":9090")
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("MINIO_ENDPOINT", "localhost:9000")
	os.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	os.Setenv("MINIO_SECRET_KEY", "minioadmin123")
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	defer func() {
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("MINIO_ENDPOINT")
		os.Unsetenv("MINIO_ACCESS_KEY")
		os.Unsetenv("MINIO_SECRET_KEY")
		os.Unsetenv("RABBITMQ_URL")
	}()

	cfg := MustReadConfig()
	assert.Equal(t, ":9090", cfg.HTTPAddr)
}
