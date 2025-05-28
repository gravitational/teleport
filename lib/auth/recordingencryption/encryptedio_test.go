package recordingencryption_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
)

func TestEncryptedIO(t *testing.T) {
	ctx := context.Background()
	keyStore := newFakeKeyStore()
	ident, err := keyStore.generateIdentity()
	require.NoError(t, err)

	srcGetter, err := newFakeSRCGetter(true, []*types.AgeEncryptionKey{
		&types.AgeEncryptionKey{
			PublicKey: []byte(ident.Recipient().String()),
		},
	})
	require.NoError(t, err)

	encryptedIO := recordingencryption.NewEncryptedIO(srcGetter, keyStore)

	out := bytes.NewBuffer(nil)
	writer, err := encryptedIO.WithEncryption(ctx, &writeCloser{Writer: out})
	require.NoError(t, err)

	msg := []byte("testing encrypted IO")
	_, err = writer.Write(msg)

	// writer must be closed to ensure data is flushed
	err = writer.Close()
	require.NoError(t, err)

	reader, err := encryptedIO.WithDecryption(out)
	require.NoError(t, err)

	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, msg, plaintext)
}

type fakeSRCGetter struct {
	config types.SessionRecordingConfig
}

func newFakeSRCGetter(encrypted bool, keys []*types.AgeEncryptionKey) (*fakeSRCGetter, error) {
	spec := types.SessionRecordingConfigSpecV2{
		Encryption: &types.SessionRecordingEncryptionConfig{
			Enabled: true,
		},
	}

	config, err := types.NewSessionRecordingConfigFromConfigFile(spec)
	if err != nil {
		return nil, err
	}

	config.SetEncryptionKeys(slices.Values(keys))

	return &fakeSRCGetter{
		config: config,
	}, nil
}

func (f *fakeSRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return f.config, nil
}

type writeCloser struct {
	io.Writer
}

func (w *writeCloser) Close() error {
	return nil
}
