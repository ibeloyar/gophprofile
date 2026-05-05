package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates and returns a production-ready SugaredLogger with Info level.
// Disables stacktraces for cleaner logs. Returns configured logger or error.
func New() (*zap.SugaredLogger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	config.DisableStacktrace = true

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger.Sugar(), nil
}

// LoggingMiddleware returns HTTP middleware that logs all requests with structured fields:
// method, URI, status code, response size, and duration. Captures response writer metrics.
func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &loggingResponseWriter{
				ResponseWriter: w,
				responseData: &responseData{
					status: http.StatusOK,
					size:   0,
				}}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			logger.Infow("HTTP request",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Int("status", rw.responseData.status),
				zap.Int("size", rw.responseData.size),
				zap.Duration("duration", duration),
			)
		})
	}
}

// responseData holds captured HTTP response metrics.
type responseData struct {
	status int
	size   int
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code and response size.
// Delegates actual writing to wrapped ResponseWriter while tracking metrics.
type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// Write writes response body and tracks total bytes written.
// Delegates to wrapped ResponseWriter.Write.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b) // write the response using the original http.ResponseWriter
	r.responseData.size += size            // grab the size
	return size, err
}

// WriteHeader writes HTTP status code and captures it for logging.
// Delegates to wrapped ResponseWriter.WriteHeader.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode) // write the status code using the original http.ResponseWriter
	r.responseData.status = statusCode       // capture the status code
}
