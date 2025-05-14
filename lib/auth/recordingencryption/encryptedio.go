package recordingencryption

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	"filippo.io/age"

	"github.com/gravitational/teleport/api/types"
)

// SessionRecordingConfigGetter returns the types.SessionRecordingConfig used to determine if
// encryption is enabled and retrieve the encryption keys to use
type SessionRecordingConfigGetter interface {
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

// EncryptedIO wraps a SessionRecordingConfigGetter and a recordingencryption.DecryptionKeyFinder in order
// to provide encryption and decryption wrapping backed by cluster resources
type EncryptedIO struct {
	srcGetter SessionRecordingConfigGetter
	keyFinder DecryptionKeyFinder
}

// NewEncryptedIO returns an EncryptedIO configured with the given SessionRecordingConfigGetter and
// recordingencryption.DecryptionKeyFinder
func NewEncryptedIO(srcgetter SessionRecordingConfigGetter, decryptionKeyGetter DecryptionKeyFinder) *EncryptedIO {
	return &EncryptedIO{
		srcGetter: srcgetter,
		keyFinder: decryptionKeyGetter,
	}
}

// WithEncryption wraps the given io.WriteCloser with encryption using the keys present in the
// retrieved types.SessionRecordingConfig
func (e *EncryptedIO) WithEncryption(ctx context.Context, writer io.WriteCloser) (io.WriteCloser, error) {
	if e.srcGetter == nil {
		return writer, nil
	}

	src, err := e.srcGetter.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	encrypter := NewEncryptionWrapper(src)
	w, err := encrypter.WithEncryption(ctx, writer)
	return w, trace.Wrap(err)
}

// WithDecryption wraps the given io.Reader with decryption using the recordingencryption.RecordingIdentity. This
// will dynamically search for an accessible decryption key using the provided recordingencryption.DecryptionKeyFinder
// in order to perform decryption
func (e *EncryptedIO) WithDecryption(reader io.Reader) (io.Reader, error) {
	if e.keyFinder == nil {
		return reader, nil
	}

	ident := NewRecordingIdentity(e.keyFinder)
	r, err := age.Decrypt(reader, ident)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return r, nil
}

// EncryptionWrapper provides a wrapper for recording data using the keys present in the given
// types.SessionRecordingConfig
type EncryptionWrapper struct {
	config types.SessionRecordingConfig
}

// NewEncryptionWrapper returns a new EncryptionWrapper backed by the given types.SessionRecordingConfig
func NewEncryptionWrapper(sessionRecordingConfig types.SessionRecordingConfig) *EncryptionWrapper {
	return &EncryptionWrapper{
		config: sessionRecordingConfig,
	}
}

// WithEncryption wraps the given io.WriteCloser with encryption using the keys present in the
// configured types.SessionRecordingConfig
func (s *EncryptionWrapper) WithEncryption(ctx context.Context, writer io.WriteCloser) (io.WriteCloser, error) {
	if !s.config.GetEncrypted() {
		return writer, nil
	}

	var recipients []age.Recipient
	for _, key := range s.config.GetEncryptionKeys() {
		recipient, err := ParseRecordingRecipient(string(key.PublicKey))
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
