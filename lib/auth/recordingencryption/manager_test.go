// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package recordingencryption_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

type oaepDecrypter struct {
	crypto.Decrypter
	hash crypto.Hash
}

func (d oaepDecrypter) Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	return d.Decrypter.Decrypt(rand, msg, opts)
}

type fakeKeyStore struct {
	keyType   types.PrivateKeyType // abusing this field as a way to simulate different auth servers
	keys      map[string][]crypto.Decrypter
	currLabel types.KeyLabel
}

func (f *fakeKeyStore) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error) {
	decrypter, err := cryptosuites.GenerateDecrypterWithAlgorithm(cryptosuites.RSA4096)
	if err != nil {
		return nil, err
	}

	private, ok := decrypter.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("expected RSA private key")
	}

	privatePEM, err := keys.MarshalDecrypter(private)
	if err != nil {
		return nil, err
	}

	publicPEM, err := keys.MarshalPublicKey(private.Public())
	if err != nil {
		return nil, err
	}

	if f.keys == nil {
		f.keys = make(map[string][]crypto.Decrypter)
	}

	label := fmt.Sprintf("%d:%s", f.currLabel.Type, f.currLabel.Label)
	f.keys[label] = append(f.keys[label], private)

	return &types.EncryptionKeyPair{
		PrivateKey:     privatePEM,
		PublicKey:      publicPEM,
		PrivateKeyType: f.keyType,
		Hash:           uint32(crypto.SHA256),
	}, nil
}

func (f *fakeKeyStore) GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error) {
	if keyPair.PrivateKeyType != f.keyType {
		return nil, errors.New("could not access decrypter")
	}

	private, err := keys.ParsePrivateKey(keyPair.PrivateKey)
	if err != nil {
		return nil, err
	}

	decrypter, ok := private.Signer.(crypto.Decrypter)
	if !ok {
		return nil, errors.New("private key should have been a decrypter")
	}
	return oaepDecrypter{Decrypter: decrypter, hash: crypto.Hash(keyPair.Hash)}, nil
}

func (f *fakeKeyStore) FindDecryptersByLabels(ctx context.Context, labels ...*types.KeyLabel) ([]crypto.Decrypter, error) {
	var decrypters []crypto.Decrypter
	for _, label := range labels {
		lookup := fmt.Sprintf("%d:%s", label.Type, label.Label)
		decrypters = append(decrypters, f.keys[lookup]...)
	}

	return decrypters, nil
}

func newLocalBackend(
	t *testing.T,
) (context.Context, backend.Backend) {
	t.Parallel()
	ctx := t.Context()
	clock := clockwork.NewRealClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	bk := backend.NewSanitizer(mem)
	return ctx, bk
}

func newManagerConfig(t *testing.T, bk backend.Backend, keyType types.PrivateKeyType) recordingencryption.ManagerConfig {
	recordingEncryptionService, err := local.NewRecordingEncryptionService(bk)
	require.NoError(t, err)

	clusterConfigService, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)

	src := &types.SessionRecordingConfigV2{}
	require.NoError(t, src.CheckAndSetDefaults())
	src.Spec.Encryption = &types.SessionRecordingEncryptionConfig{
		Enabled: true,
	}

	return recordingencryption.ManagerConfig{
		Backend:       recordingEncryptionService,
		Cache:         recordingEncryptionService,
		ClusterConfig: clusterConfigService,
		KeyStore:      &fakeKeyStore{keyType: keyType},
		Logger:        utils.NewSlogLoggerForTests(),
		LockConfig: backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            bk,
				LockNameComponents: []string{"recording_encryption"},
				TTL:                10 * time.Second,
				RetryInterval:      100 * time.Millisecond,
			},
		},
	}
}

// resolve is a proxy to Manager.resolveRecordingEncryption through calling UpsertSessionRecordingConfig
func resolve(ctx context.Context, service services.RecordingEncryption, manager *recordingencryption.Manager) (*recordingencryptionv1.RecordingEncryption, types.SessionRecordingConfig, error) {
	req := types.SessionRecordingConfigV2{
		Spec: types.SessionRecordingConfigSpecV2{
			Encryption: &types.SessionRecordingEncryptionConfig{
				Enabled: true,
			},
		},
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	src, err := manager.UpsertSessionRecordingConfig(ctx, &req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	encryption, err := service.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return encryption, src, nil
}

func TestCreateUpdateSessionRecordingConfig(t *testing.T) {
	ctx, bk := newLocalBackend(t)

	config := newManagerConfig(t, bk, types.PrivateKeyType_RAW)
	manager, err := recordingencryption.NewManager(config)
	require.NoError(t, err)

	req := &types.SessionRecordingConfigV2{}
	require.NoError(t, req.CheckAndSetDefaults())
	req.Spec.Encryption = &types.SessionRecordingEncryptionConfig{
		Enabled: true,
	}

	// create should provision initial keypair and write public key to SRC
	src, err := manager.CreateSessionRecordingConfig(ctx, req)
	require.NoError(t, err)
	encryptionKeys := src.GetEncryptionKeys()
	require.Len(t, encryptionKeys, 1)

	encryption, err := config.Backend.GetRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys := encryption.GetSpec().GetActiveKeys()
	require.Len(t, activeKeys, 1)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)
	require.NotEmpty(t, activeKeys[0].RecordingEncryptionPair.PrivateKey)
	require.NotEmpty(t, activeKeys[0].RecordingEncryptionPair.PublicKey)

	// update should change nothing
	src, err = manager.UpdateSessionRecordingConfig(ctx, src)
	require.NoError(t, err)
	newEncryptionKeys := src.GetEncryptionKeys()
	require.ElementsMatch(t, newEncryptionKeys, encryptionKeys)

	encryption, err = config.Backend.GetRecordingEncryption(ctx)
	require.NoError(t, err)
	newActiveKeys := encryption.GetSpec().GetActiveKeys()
	require.ElementsMatch(t, newActiveKeys, activeKeys)
}

func TestResolveRecordingEncryption(t *testing.T) {
	// SETUP
	ctx, bk := newLocalBackend(t)

	managerABType := types.PrivateKeyType_AWS_KMS
	managerCType := types.PrivateKeyType_GCP_KMS

	configA := newManagerConfig(t, bk, managerABType)
	configB := configA
	configC := configA
	configC.KeyStore = &fakeKeyStore{keyType: managerCType}

	managerA, err := recordingencryption.NewManager(configA)
	require.NoError(t, err)

	managerB, err := recordingencryption.NewManager(configB)
	require.NoError(t, err)

	managerC, err := recordingencryption.NewManager(configC)
	require.NoError(t, err)

	service := configA.Backend

	// TEST
	// CASE: service A first evaluation initializes recording encryption resource
	encryption, src, err := resolve(ctx, service, managerA)
	require.NoError(t, err)
	initialKeys := encryption.GetSpec().GetActiveKeys()

	require.Len(t, initialKeys, 1)
	require.Len(t, src.GetEncryptionKeys(), 1)
	key := initialKeys[0]
	require.Equal(t, key.RecordingEncryptionPair.PublicKey, src.GetEncryptionKeys()[0].PublicKey)
	require.NotNil(t, key.RecordingEncryptionPair)

	// CASE: service B should have access to the same key
	encryption, src, err = resolve(ctx, service, managerB)
	require.NoError(t, err)

	activeKeys := encryption.GetSpec().ActiveKeys
	require.Len(t, src.GetEncryptionKeys(), 1)
	require.Equal(t, key.RecordingEncryptionPair.PublicKey, src.GetEncryptionKeys()[0].PublicKey)
	require.ElementsMatch(t, initialKeys, activeKeys)

	// service C should error without access to the current key
	_, _, err = resolve(ctx, service, managerC)
	require.Error(t, err)
}

func TestResolveRecordingEncryptionConcurrent(t *testing.T) {
	t.Skip()
	// SETUP
	ctx, bk := newLocalBackend(t)

	configA := newManagerConfig(t, bk, types.PrivateKeyType_RAW)
	configB := configA
	configC := configA

	recordingEncryptionService := configA.Backend
	managerA, err := recordingencryption.NewManager(configA)
	require.NoError(t, err)

	managerB, err := recordingencryption.NewManager(configB)
	require.NoError(t, err)

	serviceC, err := recordingencryption.NewManager(configC)
	require.NoError(t, err)

	service := configA.Backend
	resolveFn := func(manager *recordingencryption.Manager, wg *sync.WaitGroup) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resolve(ctx, service, manager)
			require.NoError(t, err)
		}()
	}

	wg := sync.WaitGroup{}
	resolveFn(managerA, &wg)
	resolveFn(managerB, &wg)
	resolveFn(serviceC, &wg)
	wg.Wait()

	encryption, err := recordingEncryptionService.GetRecordingEncryption(ctx)
	require.NoError(t, err)

	activeKeys := encryption.GetSpec().ActiveKeys
	// each service should have an active wrapped key
	require.Len(t, activeKeys, 3)
	var fulfilledKeys int
	for _, key := range activeKeys {
		// all wrapped keys should have KeyEncryptionPairs
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotEmpty(t, key.KeyEncryptionPair.PublicKey)
		require.NotEmpty(t, key.KeyEncryptionPair.PrivateKey)

		if key.RecordingEncryptionPair != nil {
			fulfilledKeys += 1
		}
	}

	// only the first service to run should have a fulfilled wrapped key
	require.Equal(t, 1, fulfilledKeys)
}

func TestUnwrapKey(t *testing.T) {
	// SETUP
	ctx, bk := newLocalBackend(t)
	keyType := types.PrivateKeyType_RAW

	config := newManagerConfig(t, bk, keyType)
	manager, err := recordingencryption.NewManager(config)
	require.NoError(t, err)

	service := config.Backend
	_, _, err = resolve(ctx, service, manager)
	require.NoError(t, err)

	src, err := manager.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)

	encryptionKeys := src.GetEncryptionKeys()
	require.Len(t, encryptionKeys, 1)
	pubKeyPEM := encryptionKeys[0].PublicKey

	pubKey, err := keys.ParsePublicKey(pubKeyPEM)
	require.NoError(t, err)

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	require.True(t, ok)

	fileKey := []byte("test_file_key")
	label := []byte("test_label")
	wrappedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPubKey, fileKey, label)
	require.NoError(t, err)

	fp, err := recordingencryption.Fingerprint(pubKey)
	require.NoError(t, err)

	unwrapInput := recordingencryption.UnwrapInput{
		Fingerprint: fp,
		WrappedKey:  wrappedKey,
		Rand:        rand.Reader,
		Opts: &rsa.OAEPOptions{
			Hash:  crypto.SHA256,
			Label: label,
		},
	}
	unwrappedKey, err := manager.UnwrapKey(ctx, unwrapInput)
	require.NoError(t, err)

	require.Equal(t, fileKey, unwrappedKey)
}
