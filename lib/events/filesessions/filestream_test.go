package filesessions

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/session"

	"github.com/stretchr/testify/require"
)

func TestReserveUploadPart(t *testing.T) {
	ctx := context.Background()
	partNumber := int64(1)
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	err = handler.ReserveUploadPart(ctx, *upload, partNumber)
	require.NoError(t, err)

	fi, err := os.Stat(handler.partPath(*upload, partNumber))
	require.NoError(t, err)
	require.GreaterOrEqual(t, fi.Size(), int64(minUploadBytes))
}

func TestUploadPart(t *testing.T) {
	ctx := context.Background()
	partNumber := int64(1)
	dir := t.TempDir()
	expectedContent := []byte("upload part contents")

	handler, err := NewHandler(Config{
		Directory: dir,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	err = handler.ReserveUploadPart(ctx, *upload, partNumber)
	require.NoError(t, err)

	_, err = handler.UploadPart(ctx, *upload, partNumber, bytes.NewReader(expectedContent))
	require.NoError(t, err)

	partFile, err := os.Open(handler.partPath(*upload, partNumber))
	require.NoError(t, err)
	defer partFile.Close()

	partFileContent, err := io.ReadAll(partFile)
	require.NoError(t, err)
	require.True(t, bytes.Equal(expectedContent, partFileContent))
}
