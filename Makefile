APP_NAME=gophprofile

BUILD_DIR := build
BIN_DIR := bin
DB_HOST=192.168.0.105
DB_USER=gophprofile
DB_NAME=gophprofile
DB_PASS=gophprofile
DB_PORT=5432
DB_STRING="postgres://$(DB_NAME):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable"
DB_MIGRATIONS_PATH="./migrations"

.DEFAULT_GOAL := help

.PHONY: build
build:
	go build -o ./cmd/server ./cmd/server

.PHONY: migrate-up
migrate-up:
	migrate \
	-path $(DB_MIGRATIONS_PATH) \
	-database $(DB_STRING) up

.PHONY: migrate-down
migrate-down:
	migrate \
	-path $(DB_MIGRATIONS_PATH) \
	-database $(DB_STRING) down


.PHONY: migrate-create
migrate-create:
ifdef NAME
	migrate create \
    	-ext sql \
    	-dir $(DB_MIGRATIONS_PATH) \
    	-seq $(NAME)
else
	@echo "Require variable NAME not found"
endif


.PHONY: install-pg-tools
install-pg-tools:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest # golang-migrate CLI

.PHONY: install-mock-tools
install-mock-tools:
	go install github.com/golang/mock/mockgen@latest  # mocks for tests
	go install github.com/golang/mock/gomock@latest

.PHONY: install-all-tools
install-all-tools: install-mock-tools install-pg-tools

.PHONY: test
test:
	go test -v ./... | { grep -v 'no test files'; true; }

.PHONY: test_cover
test_cover:
	go test -coverprofile=coverage.out ./...
	cat coverage.out | grep -v '/mocks\|/test\|/vendor\|/internal/model\|/proto' > coverage.filtered.out
	go tool cover -func=coverage.filtered.out
	rm coverage.out coverage.filtered.out

#.PHONY: mock
#mock:
#	@echo "Generating mock for Storage..."
#	mockgen -destination=internal/repository/pgstorage/mocks/pgstorage_mock.go -package=pgstorage -source=internal/service/service.go Storage
#	@echo "Generating mock for Service..."
#	mockgen -destination=internal/service/mocks/service_mock.go -package=service -source=internal/controller/grpc/grpc.go Service

.PHONY: up
up:
	@sudo docker compose up -d

.PHONY: down
down:
	@sudo docker compose down

.PHONY: gofmt
gofmt:
	@gofmt -w ./..

CYAN := \033[36m
BOLD := \033[1m
NO_COLOR := \033[0m
LOGO := "🦫"

.PHONY: help
help:
	@echo "$(LOGO)  $(CYAN)$(BOLD)$(APP_NAME)$(NO_COLOR)"
	@echo ""
	@echo "command                | description"
	@echo "===================================================="
	@echo "install-pg-tools       | install golang-migrate CLI"
	@echo "install-mock-tools     | install mockgen/gomock"
	@echo "install-all-tools      | install proto/mock/pg tools"
	@echo "migrate-up             | apply DB migrations"
	@echo "migrate-down           | rollback migration"
	@echo "migrate-create         | create migration (NAME=...) "
	@echo "mock                   | generate Storage/Service mocks"
	@echo "test                   | run tests"
	@echo "test_cover             | tests + coverage report"
	@echo "gofmt                  | format all Go files"
	@echo "build-cli              | cross-platform CLI builds"
	@echo "build                  | build server"