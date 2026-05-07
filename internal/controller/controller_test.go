package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	_ "image"     // for getDimensions in service/helpers
	_ "image/png" // image.DecodeConfig

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/ibeloyar/gophprofile/internal/service/mocks"
)

type MultipartBody struct {
	io.Reader
	ContentType string
}

func createPNGMultipartBody(filename string, fileSize int) MultipartBody {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", "image/png")

	filePart, _ := writer.CreatePart(header)

	pngData, _ := os.ReadFile("./tests/test.png")

	filePart.Write(pngData)
	writer.Close()

	return MultipartBody{
		Reader:      body,
		ContentType: writer.FormDataContentType(),
	}
}

func createTestController(t *testing.T, service *service.MockService) *Controller {
	logger := zaptest.NewLogger(t).Sugar()

	return New(logger, service)
}

func TestHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().Health().Return(&model.HealthResponse{
		Postgresql: true,
		Minio:      true,
		RabbitMQ:   true,
	})

	httpController := createTestController(t, mockService)
	rr := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	httpController.Health(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUploadAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	mockService := service.NewMockService(ctrl)

	mockService.EXPECT().UploadAvatar(gomock.Any(), userID, gomock.Any()).Return(&model.AvatarCreateInfo{
		ID:               avatarID,
		UserID:           userID,
		CreatedAt:        time.Now(),
		ProcessingStatus: string(model.ProcessingOpProcessing),
	}, nil).Times(1)

	httpController := createTestController(t, mockService)

	body := createPNGMultipartBody("avatar.png", 1024)

	req := httptest.NewRequest("POST", "/api/v1/avatars", body.Reader)
	req.Header.Set("Content-Type", body.ContentType)
	req.Header.Set("X-User-ID", "1")

	rr := httptest.NewRecorder()
	httpController.UploadAvatar(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestUploadAvatar_NoFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	httpController := createTestController(t, mockService)

	req := httptest.NewRequest("POST", "/api/v1/avatars", nil)
	req.Header.Set("Content-Type", "multipart/form-data;boundary=")
	req.Header.Set("X-User-ID", "1")

	rr := httptest.NewRecorder()
	httpController.UploadAvatar(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "file required")
}

func TestUploadAvatar_Internal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := "1"
	avatarID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	mockService := service.NewMockService(ctrl)

	mockService.EXPECT().UploadAvatar(gomock.Any(), userID, gomock.Any()).Return(&model.AvatarCreateInfo{
		ID:               avatarID,
		UserID:           userID,
		CreatedAt:        time.Now(),
		ProcessingStatus: string(model.ProcessingOpProcessing),
	}, errors.New("internal error")).Times(1)

	httpController := createTestController(t, mockService)

	body := createPNGMultipartBody("avatar.png", 1024)

	req := httptest.NewRequest("POST", "/api/v1/avatars", body.Reader)
	req.Header.Set("Content-Type", body.ContentType)
	req.Header.Set("X-User-ID", "1")

	rr := httptest.NewRecorder()
	httpController.UploadAvatar(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDownloadAvatar_UserIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := "123"

	mockService := service.NewMockService(ctrl)

	r := httptest.NewRequest(http.MethodGet, "/avatars/123", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", avatarID)
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	c.DownloadAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "X-User-ID required")
}

func TestDownloadAvatar_AvatarNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DownloadAvatar(gomock.Any(), "123", "user1").
		Return(nil, "", errors.New("key does not exist"))

	r := httptest.NewRequest(http.MethodGet, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	w := httptest.NewRecorder()
	c.DownloadAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp model.AvatarNotFoundError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "Avatar not found", errResp.Error)
}

func TestDownloadAvatar_DownloadFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DownloadAvatar(gomock.Any(), "123", "user1").
		Return(nil, "", errors.New("s3 internal error"))

	r := httptest.NewRequest(http.MethodGet, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	w := httptest.NewRecorder()
	c.DownloadAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "download failed")
}

func TestDownloadAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pngData, _ := os.ReadFile("./tests/test.png")

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DownloadAvatar(gomock.Any(), "123", "user1").
		Return(pngData, "image/png", nil)

	r := httptest.NewRequest(http.MethodGet, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	w := httptest.NewRecorder()
	c.DownloadAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ct := resp.Header.Get("Content-Type")
	fmt.Println(ct)
	assert.Contains(t, ct, "image/png")

	assert.Equal(t, "max-age=86400", resp.Header.Get("Cache-Control"))

	etag := resp.Header.Get("ETag")
	assert.NotEmpty(t, etag)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, pngData, body)
}

func TestGetAvatarMeta_UserIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodGet, "/avatars/123/meta", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c.GetAvatarMeta(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "X-User-ID required")
}

func TestGetAvatarMeta_GetAvatarMetaError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		GetAvatarMeta(gomock.Any(), "123", "user1").
		Return(nil, errors.New("db error"))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodGet, "/avatars/123/meta", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.GetAvatarMeta(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "failed to get avatar")
}

func TestGetAvatarMeta_AvatarNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		GetAvatarMeta(gomock.Any(), "123", "user1").
		Return(nil, nil)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodGet, "/avatars/123/meta", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.GetAvatarMeta(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp model.AvatarNotFoundError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "Avatar not found", errResp.Error)
}

func TestGetAvatarMeta_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	avatarID := uuid.New()
	avatar := &model.AvatarMeta{
		ID:     avatarID,
		UserID: "user1",
		Dimensions: model.AvatarMetaDimensions{
			Width:  300,
			Height: 400,
		},
	}

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		GetAvatarMeta(gomock.Any(), avatarID.String(), "user1").
		Return(avatar, nil)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/avatars/%s/meta", avatarID.String()), nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", avatarID.String())
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.GetAvatarMeta(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result model.AvatarMeta
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, avatar, &result)
}

func TestDeleteAvatar_UserIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodDelete, "/avatars/123", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	c.DeleteAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "X-User-ID required")
}

func TestDeleteAvatar_AvatarNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DeleteAvatar(gomock.Any(), "123", "user1").
		Return(model.ErrAvatarNotFound)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodDelete, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.DeleteAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp model.AvatarNotFoundError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "Avatar not found", errResp.Error)
}

func TestDeleteAvatar_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DeleteAvatar(gomock.Any(), "123", "user1").
		Return(errors.New("db error"))

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodDelete, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.DeleteAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "failed to delete avatar")
}

func TestDeleteAvatar_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := service.NewMockService(ctrl)
	mockService.EXPECT().
		DeleteAvatar(gomock.Any(), "123", "user1").
		Return(nil)

	c := &Controller{
		service: mockService,
		lg:      zaptest.NewLogger(t).Sugar(),
	}

	r := httptest.NewRequest(http.MethodDelete, "/avatars/123", nil)
	r.Header.Set("X-User-ID", "user1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", "123")
	req := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	c.DeleteAvatar(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 0, len(body))
}
