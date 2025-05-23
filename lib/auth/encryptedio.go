package auth

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	"filippo.io/age"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/events"
)

type SessionRecordingConfigGetter interface {
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

type EncryptedIO struct {
	srcGetter SessionRecordingConfigGetter
	keyFinder recordingencryption.DecryptionKeyFinder
}

var _ events.EncryptionWrapper = (*EncryptedIO)(nil)
var _ events.DecryptionWrapper = (*EncryptedIO)(nil)

func NewEncryptedIO(srcgetter SessionRecordingConfigGetter, decryptionKeyGetter recordingencryption.DecryptionKeyFinder) *EncryptedIO {
	return &EncryptedIO{
		srcGetter: srcgetter,
		keyFinder: decryptionKeyGetter,
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
	if e.keyFinder == nil {
		return reader, nil
	}

	ident := recordingencryption.NewRecordingIdentity(e.keyFinder)
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
		recipient, err := recordingencryption.ParseRecordingRecipient(string(key.PublicKey))
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
