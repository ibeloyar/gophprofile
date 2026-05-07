include .env

APP_NAME=gophprofile

BUILD_DIR := build
BIN_DIR := bin

DB_STRING="postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable"
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


.PHONY: install-tools
install-tools:
	go install github.com/golang/mock/mockgen@latest  # mocks for tests
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest # golang-migrate CLI


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

.PHONY: gofmt
gofmt:
	@gofmt -w ./..

## golangci-lint
GO_LINT_VERSION := $(shell curl -s https://api.github.com/repos/golangci/golangci-lint/releases/latest | jq -r '.tag_name')
ifeq ($(GO_LINT_VERSION),null) # в случае если запрос нарвется на rate limit
	GO_LINT_VERSION := 2.8.0
else
	GO_LINT_VERSION := $(shell echo $(GO_LINT_VERSION) | cut -c 2-)
endif
GO_LINT_TAR_GZ := golangci-lint-$(GO_LINT_VERSION)-linux-amd64.tar.gz
GO_LINT_DOWNLOAD_LINK := https://github.com/golangci/golangci-lint/releases/download/v$(GO_LINT_VERSION)/$(GO_LINT_TAR_GZ)
GO_LINT_TAR_GZ_BIN := $(patsubst %.tar.gz,%,$(GO_LINT_TAR_GZ))/golangci-lint
GO_LINT_BIN := $(BUILD_DIR)/golangci-lint-$(GO_LINT_VERSION)
$(GO_LINT_BIN):
	@mkdir -p $(BUILD_DIR)
	@wget -q --show-progress -O $(BUILD_DIR)/$(GO_LINT_TAR_GZ) $(GO_LINT_DOWNLOAD_LINK)
	@tar -xzf $(BUILD_DIR)/$(GO_LINT_TAR_GZ) -C $(BUILD_DIR) $(GO_LINT_TAR_GZ_BIN) --strip-components=1
	@mv $(BUILD_DIR)/golangci-lint $(GO_LINT_BIN)
	@rm $(BUILD_DIR)/$(GO_LINT_TAR_GZ)
###

.PHONY: lint
lint: $(GO_LINT_BIN)
	@$(GO_LINT_BIN) run



.PHONY: up
up:
	@sudo docker compose up -d

.PHONY: down
down:
	@sudo docker compose down

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
	@echo "install-tools          | install mock/migrate tools"
	@echo "mock                   | generate mocks"
	@echo "migrate-up             | apply DB migrations"
	@echo "migrate-down           | rollback migration"
	@echo "migrate-create         | create migration (NAME=...) "
	@echo "test                   | run tests"
	@echo "test_cover             | tests + coverage report"
	@echo "gofmt                  | format all Go files"
	@echo "up                     | run docker compose up -d"
	@echo "down                   | run docker compose down"