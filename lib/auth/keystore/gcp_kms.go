/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package keystore

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/api/iterator"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore/internal/faketime"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	// GCP does not allow "." or "/" in labels
	hostLabel                      = "teleport_auth_host"
	gcpkmsPrefix                   = "gcpkms:"
	gcpOAEPHash                    = crypto.SHA256
	defaultGCPRequestTimeout       = 30 * time.Second
	defaultGCPPendingTimeout       = 2 * time.Minute
	defaultGCPPendingRetryInterval = 5 * time.Second
	// We always use version 1, during rotation brand new keys are created.
	keyVersionSuffix = "/cryptoKeyVersions/1"
)

type pendingRetryTag struct{}

var (
	gcpKMSProtectionLevels = map[string]kmspb.ProtectionLevel{
		servicecfg.GCPKMSProtectionLevelHSM:      kmspb.ProtectionLevel_HSM,
		servicecfg.GCPKMSProtectionLevelSoftware: kmspb.ProtectionLevel_SOFTWARE,
	}
)

type gcpKMSKeyStore struct {
	hostUUID        string
	keyRing         string
	protectionLevel kmspb.ProtectionLevel
	kmsClient       *kms.KeyManagementClient
	log             *slog.Logger
	clock           faketime.Clock
	waiting         chan struct{}
}

// newGCPKMSKeyStore returns a new keystore configured to use a GCP KMS keyring
// to manage all key material.
func newGCPKMSKeyStore(ctx context.Context, cfg *servicecfg.GCPKMSConfig, opts *Options) (*gcpKMSKeyStore, error) {
	kmsClient := opts.kmsClient
	if kmsClient == nil {
		var err error
		kmsClient, err = kms.NewKeyManagementClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clock := opts.faketimeOverride
	if clock == nil {
		clock = faketime.NewRealClock()
	}

	return &gcpKMSKeyStore{
		hostUUID:        opts.HostUUID,
		keyRing:         cfg.KeyRing,
		protectionLevel: gcpKMSProtectionLevels[cfg.ProtectionLevel],
		kmsClient:       kmsClient,
		log:             opts.Logger,
		clock:           clock,
		waiting:         make(chan struct{}),
	}, nil
}

func (a *gcpKMSKeyStore) name() string {
	return storeGCP
}

// keyTypeDescription returns a human-readable description of the types of keys
// this backend uses.
func (g *gcpKMSKeyStore) keyTypeDescription() string {
	return fmt.Sprintf("GCP KMS keys in keyring %s", g.keyRing)
}

func (g *gcpKMSKeyStore) generateKey(ctx context.Context, algorithm cryptosuites.Algorithm, usage keyUsage) (gcpKMSKeyID, error) {
	alg, err := gcpAlgorithm(algorithm)
	if err != nil {
		return gcpKMSKeyID{}, trace.Wrap(err)
	}

	keyUUID := uuid.NewString()
	g.log.InfoContext(ctx, "Creating new GCP KMS keypair.", "id", keyUUID, "algorithm", alg.String())

	req := &kmspb.CreateCryptoKeyRequest{
		Parent:      g.keyRing,
		CryptoKeyId: keyUUID,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: usage.toGCP(),
			Labels: map[string]string{
				hostLabel: g.hostUUID,
			},
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				ProtectionLevel: g.protectionLevel,
				Algorithm:       alg,
			},
		},
	}
	resp, err := doGCPRequest(ctx, g, g.kmsClient.CreateCryptoKey, req)
	if err != nil {
		return gcpKMSKeyID{}, trace.Wrap(err, "error while attempting to generate new GCP KMS key")
	}

	return gcpKMSKeyID{
		keyVersionName: resp.Name + keyVersionSuffix,
	}, nil
}

// generateSigner creates a new private key and returns its identifier and a crypto.Signer. The returned
// identifier for gcpKMSKeyStore encodes the full GCP KMS key version name, and can be passed to getSigner
// later to get an equivalent crypto.Signer.
func (g *gcpKMSKeyStore) generateSigner(ctx context.Context, algorithm cryptosuites.Algorithm) ([]byte, crypto.Signer, error) {
	keyID, err := g.generateKey(ctx, algorithm, keyUsageSign)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := g.newKmsKey(ctx, keyID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID.marshal(), signer, nil
}

// generateDecrypter creates a new private key and returns its identifier and a crypto.Decrypter. The returned
// identifier for gcpKMSKeyStore encodes the full GCP KMS key version name, and can be passed to getDecrypter
// later to get an equivalent crypto.Decrypter.
func (g *gcpKMSKeyStore) generateDecrypter(ctx context.Context, algorithm cryptosuites.Algorithm) ([]byte, crypto.Decrypter, crypto.Hash, error) {
	keyID, err := g.generateKey(ctx, algorithm, keyUsageDecrypt)
	if err != nil {
		return nil, nil, gcpOAEPHash, trace.Wrap(err)
	}

	decrypter, err := g.newKmsKey(ctx, keyID)
	if err != nil {
		return nil, nil, gcpOAEPHash, trace.Wrap(err)
	}
	return keyID.marshal(), decrypter, gcpOAEPHash, nil
}

func gcpAlgorithm(alg cryptosuites.Algorithm) (kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm, error) {
	switch alg {
	case cryptosuites.RSA2048:
		return kmspb.CryptoKeyVersion_RSA_SIGN_PKCS1_2048_SHA256, nil
	case cryptosuites.ECDSAP256:
		return kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256, nil
	}
	return kmspb.CryptoKeyVersion_CRYPTO_KEY_VERSION_ALGORITHM_UNSPECIFIED, trace.BadParameter("unsupported algorithm: %v", alg)
}

// getSigner returns a crypto.Signer for the given raw private key.
func (g *gcpKMSKeyStore) getSigner(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey) (crypto.Signer, error) {
	keyID, err := parseGCPKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := g.newKmsKeyWithPublicKey(ctx, keyID, publicKey)
	return signer, trace.Wrap(err)
}

// getDecrypter returns a crypto.Decrypter for the given raw private key.
func (g *gcpKMSKeyStore) getDecrypter(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey, hash crypto.Hash) (crypto.Decrypter, error) {
	keyID, err := parseGCPKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := g.newKmsKeyWithPublicKey(ctx, keyID, publicKey)
	return signer, trace.Wrap(err)
}

// deleteKey deletes the given key from the KeyStore.
func (g *gcpKMSKeyStore) deleteKey(ctx context.Context, rawKey []byte) error {
	keyID, err := parseGCPKMSKeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	req := &kmspb.DestroyCryptoKeyVersionRequest{
		Name: keyID.keyVersionName,
	}
	// Retry to destroy while the key is still pending creation.
	_, err = retryWhilePending(ctx, g, g.kmsClient.DestroyCryptoKeyVersion, req)
	return trace.Wrap(err, "error while attempting to delete GCP KMS key")
}

// canUseKey returns true if given a GCP_KMS key in the same key ring managed
// by this keystore. This means that it's possible (and expected) for
// multiple auth servers in a cluster to sign with the same KMS keys if they are
// configured with the same keyring. This is a divergence from the PKCS#11
// keystore where different auth servers will always create their own keys even
// if configured to use the same HSM
func (g *gcpKMSKeyStore) canUseKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error) {
	if keyType != types.PrivateKeyType_GCP_KMS {
		return false, nil
	}
	keyID, err := parseGCPKMSKeyID(raw)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !strings.HasPrefix(keyID.keyVersionName, g.keyRing) {
		return false, nil
	}

	return true, nil
}

// deleteUnusedKeys deletes all keys from the configured KMS keyring if they:
//  1. Are not included in the argument activeKeys
//  2. Are labeled with hostLabel (teleport_auth_host)
//  3. The hostLabel value matches the local host UUID
//
// The activeKeys argument is meant to contain to complete set of raw key IDs as
// stored in the current CA specs in the backend.
//
// The reason this does not delete any keys created by a different auth server
// is to avoid a case where:
// 1. A different auth server (auth2) creates a new key in GCP KMS
// 2. This function (running on auth1) deletes that new key
// 3. auth2 saves the id of this deleted key to the backend CA
//
// or a simpler case where: the other auth server is running in a completely
// different Teleport cluster and the keys it's actively using will never appear
// in the activeKeys argument.
func (g *gcpKMSKeyStore) deleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	// Make a map of currently active key versions, this is used for lookups to
	// check which keys in KMS are unused.
	activeKmsKeyVersions := make(map[string]int)
	for _, activeKey := range activeKeys {
		keyIsRelevant, err := g.canUseKey(ctx, activeKey, keyType(activeKey))
		if err != nil {
			// Don't expect this error to ever hit, safer to return if it does.
			return trace.Wrap(err)
		}
		if !keyIsRelevant {
			// Ignore active keys that are not GCP KMS keys or are in a
			// different keyring than the one this Auth is configured to use.
			continue
		}
		keyID, err := parseGCPKMSKeyID(activeKey)
		if err != nil {
			// Realistically we should not hit this since canUseKey already
			// calls parseGCPKMSKeyID.
			return trace.Wrap(err)
		}
		activeKmsKeyVersions[keyID.keyVersionName] = 0
	}

	var keysToDelete []gcpKMSKeyID

	listKeyRequest := &kmspb.ListCryptoKeysRequest{
		Parent: g.keyRing,
		// Only bother listing keys created by Teleport which should have the
		// hostLabel set. A filter of "labels.label:*" tests if the label is
		// defined.
		// https://cloud.google.com/sdk/gcloud/reference/topic/filters
		// > Use key:* to test if key is defined
		Filter: fmt.Sprintf("labels.%s:*", hostLabel),
	}
	iter := g.kmsClient.ListCryptoKeys(ctx, listKeyRequest)
	key, err := iter.Next()
	for err == nil {
		keyVersionName := key.Name + keyVersionSuffix
		if _, active := activeKmsKeyVersions[keyVersionName]; active {
			// Record that this current active key was actually found.
			activeKmsKeyVersions[keyVersionName] += 1
		} else if key.Labels[hostLabel] == g.hostUUID {
			// This key is not active (it is not currently stored in any
			// Teleport CA) and it was created by this Auth server, so it should
			// be safe to delete.
			keysToDelete = append(keysToDelete, gcpKMSKeyID{
				keyVersionName: keyVersionName,
			})
		}
		key, err = iter.Next()
	}
	if err != nil && !errors.Is(err, iterator.Done) {
		return trace.Wrap(err, "unexpected error while iterating GCP KMS keys")
	}

	// If any member of activeKeys which is part of the same GCP KMS keyring
	// queried here was not found in the ListCryptoKeys response, something has
	// gone wrong and there's a chance we have a bug or GCP has made a breaking
	// API change. In this case we should abort to avoid the chance of deleting
	// any currently active keys.
	for keyVersion, found := range activeKmsKeyVersions {
		if found == 0 {
			return trace.NotFound(
				"cannot find currently active CA key %q in GCP KMS, aborting attempt to delete unused keys",
				keyVersion)
		}
	}

	for _, unusedKey := range keysToDelete {
		g.log.InfoContext(ctx, "Deleting unused GCP KMS key.", "key_version", unusedKey.keyVersionName)
		err := g.deleteKey(ctx, unusedKey.marshal())
		// Ignore errors where we can't destroy because the state is already
		// DESTROYED or DESTROY_SCHEDULED
		if err != nil && !strings.Contains(err.Error(), "has value DESTROY") {
			return trace.Wrap(err, "error deleting unused GCP KMS key %q", unusedKey.keyVersionName)
		}
	}
	return nil
}

// kmsKey implements the crypto.Signer and crypto.Decrypter interface
type kmsKey struct {
	ctx    context.Context
	g      *gcpKMSKeyStore
	keyID  gcpKMSKeyID
	public crypto.PublicKey
}

func (g *gcpKMSKeyStore) newKmsKey(ctx context.Context, keyID gcpKMSKeyID) (*kmsKey, error) {
	req := &kmspb.GetPublicKeyRequest{
		Name: keyID.keyVersionName,
	}
	// Retry fetching the public key while the key is pending creation.
	resp, err := retryWhilePending(ctx, g, g.kmsClient.GetPublicKey, req)
	if err != nil {
		return nil, trace.Wrap(err, "unexpected error fetching public key")
	}

	block, _ := pem.Decode([]byte(resp.Pem))
	if block == nil {
		return nil, trace.BadParameter("GCP KMS key %s has invalid public key PEM", keyID.keyVersionName)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err, "unexpected error parsing public key PEM")
	}

	return g.newKmsKeyWithPublicKey(ctx, keyID, pub)
}

func (g *gcpKMSKeyStore) newKmsKeyWithPublicKey(ctx context.Context, keyID gcpKMSKeyID, publicKey crypto.PublicKey) (*kmsKey, error) {
	return &kmsKey{
		ctx:    ctx,
		g:      g,
		keyID:  keyID,
		public: publicKey,
	}, nil
}

func (s *kmsKey) Public() crypto.PublicKey {
	return s.public
}

func (s *kmsKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	var (
		requestDigest *kmspb.Digest
		data          []byte
	)
	switch opts.HashFunc() {
	case crypto.SHA256:
		requestDigest = &kmspb.Digest{
			Digest: &kmspb.Digest_Sha256{
				Sha256: digest,
			},
		}
	case crypto.SHA512:
		requestDigest = &kmspb.Digest{
			Digest: &kmspb.Digest_Sha512{
				Sha512: digest,
			},
		}
	case crypto.Hash(0):
		// Ed25519 uses no hash and sends the full raw data.
		data = digest
	default:
		return nil, trace.BadParameter("unsupported hash func for GCP KMS signer: %v", opts.HashFunc())
	}

	resp, err := doGCPRequest(s.ctx, s.g, s.g.kmsClient.AsymmetricSign, &kmspb.AsymmetricSignRequest{
		Name:   s.keyID.keyVersionName,
		Digest: requestDigest,
		Data:   data,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error while attempting GCP KMS signing operation")
	}
	return resp.Signature, nil
}

func (s *kmsKey) Decrypt(rand io.Reader, ciphertext []byte, opts crypto.DecrypterOpts) (plaintext []byte, err error) {
	resp, err := doGCPRequest(s.ctx, s.g, s.g.kmsClient.AsymmetricDecrypt, &kmspb.AsymmetricDecryptRequest{
		Name:       s.keyID.keyVersionName,
		Ciphertext: ciphertext,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error while attempting GCP KMS signing operation")
	}
	return resp.Plaintext, nil
}

func (u keyUsage) toGCP() kmspb.CryptoKey_CryptoKeyPurpose {
	switch u {
	case keyUsageDecrypt:
		return kmspb.CryptoKey_ASYMMETRIC_DECRYPT
	default:
		return kmspb.CryptoKey_ASYMMETRIC_SIGN
	}
}

type gcpKMSKeyID struct {
	keyVersionName string
}

func (g gcpKMSKeyID) marshal() []byte {
	return []byte(gcpkmsPrefix + g.keyVersionName)
}

func (g gcpKMSKeyID) keyring() (string, error) {
	// keyVersionName has this format:
	//   projects/*/locations/*/keyRings/*/cryptoKeys/*/cryptoKeyVersions/1
	// want to extract:
	//   projects/*/locations/*/keyRings/*
	// project name, location, and keyRing name can't contain '/'
	splits := strings.SplitN(g.keyVersionName, "/", 7)
	if len(splits) < 7 {
		return "", trace.BadParameter("GCP KMS keyVersionName has bad format")
	}
	return strings.Join(splits[:6], "/"), nil
}

func parseGCPKMSKeyID(key []byte) (gcpKMSKeyID, error) {
	var keyID gcpKMSKeyID
	if keyType(key) != types.PrivateKeyType_GCP_KMS {
		return keyID, trace.BadParameter("unable to parse invalid GCP KMS key")
	}
	// strip gcpkms: prefix
	keyID.keyVersionName = strings.TrimPrefix(string(key), gcpkmsPrefix)
	return keyID, nil
}

func retryWhilePending[reqType, optType, respType any](
	ctx context.Context,
	g *gcpKMSKeyStore,
	f func(context.Context, reqType, ...optType) (*respType, error),
	req reqType,
) (*respType, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultGCPPendingTimeout)
	defer cancel()

	ticker := g.clock.NewTicker(defaultGCPPendingRetryInterval, pendingRetryTag{})
	defer ticker.Stop()
	for {
		resp, err := doGCPRequest(ctx, g, f, req)
		if err == nil {
			return resp, nil
		}
		if !strings.Contains(err.Error(), "PENDING") {
			return nil, trace.Wrap(err)
		}
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-ticker.C():
		}
	}
}

func doGCPRequest[reqType, optType, respType any](
	ctx context.Context,
	g *gcpKMSKeyStore,
	f func(context.Context, reqType, ...optType) (*respType, error),
	req reqType,
) (*respType, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultGCPRequestTimeout)
	defer cancel()
	resp, err := f(ctx, req)
	return resp, trace.Wrap(err)
}
