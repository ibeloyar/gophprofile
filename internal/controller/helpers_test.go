package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ibeloyar/gophprofile/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func createTestLogger(t *testing.T) (*zap.SugaredLogger, error) {
	t.Helper()

	lg, err := logger.New()
	if err != nil {
		log.Fatal(err)
	}
	defer lg.Sync()

	return lg, err
}

func TestReadBody_TextPlain_String_Success(t *testing.T) {
	body := "test string"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	got, err := readBody[string](req)

	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestReadBody_TextPlain_String_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")

	got, err := readBody[string](req)

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestReadBody_TextPlain_NonString_Fail(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test"))
	req.Header.Set("Content-Type", "text/plain")

	type TestStruct struct{ Field string }

	_, err := readBody[TestStruct](req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body: text/plain")
}

func TestReadBody_JSON_Success(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}
	expected := TestStruct{Name: "test"}

	bodyJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")

	got, err := readBody[TestStruct](req)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestReadBody_JSON_Invalid_Fail(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"invalid": "json"`))
	req.Header.Set("Content-Type", "application/json")

	type TestStruct struct{ Name string }

	_, err := readBody[TestStruct](req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body application/json")
}

func TestReadBody_ReadError(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/", errorReader{})
	req.Header.Set("Content-Type", "application/json")

	type TestStruct struct{ Name string }

	_, err := readBody[TestStruct](req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")
}

func TestReadBody_NoContentType_JSON(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}
	expected := TestStruct{Name: "test"}

	bodyJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyJSON))

	got, err := readBody[TestStruct](req)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestReadBody_TextPlainWithCharset(t *testing.T) {
	body := "test string"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	got, err := readBody[string](req)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestWriteJSON_Success(t *testing.T) {
	lg, err := createTestLogger(t)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeJSON(w, lg, data, http.StatusOK)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, data, got)
}

func TestWriteJSON_MarshalError(t *testing.T) {
	lg, err := createTestLogger(t)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := make(chan int)

	writeJSON(w, lg, data, http.StatusOK)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	assert.Contains(t, w.Body.String(), "Internal Server Error")
}

func TestReadAvatarFile_HeaderSizeTooLarge(t *testing.T) {
	body := strings.NewReader("--boundary\r\n" +
		`Content-Disposition: form-data; name="file"; filename="big.jpg"` + "\r\n" +
		"Content-Type: image/jpeg\r\n" +
		"\r\n" +
		string(make([]byte, maxFileSize+1)) +
		"\r\n--boundary--\r\n",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/",
		body,
	)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	req.ParseMultipartForm(maxFileSize + 100)

	avatarFile, err := readAvatarFile(req)
	require.Error(t, err)
	assert.Nil(t, avatarFile)
	assert.Equal(t, ErrFileTooLarge, err)
}

func TestReadAvatarFile_InvalidContentType(t *testing.T) {
	body := strings.NewReader("--boundary\r\n" +
		`Content-Disposition: form-data; name="file"; filename="test.txt"` + "\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"some text data" +
		"\r\n--boundary--\r\n",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/",
		body,
	)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.ParseMultipartForm(10 << 20) // 10 MiB

	avatarFile, err := readAvatarFile(req)
	require.Error(t, err)
	assert.Nil(t, avatarFile)
	assert.Equal(t, ErrFileInvalidFormat, err)
}

func TestGetDimensions_ValidPNG(t *testing.T) {
	// PNG 500x200
	img := image.NewRGBA(image.Rect(0, 0, 500, 200))
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	require.NoError(t, err)

	r := bytes.NewReader(buf.Bytes())

	dim, err := getDimensions(r)
	require.NoError(t, err)
	assert.Equal(t, 500, dim.Width)
	assert.Equal(t, 200, dim.Height)
}

func TestGetDimensions_EmptyReader(t *testing.T) {
	r := bytes.NewReader(nil)

	dim, err := getDimensions(r)
	require.Error(t, err)
	assert.Nil(t, dim)
}
