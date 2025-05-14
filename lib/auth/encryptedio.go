package auth

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	"filippo.io/age"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
)

type SessionRecordingConfigGetter interface {
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

type DecryptionKeyGetter interface {
	FindDecryptionKey(ctx context.Context, publicKeys ...[]byte) (*types.EncryptionKeyPair, error)
}

type EncryptedIO struct {
	srcGetter           SessionRecordingConfigGetter
	decryptionKeyGetter DecryptionKeyGetter
}

var _ events.EncryptionWrapper = (*EncryptedIO)(nil)
var _ events.DecryptionWrapper = (*EncryptedIO)(nil)

func NewEncryptedIO(srcgetter SessionRecordingConfigGetter, decryptionKeyGetter DecryptionKeyGetter) *EncryptedIO {
	return &EncryptedIO{
		srcGetter:           srcgetter,
		decryptionKeyGetter: decryptionKeyGetter,
	}
}

func (e *EncryptedIO) WithEncryption(writer io.WriteCloser) (io.WriteCloser, error) {
	if e.srcGetter == nil {
		return writer, nil
	}

	ctx := context.TODO()
	src, err := e.srcGetter.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	encrypter := NewEncryptionWrapper(src)
	w, err := encrypter.WithEncryption(writer)
	return w, trace.Wrap(err)
}

func (e *EncryptedIO) WithDecryption(reader io.Reader) (io.Reader, error) {
	if e.decryptionKeyGetter == nil {
		return reader, nil
	}
	ctx := context.TODO()
	pair, err := e.decryptionKeyGetter.FindDecryptionKey(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ident, err := age.ParseX25519Identity(string(pair.PrivateKey))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := age.Decrypt(reader, ident)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return r, nil
}

type EncryptionWrapper struct {
	config types.SessionRecordingConfig
}

var _ events.EncryptionWrapper = (*EncryptionWrapper)(nil)

func NewEncryptionWrapper(sessionRecordingConfig types.SessionRecordingConfig) *EncryptionWrapper {
	return &EncryptionWrapper{
		config: sessionRecordingConfig,
	}
}

func (s *EncryptionWrapper) WithEncryption(writer io.WriteCloser) (io.WriteCloser, error) {
	if !s.config.GetEncrypted() {
		return writer, nil
	}

	var recipients []age.Recipient
	for _, key := range s.config.GetStatus().EncryptionKeys {
		recipient, err := age.ParseX25519Recipient(string(key.PublicKey))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		recipients = append(recipients, recipient)
	}

	w, err := age.Encrypt(writer, recipients...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return w, nil
}
