package logger

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew_Valid(t *testing.T) {
	logger, err := New()

	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Info("test")
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer

	config := zap.NewDevelopmentConfig()
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(config.EncoderConfig),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)

	logger := zap.New(core).Sugar()

	middleware := LoggingMiddleware(logger)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "hello", w.Body.String())

	logOutput := buf.String()
	assert.Contains(t, logOutput, "uri\": \"/test")
	assert.Contains(t, logOutput, "method\": \"GET")
	assert.Contains(t, logOutput, "status\": 201")
	assert.Contains(t, logOutput, "size\": 5")
}

func TestLoggingMiddleware_StatusOK(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	logger := zap.New(core).Sugar()

	middleware := LoggingMiddleware(logger)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("POST", "/ok", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "status\": 200")
	assert.Contains(t, logOutput, "size\": 2")
}

func TestLoggingMiddleware_ZeroSize(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	logger := zap.New(core).Sugar()

	middleware := LoggingMiddleware(logger)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("DELETE", "/empty", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "status\": 204")
	assert.Contains(t, logOutput, "size\": 0")
}

func TestLoggingMiddleware_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	logger := zap.New(core).Sugar()

	middleware := LoggingMiddleware(logger)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
		w.Write([]byte("world"))
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/multi", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "helloworld", w.Body.String())

	logOutput := buf.String()
	assert.Contains(t, logOutput, "size\": 10")
}

func TestLoggingResponseWriter_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := &loggingResponseWriter{
		ResponseWriter: recorder,
		responseData: &responseData{
			status: http.StatusOK,
			size:   0,
		},
	}

	size, err := rw.Write([]byte("test"))

	assert.NoError(t, err)
	assert.Equal(t, 4, size)
	assert.Equal(t, 4, rw.responseData.size)
	assert.Equal(t, "test", recorder.Body.String())
}

func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := &loggingResponseWriter{
		ResponseWriter: recorder,
		responseData: &responseData{
			status: http.StatusOK,
			size:   0,
		},
	}

	rw.WriteHeader(http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, rw.responseData.status)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestLoggingMiddleware_LongRequest(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	logger := zap.New(core).Sugar()

	middleware := LoggingMiddleware(logger)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "duration\": ")
}
