package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	broker "github.com/ibeloyar/gophprofile/internal/repository/broker/mocks"
	s3 "github.com/ibeloyar/gophprofile/internal/repository/s3/mocks"
	storage "github.com/ibeloyar/gophprofile/internal/repository/storage/mocks"
)

func TestHealth_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().Health().Return(nil).Times(1)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Health().Return(nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().Health().Return(nil).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	health := srv.Health()

	require.True(t, health.RabbitMQ)
	require.True(t, health.Postgresql)
	require.True(t, health.Minio)
}

func TestHealth_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	healthErr := errors.New("health error")

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().Health().Return(healthErr).Times(1)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Health().Return(healthErr).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().Health().Return(healthErr).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	health := srv.Health()

	require.False(t, health.RabbitMQ)
	require.False(t, health.Postgresql)
	require.False(t, health.Minio)
}

func TestUploadAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.New()
	fileName := "file1"
	contentType := "image/jpeg"
	width := 512
	height := 512
	size := int64(1024)
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateAvatar(gomock.Any(), userID, fileName, contentType, width, height, size).
		Return(&model.AvatarCreateInfo{
			ID:               avatarID,
			UserID:           userID,
			CreatedAt:        time.Now(),
			ProcessingStatus: string(model.ProcessingOpProcessing),
		}, nil).Times(1)
	mockStorage.EXPECT().UpdateAvatarS3Key(gomock.Any(), avatarID.String(), objectKey).
		Return(nil).Times(1)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), objectKey, contentType, []byte{}).
		Return(nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishUpload(gomock.Any(), &model.AvatarUploadEvent{
		AvatarID: avatarID.String(),
		UserID:   userID,
		S3Key:    objectKey,
	}).Return(nil).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatar, err := srv.UploadAvatar(context.Background(), userID, &model.AvatarFile{
		Data:        []byte{},
		Filename:    fileName,
		Size:        size,
		ContentType: contentType,
		Height:      height,
		Width:       width,
	})

	require.NoError(t, err)
	require.Equal(t, avatar.ID, avatarID)
	require.Equal(t, avatar.UserID, userID)
}

func TestUploadAvatar_CreateAvatarError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.New()
	fileName := "file1"
	contentType := "image/jpeg"
	width := 512
	height := 512
	size := int64(1024)
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)
	createErr := errors.New("create avatar error")

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateAvatar(gomock.Any(), userID, fileName, contentType, width, height, size).
		Return(nil, createErr).Times(1)

	mockStorage.EXPECT().UpdateAvatarS3Key(gomock.Any(), avatarID.String(), objectKey).
		Return(nil).Times(0)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), objectKey, contentType, []byte{}).
		Return(nil).Times(0)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishUpload(gomock.Any(), gomock.Any()).
		Return(nil).Times(0)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatar, err := srv.UploadAvatar(context.Background(), userID, &model.AvatarFile{
		Data:        []byte{},
		Filename:    fileName,
		Size:        size,
		ContentType: contentType,
		Height:      height,
		Width:       width,
	})

	require.Error(t, err)
	require.Nil(t, avatar)
	require.EqualError(t, err, "failed db create avatar")
}

func TestUploadAvatar_S3Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.New()
	fileName := "file1"
	contentType := "image/jpeg"
	width := 512
	height := 512
	size := int64(1024)
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)
	s3Error := errors.New("s3 error")

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateAvatar(gomock.Any(), userID, fileName, contentType, width, height, size).
		Return(&model.AvatarCreateInfo{
			ID:               avatarID,
			UserID:           userID,
			CreatedAt:        time.Now(),
			ProcessingStatus: string(model.ProcessingOpProcessing),
		}, nil).Times(1)
	mockStorage.EXPECT().UpdateAvatarS3Key(gomock.Any(), avatarID.String(), objectKey).
		Return(nil).Times(0)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), objectKey, contentType, []byte{}).
		Return(s3Error).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishUpload(gomock.Any(), gomock.Any()).Return(nil).Times(0)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatar, err := srv.UploadAvatar(context.Background(), userID, &model.AvatarFile{
		Data:        []byte{},
		Filename:    fileName,
		Size:        size,
		ContentType: contentType,
		Height:      height,
		Width:       width,
	})

	require.Error(t, err)
	require.Nil(t, avatar)
	require.EqualError(t, err, "failed to s3 upload avatar")
}

func TestUploadAvatar_UpdateAvatarS3KeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.New()
	fileName := "file1"
	contentType := "image/jpeg"
	width := 512
	height := 512
	size := int64(1024)
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)
	updateS3KeyErr := errors.New("update s3 key error")

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateAvatar(gomock.Any(), userID, fileName, contentType, width, height, size).
		Return(&model.AvatarCreateInfo{
			ID:               avatarID,
			UserID:           userID,
			CreatedAt:        time.Now(),
			ProcessingStatus: string(model.ProcessingOpProcessing),
		}, nil).Times(1)
	mockStorage.EXPECT().UpdateAvatarS3Key(gomock.Any(), avatarID.String(), objectKey).
		Return(updateS3KeyErr).Times(1)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), objectKey, contentType, []byte{}).
		Return(nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishUpload(gomock.Any(), gomock.Any()).
		Return(nil).Times(0)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatar, err := srv.UploadAvatar(context.Background(), userID, &model.AvatarFile{
		Data:        []byte{},
		Filename:    fileName,
		Size:        size,
		ContentType: contentType,
		Height:      height,
		Width:       width,
	})

	require.Error(t, err)
	require.Nil(t, avatar)
	require.EqualError(t, err, "failed to storage update S3 key")
}

func TestUploadAvatar_PublishUploadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.New()
	fileName := "file1"
	contentType := "image/jpeg"
	width := 512
	height := 512
	size := int64(1024)
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)
	publishErr := errors.New("publish error")

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateAvatar(gomock.Any(), userID, fileName, contentType, width, height, size).
		Return(&model.AvatarCreateInfo{
			ID:               avatarID,
			UserID:           userID,
			CreatedAt:        time.Now(),
			ProcessingStatus: string(model.ProcessingOpProcessing),
		}, nil).Times(1)
	mockStorage.EXPECT().UpdateAvatarS3Key(gomock.Any(), avatarID.String(), objectKey).
		Return(nil).Times(1)

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), objectKey, contentType, []byte{}).
		Return(nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishUpload(gomock.Any(), gomock.Any()).
		Return(publishErr).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatar, err := srv.UploadAvatar(context.Background(), userID, &model.AvatarFile{
		Data:        []byte{},
		Filename:    fileName,
		Size:        size,
		ContentType: contentType,
		Height:      height,
		Width:       width,
	})

	require.Error(t, err)
	require.Nil(t, avatar)
	require.EqualError(t, err, "failed to publish upload avatar")
}

func TestDownloadAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"

	var avatarData []byte

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockStorage := storage.NewMockStorage(ctrl)
	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Download(gomock.Any(), fmt.Sprintf("%s/%s", userID, avatarID)).Return(avatarData, "image/png", nil).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatarBytes, contentType, err := srv.DownloadAvatar(context.Background(), avatarID.String(), userID)
	require.NoError(t, err)
	require.Equal(t, avatarData, avatarBytes)
	require.Equal(t, contentType, "image/png")
}

func TestDownloadAvatar_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"
	var avatarData []byte
	contentType := "image/png"
	expectedError := errors.New("error")

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockStorage := storage.NewMockStorage(ctrl)
	mockS3 := s3.NewMockS3Storage(ctrl)
	mockS3.EXPECT().Download(gomock.Any(), fmt.Sprintf("%s/%s", userID, avatarID)).Return(avatarData, contentType, expectedError).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	_, _, err := srv.DownloadAvatar(context.Background(), avatarID.String(), userID)
	require.Error(t, err)
	require.EqualError(t, err, expectedError.Error())
}

func TestGetAvatarMeta_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockBrokerPublisher := broker.NewMockPublisher(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().GetAvatarMeta(gomock.Any(), avatarID.String(), userID).Return(&model.AvatarMeta{
		ID:     avatarID,
		UserID: userID,
	}, nil).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatarMeta, err := srv.GetAvatarMeta(context.Background(), avatarID.String(), userID)
	require.NoError(t, err)
	require.Equal(t, avatarID, avatarMeta.ID)
	require.Equal(t, userID, avatarMeta.UserID)
}

func TestGetAvatarMeta_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"
	internalErr := errors.New("internal error")

	mockS3 := s3.NewMockS3Storage(ctrl)
	mockBrokerPublisher := broker.NewMockPublisher(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().GetAvatarMeta(gomock.Any(), avatarID.String(), userID).Return(&model.AvatarMeta{
		ID:     avatarID,
		UserID: userID,
	}, internalErr).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	avatarMeta, err := srv.GetAvatarMeta(context.Background(), avatarID.String(), userID)
	require.Nil(t, avatarMeta)
	require.Error(t, err)
	require.EqualError(t, err, internalErr.Error())
}

func TestDeleteAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"

	mockS3 := s3.NewMockS3Storage(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().SoftDeleteAvatar(gomock.Any(), avatarID.String(), userID).Return(nil).Times(1)
	mockStorage.EXPECT().GetAvatarByID(gomock.Any(), avatarID.String(), userID).Return(&model.Avatar{
		ID:              avatarID,
		UserID:          userID,
		ThumbnailS3Keys: rawJSON(t, `{"100x100":"url_100x100"}`),
	}, nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishDelete(gomock.Any(), &model.AvatarDeleteEvent{
		AvatarID: avatarID.String(),
		S3Keys:   []string{"1/url_100x100"},
	}).Return(nil).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	err := srv.DeleteAvatar(context.Background(), avatarID.String(), userID)
	require.NoError(t, err)
}

func TestDeleteAvatar_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"

	mockS3 := s3.NewMockS3Storage(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().GetAvatarByID(gomock.Any(), avatarID.String(), userID).Return(nil, nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	err := srv.DeleteAvatar(context.Background(), avatarID.String(), userID)
	require.Error(t, err)
	require.EqualError(t, err, model.ErrAvatarNotFound.Error())
}

func TestDeleteAvatar_Internal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"
	internalErr := errors.New("internal error")

	mockS3 := s3.NewMockS3Storage(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().GetAvatarByID(gomock.Any(), avatarID.String(), userID).
		Return(nil, internalErr).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	err := srv.DeleteAvatar(context.Background(), avatarID.String(), userID)
	require.Error(t, err)
	require.EqualError(t, err, internalErr.Error())
}

func TestDeleteAvatar_PublishErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	userID := "1"
	publishErr := errors.New("publish error")

	mockS3 := s3.NewMockS3Storage(ctrl)

	mockStorage := storage.NewMockStorage(ctrl)
	mockStorage.EXPECT().SoftDeleteAvatar(gomock.Any(), avatarID.String(), userID).Return(nil).Times(1)
	mockStorage.EXPECT().GetAvatarByID(gomock.Any(), avatarID.String(), userID).Return(&model.Avatar{
		ID:              avatarID,
		UserID:          userID,
		ThumbnailS3Keys: rawJSON(t, `{"100x100":"url_100x100"}`),
	}, nil).Times(1)

	mockBrokerPublisher := broker.NewMockPublisher(ctrl)
	mockBrokerPublisher.EXPECT().PublishDelete(gomock.Any(), &model.AvatarDeleteEvent{
		AvatarID: avatarID.String(),
		S3Keys:   []string{"1/url_100x100"},
	}).Return(publishErr).Times(1)

	srv := Service{
		lg:        zaptest.NewLogger(t).Sugar(),
		storage:   mockStorage,
		s3:        mockS3,
		publisher: mockBrokerPublisher,
	}

	err := srv.DeleteAvatar(context.Background(), avatarID.String(), userID)
	require.Error(t, err)
	require.EqualError(t, err, publishErr.Error())
}

func TestParseThumbnailUrls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userID  string
		raw     *json.RawMessage
		want    []string
		wantErr string
	}{
		{
			name:    "nil raw returns nil",
			userID:  "user1",
			raw:     nil,
			want:    nil,
			wantErr: "",
		},
		{
			name:   "empty map returns empty slice",
			userID: "user1",
			raw:    rawJSON(t, `{}`),
			want:   []string{},
		},
		{
			name:   "filters empty values and prefixes user id",
			userID: "user1",
			raw:    rawJSON(t, `{"a":"thumb1.jpg"}`),
			want:   []string{"user1/thumb1.jpg"},
		},
		{
			name:    "invalid json returns error",
			userID:  "user1",
			raw:     rawJSON(t, `{bad json}`),
			wantErr: "unmarshal thumbnails",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseThumbnailUrls(tt.userID, tt.raw)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func rawJSON(t *testing.T, s string) *json.RawMessage {
	t.Helper()

	rm := json.RawMessage(s)

	return &rm
}
