package recordingencryption_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"filippo.io/age"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
)

func TestRecordingAgePlugin(t *testing.T) {
	keyStore := newFakeKeyStore()
	recordingIdentity := recordingencryption.NewRecordingIdentity(keyStore)

	ident, err := keyStore.generateIdentity()
	require.NoError(t, err)

	recipient, err := recordingencryption.ParseRecordingRecipient(ident.Recipient().String())
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	writer, err := age.Encrypt(out, recipient)
	require.NoError(t, err)

	msg := []byte("testing age plugin for session recordings")
	_, err = writer.Write(msg)
	require.NoError(t, err)

	// writer must be closed to ensure data is flushed
	err = writer.Close()
	require.NoError(t, err)

	reader, err := age.Decrypt(out, recordingIdentity)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, msg, plaintext)

	// running the same test with the raw recipient should fail because the
	// the extra stanza added by RecordingRecipient won't be present and
	// the private key can't be found
	out.Reset()
	writer, err = age.Encrypt(out, ident.Recipient())
	require.NoError(t, err)
	_, err = writer.Write(msg)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	_, err = age.Decrypt(out, recordingIdentity)
	require.Error(t, err)
}

type fakeKeyStore struct {
	keys map[string]string
}

func newFakeKeyStore() *fakeKeyStore {
	return &fakeKeyStore{
		keys: make(map[string]string),
	}
}

func (f *fakeKeyStore) FindDecryptionKey(publicKeys ...[]byte) (*types.EncryptionKeyPair, error) {
	for _, pubKey := range publicKeys {
		key, ok := f.keys[string(pubKey)]
		if !ok {
			continue
		}

		return &types.EncryptionKeyPair{
			PrivateKey: []byte(key),
			PublicKey:  pubKey,
		}, nil
	}

	return nil, errors.New("no accessible decryption key found")
}

func (f *fakeKeyStore) generateIdentity() (*age.X25519Identity, error) {
	ident, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, err
	}

	f.keys[ident.Recipient().String()] = ident.String()
	return ident, nil
}
