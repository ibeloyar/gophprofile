package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr       string
	PGConnString   string
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	RabbitURL      string
}

func ReadConfig() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPAddr: getEnvOrDefault("HTTP_ADDR", ":8080"),
	}

	cfg.PGConnString = buildPostgresDSN()
	if cfg.PGConnString == "" {
		return nil, fmt.Errorf("PG connection string is empty")
	}

	cfg.MinIOEndpoint = mustEnv("MINIO_ENDPOINT")
	cfg.MinIOAccessKey = mustEnv("MINIO_ACCESS_KEY")
	cfg.MinIOSecretKey = mustEnv("MINIO_SECRET_KEY")
	cfg.RabbitURL = mustEnv("RABBITMQ_URL")

	return cfg, nil
}

func mustEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	log.Fatalf("%s is required", key)

	return ""
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func buildPostgresDSN() string {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5432")
	user := mustEnv("DB_USER")
	pass := mustEnv("DB_PASSWORD")
	name := mustEnv("DB_NAME")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
}
