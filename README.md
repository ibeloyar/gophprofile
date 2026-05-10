# Gophprofile

**GophProfile** is a service for managing user avatars. 
Users upload their photo once, and any third-party platforms (blogs, forums, comment services, etc.) 
can request the avatar via the user's email address.

### Tech Stack

| Component | Technology |
|-----------|------------|
| **Language** | Go 1.25 |
| **HTTP Framework** | Echo / Chi |
| **Database** | PostgreSQL 16 |
| **File Storage** | MinIO (S3-compatible) |
| **Message Broker** | RabbitMQ |
| **Migrations** | golang-migrate |
| **Testing** | testify, sqlmock, testcontainers |
| **Containerization** | Docker, Docker Compose |
| **Linter** | golangci-lint |

## Get started

**Fill in .env as in .env.example** and run containers

```terminaloutput
make build
make up
```

The application will be available:
- API: http://localhost:8080
- MinIO Console: http://localhost:9001 (miniouser/miniopass)
- RabbitMQ Management: http://localhost:15672 (rabbituser/rabbitpass)

### Development
Make available commands
```
command                | description
====================================================
install-tools          | install mock/migrate tools
mock                   | generate mocks
migrate-up             | apply DB migrations
migrate-down           | rollback migration
migrate-create         | create migration (NAME=...)
test                   | run tests
test_cover             | tests + coverage report
gofmt                  | format all Go files
up                     | run docker compose up -d
down                   | run docker compose down
build                  | build docker images
```

### Docker Compose

| Service | Port | Purpose |
|---------|------|---------|
| **app** | 8080 | server + worker |
| **postgres** | 5432 | Database |
| **rabbitmq** | 5672, 15672 | RabbitMQ broker |
| **minio** | 9000, 9001 | MinIO (S3) |


### Last test cover
```
go tool cover -func=coverage.filtered.out
github.com/ibeloyar/gophprofile/cmd/server/main.go:10:                          main                                    0.0%
github.com/ibeloyar/gophprofile/cmd/worker/main.go:10:                          main                                    0.0%
github.com/ibeloyar/gophprofile/internal/app/app.go:26:                         Run                                     0.0%
github.com/ibeloyar/gophprofile/internal/app/server.go:14:                      NewServer                               0.0%
github.com/ibeloyar/gophprofile/internal/config/config.go:21:                   MustReadConfig                          81.8%
github.com/ibeloyar/gophprofile/internal/config/config.go:42:                   mustEnv                                 50.0%
github.com/ibeloyar/gophprofile/internal/config/config.go:52:                   getEnvOrDefault                         100.0%
github.com/ibeloyar/gophprofile/internal/config/config.go:60:                   buildPostgresDSN                        100.0%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:35:           New                                     100.0%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:44:           Health                                  100.0%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:53:           UploadAvatar                            54.2%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:104:          DownloadAvatar                          91.3%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:144:          GetAvatarMeta                           100.0%
github.com/ibeloyar/gophprofile/internal/controller/controller.go:173:          DeleteAvatar                            87.5%
github.com/ibeloyar/gophprofile/internal/controller/helpers.go:23:              readBody                                100.0%
github.com/ibeloyar/gophprofile/internal/controller/helpers.go:62:              writeJSON                               100.0%
github.com/ibeloyar/gophprofile/internal/controller/helpers.go:77:              readAvatarFile                          84.2%
github.com/ibeloyar/gophprofile/internal/controller/helpers.go:120:             getDimensions                           100.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:45:      NewConsumer                             0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:68:      Run                                     0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:94:      Shutdown                                0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:103:     handleUpload                            0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:164:     handleDelete                            0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:237:     UploadHandler                           0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/consumer.go:288:     DeleteHandler                           0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:27:     NewPublisher                            0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:51:     Init                                    0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:62:     Health                                  0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:74:     Shutdown                                0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:87:     PublishUpload                           0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:109:    PublishDelete                           0.0%
github.com/ibeloyar/gophprofile/internal/repository/broker/publisher.go:131:    handleConfirms                          0.0%
github.com/ibeloyar/gophprofile/internal/repository/s3/s3.go:24:                New                                     81.8%
github.com/ibeloyar/gophprofile/internal/repository/s3/s3.go:52:                Health                                  71.4%
github.com/ibeloyar/gophprofile/internal/repository/s3/s3.go:68:                Upload                                  75.0%
github.com/ibeloyar/gophprofile/internal/repository/s3/s3.go:82:                Download                                81.8%
github.com/ibeloyar/gophprofile/internal/repository/s3/s3.go:104:               DeleteObjects                           80.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:35:           New                                     0.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:71:           Health                                  0.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:76:           Shutdown                                0.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:83:           CreateAvatar                            100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:106:          UpdateAvatarS3Key                       100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:117:          GetAvatarMeta                           94.7%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:180:          GetAvatarByID                           92.3%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:231:          SoftDeleteAvatar                        100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:243:          UpdateProcessingStatus                  100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:259:          SetThumbnailsData                       100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:278:          AvatarResizeIsProcessed                 100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:292:          CheckAvatarThumbnailKeysIsDeleted       100.0%
github.com/ibeloyar/gophprofile/internal/repository/storage/pg.go:312:          DeleteAvatarThumbnailsData              100.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:52:                 New                                     0.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:62:                 Shutdown                                0.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:75:                 Health                                  100.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:102:                UploadAvatar                            100.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:139:                DownloadAvatar                          100.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:151:                GetAvatarMeta                           100.0%
github.com/ibeloyar/gophprofile/internal/service/service.go:165:                DeleteAvatar                            78.9%
github.com/ibeloyar/gophprofile/internal/service/service.go:205:                parseThumbnailUrls                      100.0%
github.com/ibeloyar/gophprofile/internal/worker/worker.go:20:                   Run                                     0.0%
github.com/ibeloyar/gophprofile/pkg/logger/logger.go:13:                        New                                     85.7%
github.com/ibeloyar/gophprofile/pkg/logger/logger.go:28:                        LoggingMiddleware                       100.0%
github.com/ibeloyar/gophprofile/pkg/logger/logger.go:68:                        Write                                   100.0%
github.com/ibeloyar/gophprofile/pkg/logger/logger.go:76:                        WriteHeader                             100.0%
github.com/ibeloyar/gophprofile/pkg/resizer/resizer.go:17:                      Resize                                  88.9%
total:                                                                          (statements)                            50.3%
```