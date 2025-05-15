package auth

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	"filippo.io/age"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

type EncryptedIO struct {
	clusterConfig       services.ClusterConfiguration
	recordingEncryption services.RecordingEncryptionResolver
}

var _ events.EncryptedIO = (*EncryptedIO)(nil)

func NewEncryptedIO(clusterConfig services.ClusterConfiguration, recordingEncryption services.RecordingEncryptionResolver) *EncryptedIO {
	return &EncryptedIO{
		clusterConfig:       clusterConfig,
		recordingEncryption: recordingEncryption,
	}
}

func (e *EncryptedIO) WithEncryption(writer io.WriteCloser) (io.WriteCloser, error) {
	if e.clusterConfig == nil {
		return writer, nil
	}

	ctx := context.TODO()
	src, err := e.clusterConfig.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var recipients []age.Recipient
	for _, key := range src.GetStatus().EncryptionKeys {
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

func (e *EncryptedIO) WithDecryption(reader io.Reader) (io.Reader, error) {
	if e.recordingEncryption == nil {
		return reader, nil
	}
	ctx := context.TODO()
	pair, err := e.recordingEncryption.GetDecryptionKey(ctx, nil)
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
