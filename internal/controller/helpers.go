package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"

	"github.com/ibeloyar/gophprofile/internal/model"
	"go.uber.org/zap"
)

// readBody - reads and parses JSON and Text/Plain request body into a T struct
func readBody[T any](r *http.Request) (T, error) {
	var body T

	contentType := r.Header.Get("Content-Type")

	if contentType == "" {
		contentType = "application/json"
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return body, fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	if strings.HasPrefix(contentType, "text/plain") {
		switch any(body).(type) {
		case string:
			if len(bodyBytes) == 0 {
				return body, nil
			}

			return any(string(bodyBytes)).(T), nil
		default:
			return body, fmt.Errorf("failed to read request body: %s", contentType)
		}
	}

	if strings.HasPrefix(contentType, "application/json") {
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			return body, fmt.Errorf("failed to read request body %s: %w", contentType, err)
		}
	}

	return body, nil
}

// writeJSON - writes the response in JSON format and adds the Content-Type: application/json header
func writeJSON(w http.ResponseWriter, lg *zap.SugaredLogger, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response, err := json.Marshal(data)
	if err != nil {
		lg.Errorf("failed to parse request body: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Write(response)
}

var (
	ErrFileRequired      = errors.New("file required")
	ErrFileTooLarge      = errors.New("file too large")
	ErrFileInvalidFormat = errors.New("invalid file format")
)

// readAvatarFile -
func readAvatarFile(r *http.Request) (*model.AvatarFile, error) {
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, ErrFileRequired
	}
	defer file.Close()

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		return nil, ErrFileTooLarge
	}

	contentType := header.Header.Get("Content-Type")
	if !strings.Contains(supportedFormats, contentType) {
		return nil, ErrFileInvalidFormat
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if header.Size > maxFileSize || int64(len(data)) > maxFileSize {
		return nil, ErrFileTooLarge
	}

	// Перематываем file в начало для getDimensions
	file.Seek(0, 0)
	dimensions, err := getDimensions(file)
	if err != nil {
		return nil, err
	}

	return &model.AvatarFile{
		ContentType: contentType,
		Filename:    header.Filename,
		Width:       dimensions.Width,
		Height:      dimensions.Height,
		Size:        header.Size,
		Data:        data,
	}, nil
}

// getDimensions -
func getDimensions(r io.Reader) (*model.AvatarMetaDimensions, error) {
	config, _, err := image.DecodeConfig(r)
	if err != nil {
		return nil, err
	}
	return &model.AvatarMetaDimensions{
		Width:  config.Width,
		Height: config.Height,
	}, nil
}
