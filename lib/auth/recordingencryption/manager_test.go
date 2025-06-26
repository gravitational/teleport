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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"filippo.io/age"
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
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type oaepDecrypter struct {
	crypto.Decrypter
	hash crypto.Hash
}

func (d oaepDecrypter) Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	return d.Decrypter.Decrypt(rand, msg, &rsa.OAEPOptions{
		Hash: d.hash,
	})
}

type fakeKeyStore struct {
	keyType types.PrivateKeyType // abusing this field as a way to simulate different auth servers
}

func (f *fakeKeyStore) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error) {
	decrypter, err := cryptosuites.GenerateDecrypterWithAlgorithm(cryptosuites.RSA2048)
	if err != nil {
		return nil, err
	}

	private, ok := decrypter.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("expected RSA private key")
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  keys.PKCS1PrivateKeyType,
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	})

	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  keys.PKCS1PublicKeyType,
		Bytes: x509.MarshalPKCS1PublicKey(&private.PublicKey),
	})

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

	block, _ := pem.Decode(keyPair.PrivateKey)

	private, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return oaepDecrypter{Decrypter: private, hash: crypto.Hash(keyPair.Hash)}, nil
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
		Logger:        logtest.NewLogger(),
		LockConfig: backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            bk,
				LockNameComponents: []string{"recording_encryption"},
				TTL:                5 * time.Second,
				RetryInterval:      10 * time.Millisecond,
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
	require.NotNil(t, activeKeys[0].KeyEncryptionPair)
	require.NotEmpty(t, activeKeys[0].KeyEncryptionPair.PrivateKey)
	require.NotEmpty(t, activeKeys[0].KeyEncryptionPair.PublicKey)

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

	managerAType := types.PrivateKeyType_RAW
	managerBType := types.PrivateKeyType_AWS_KMS

	configA := newManagerConfig(t, bk, managerAType)
	configB := configA
	configB.KeyStore = &fakeKeyStore{managerBType}

	managerA, err := recordingencryption.NewManager(configA)
	require.NoError(t, err)

	managerB, err := recordingencryption.NewManager(configB)
	require.NoError(t, err)

	service := configA.Backend

	// TEST
	// CASE: service A first evaluation initializes recording encryption resource
	encryption, src, err := resolve(ctx, service, managerA)
	require.NoError(t, err)
	activeKeys := encryption.GetSpec().GetActiveKeys()

	require.Len(t, activeKeys, 1)
	require.Len(t, src.GetEncryptionKeys(), 1)
	firstKey := activeKeys[0]

	// should generate a wrapped key with the initial recording encryption pair
	require.NotNil(t, firstKey.KeyEncryptionPair)
	require.NotNil(t, firstKey.RecordingEncryptionPair)

	// CASE: service B should generate an unfulfilled key since there's an existing recording encryption resource
	encryption, src, err = resolve(ctx, service, managerB)
	require.NoError(t, err)

	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	require.Len(t, src.GetEncryptionKeys(), 1)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == managerAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// service B re-evaluting with an unfulfilled key should do nothing
	encryption, src, err = resolve(ctx, service, managerB)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	require.Len(t, src.GetEncryptionKeys(), 1)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == managerAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// CASE: service A evaluation should fulfill service B's key
	encryption, src, err = resolve(ctx, service, managerA)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	require.Len(t, src.GetEncryptionKeys(), 1)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
	}
}

func TestResolveRecordingEncryptionConcurrent(t *testing.T) {
	// SETUP
	ctx, bk := newLocalBackend(t)

	managerAType := types.PrivateKeyType_RAW
	managerBType := types.PrivateKeyType_AWS_KMS
	serviceCType := types.PrivateKeyType_GCP_KMS

	configA := newManagerConfig(t, bk, managerAType)
	configB := configA
	configB.KeyStore = &fakeKeyStore{managerBType}
	configC := configA
	configC.KeyStore = &fakeKeyStore{serviceCType}
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

func TestFindDecryptionKeyFromActiveKeys(t *testing.T) {
	// SETUP
	ctx, bk := newLocalBackend(t)
	keyTypeA := types.PrivateKeyType_RAW
	keyTypeB := types.PrivateKeyType_AWS_KMS

	configA := newManagerConfig(t, bk, keyTypeA)
	configB := configA
	configB.KeyStore = &fakeKeyStore{keyTypeB}
	managerA, err := recordingencryption.NewManager(configA)
	require.NoError(t, err)

	managerB, err := recordingencryption.NewManager(configB)
	require.NoError(t, err)

	service := configA.Backend
	_, _, err = resolve(ctx, service, managerA)
	require.NoError(t, err)

	encryption, _, err := resolve(ctx, service, managerB)
	require.NoError(t, err)

	activeKeys := encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	pubKey := activeKeys[0].RecordingEncryptionPair.PublicKey

	// fail to find private key for manager B because it is waiting for key fulfillment
	_, err = managerB.FindDecryptionKey(ctx, pubKey)
	require.Error(t, err)

	_, _, err = resolve(ctx, service, managerA)
	require.NoError(t, err)

	// find private key for manager A because it provisioned the key
	decryptionPair, err := managerA.FindDecryptionKey(ctx, pubKey)
	require.NoError(t, err)
	ident, err := age.ParseX25519Identity(string(decryptionPair.PrivateKey))
	require.NoError(t, err)
	require.Equal(t, ident.Recipient().String(), string(pubKey))

	// find private key for manager B after fulfillment
	decryptionPair, err = managerB.FindDecryptionKey(ctx, pubKey)
	require.NoError(t, err)
	ident, err = age.ParseX25519Identity(string(decryptionPair.PrivateKey))
	require.NoError(t, err)
	require.Equal(t, ident.Recipient().String(), string(pubKey))
}
