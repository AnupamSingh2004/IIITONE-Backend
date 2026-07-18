package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinioStore_PutGetDeleteExists(t *testing.T) {
	endpoint := os.Getenv("STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("STORAGE_ENDPOINT not set, skipping integration test")
	}

	store, err := NewMinioStore(Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
		SecretKey: os.Getenv("STORAGE_SECRET_KEY"),
		Bucket:    os.Getenv("STORAGE_BUCKET"),
		UseSSL:    false,
	})
	require.NoError(t, err)

	ctx := context.Background()
	key := "test/hello.txt"
	content := []byte("hello iiitone")

	require.NoError(t, store.Put(ctx, key, bytes.NewReader(content), int64(len(content))))

	exists, err := store.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, exists)

	reader, err := store.Get(ctx, key)
	require.NoError(t, err)
	defer reader.Close()
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, content, got)

	require.NoError(t, store.Delete(ctx, key))
	exists, err = store.Exists(ctx, key)
	require.NoError(t, err)
	require.False(t, exists)
}
