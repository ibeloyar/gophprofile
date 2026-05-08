// s3_test.go
package s3

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	minioImage        = "minio/minio:latest"
	minioAccessKey    = "minioadmin"
	minioSecretKey    = "minioadmin123"
	bucketWaitTimeout = 60 * time.Second
)

type MinioContainer struct {
	Container testcontainers.Container
	Endpoint  string
	AccessKey string
	SecretKey string
}

func SetupMinIOContainer(t *testing.T) *MinioContainer {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        minioImage,
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": minioAccessKey,
			"MINIO_SECRET_KEY": minioSecretKey,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("API: http://").
				WithStartupTimeout(bucketWaitTimeout),
			wait.ForListeningPort("9000/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	time.Sleep(2 * time.Second)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
	require.NoError(t, err)

	var healthErr error
	for i := 0; i < 10; i++ {
		_, err := minioClient.BucketExists(ctx, "test")
		if err == nil {
			healthErr = nil
			break
		}
		healthErr = err
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, healthErr)

	return &MinioContainer{
		Container: container,
		Endpoint:  endpoint,
		AccessKey: minioAccessKey,
		SecretKey: minioSecretKey,
	}
}

func (m *MinioContainer) Teardown(t *testing.T) {
	err := m.Container.Terminate(context.Background())
	require.NoError(t, err)
}

func TestNew_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, avatarsS3BuckedName, client.bucket)

	ctx := context.Background()
	exists, err := client.client.BucketExists(ctx, avatarsS3BuckedName)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestNew_InvalidEndpoint(t *testing.T) {
	client, err := New("invalid-endpoint:9000", "test", "test")
	require.Error(t, err)
	require.Nil(t, client)
}

func TestClient_Health_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	err = client.Health()
	require.NoError(t, err)
}

func TestClient_UploadAndDownload_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	ctx := context.Background()
	testCases := []struct {
		name        string
		objectKey   string
		contentType string
		data        []byte
	}{
		{
			name:        "PNG image",
			objectKey:   "avatars/user1/avatar.png",
			contentType: "image/png",
			data:        []byte("fake png data"),
		},
		{
			name:        "JPEG image",
			objectKey:   "avatars/user2/photo.jpg",
			contentType: "image/jpeg",
			data:        []byte("fake jpeg data"),
		},
		{
			name:        "Empty file",
			objectKey:   "avatars/user3/empty.txt",
			contentType: "text/plain",
			data:        []byte{},
		},
		{
			name:        "Large file",
			objectKey:   "avatars/user4/large.bin",
			contentType: "application/octet-stream",
			data:        bytes.Repeat([]byte("A"), 1024*1024), // 1MB
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.Upload(ctx, tc.objectKey, tc.contentType, tc.data)
			require.NoError(t, err)

			downloadedData, contentType, err := client.Download(ctx, tc.objectKey)
			require.NoError(t, err)
			assert.Equal(t, tc.data, downloadedData)
			assert.Equal(t, tc.contentType, contentType)

			obj, err := client.client.GetObject(ctx, client.bucket, tc.objectKey, minio.GetObjectOptions{})
			require.NoError(t, err)
			defer obj.Close()

			stat, err := obj.Stat()
			require.NoError(t, err)
			assert.Equal(t, int64(len(tc.data)), stat.Size)
			assert.Equal(t, tc.contentType, stat.ContentType)
		})
	}
}

func TestClient_Download_NotFound_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	ctx := context.Background()
	_, _, err = client.Download(ctx, "nonexistent/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key does not exist")
}

func TestClient_DeleteObjects_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	ctx := context.Background()

	testFiles := []struct {
		key  string
		data []byte
	}{
		{"avatars/user1/avatar1.png", []byte("data1")},
		{"avatars/user1/avatar2.png", []byte("data2")},
		{"avatars/user2/avatar3.png", []byte("data3")},
	}

	for _, file := range testFiles {
		err := client.Upload(ctx, file.key, "image/png", file.data)
		require.NoError(t, err)
	}

	for _, file := range testFiles {
		_, _, err := client.Download(ctx, file.key)
		require.NoError(t, err)
	}

	keysToDelete := []string{testFiles[0].key, testFiles[1].key}
	err = client.DeleteObjects(ctx, keysToDelete)
	require.NoError(t, err)

	for i, file := range testFiles {
		_, _, err := client.Download(ctx, file.key)
		if i < 2 {
			require.Error(t, err, "file should be deleted: %s", file.key)
		} else {
			require.NoError(t, err, "file should still exist: %s", file.key)
		}
	}
}

func TestClient_DeleteObjects_EmptyList_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	err = client.DeleteObjects(context.Background(), []string{})
	require.NoError(t, err)
}

func TestClient_ConcurrentOperations_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	ctx := context.Background()
	numOperations := 10
	errChan := make(chan error, numOperations)

	for i := 0; i < numOperations; i++ {
		go func(idx int) {
			key := fmt.Sprintf("avatars/concurrent/file%d.png", idx)
			data := []byte(fmt.Sprintf("data%d", idx))
			err := client.Upload(ctx, key, "image/png", data)
			errChan <- err
		}(i)
	}

	for i := 0; i < numOperations; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("avatars/concurrent/file%d.png", i)
		expectedData := []byte(fmt.Sprintf("data%d", i))
		downloaded, _, err := client.Download(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, expectedData, downloaded)
	}
}

func TestClient_UploadWithSpecialCharacters_WithRealMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := SetupMinIOContainer(t)
	defer container.Teardown(t)

	client, err := New(container.Endpoint, container.AccessKey, container.SecretKey)
	require.NoError(t, err)

	ctx := context.Background()
	testCases := []struct {
		key  string
		data []byte
	}{
		{"avatars/user with spaces/avatar.png", []byte("space in path")},
		{"avatars/user/avatar with spaces.png", []byte("space in filename")},
		{"avatars/user/аватар/avatar.png", []byte("russian text data")},
		{"avatars/user/avatar_!@#$%.png", []byte("special chars")},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			err := client.Upload(ctx, tc.key, "image/png", tc.data)
			require.NoError(t, err)

			downloaded, _, err := client.Download(ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.data, downloaded)
		})
	}
}
