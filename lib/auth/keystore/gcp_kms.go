// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keystore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore/internal/faketime"
)

const (
	// GCP does not allow "." or "/" in labels
	hostLabel                      = "teleport_auth_host"
	gcpkmsPrefix                   = "gcpkms:"
	defaultGCPRequestTimeout       = 30 * time.Second
	defaultGCPPendingTimeout       = 2 * time.Minute
	defaultGCPPendingRetryInterval = 5 * time.Second
	// We always use version 1, during rotation brand new keys are created.
	keyVersionSuffix = "/cryptoKeyVersions/1"
)

type pendingRetryTag struct{}

var (
	gcpKMSProtectionLevels = map[string]kmspb.ProtectionLevel{
		"SOFTWARE": kmspb.ProtectionLevel_SOFTWARE,
		"HSM":      kmspb.ProtectionLevel_HSM,
	}
)

// GCPKMS holds configuration parameters specific to GCP KMS keystores.
type GCPKMSConfig struct {
	// KeyRing is the fully qualified name of the GCP KMS keyring.
	KeyRing string
	// ProtectionLevel specifies how cryptographic operations are performed.
	// For more information, see https://cloud.google.com/kms/docs/algorithms#protection_levels
	// Supported options are "HSM" and "SOFTWARE".
	ProtectionLevel string
	// HostUUID is the UUID of the teleport host (auth server) running this
	// keystore. Used to label keys so that they can be queried and deleted per
	// server without races when multiple auth servers are configured with the
	// same KeyRing.
	HostUUID string

	kmsClientOverride *kms.KeyManagementClient
	clockOverride     faketime.Clock
}

func (cfg *GCPKMSConfig) CheckAndSetDefaults() error {
	if cfg.KeyRing == "" {
		return trace.BadParameter("must provide a valid KeyRing to GCPKMSConfig")
	}
	if _, ok := gcpKMSProtectionLevels[cfg.ProtectionLevel]; !ok {
		return trace.BadParameter("unsupported ProtectionLevel %s", cfg.ProtectionLevel)
	}
	if cfg.HostUUID == "" {
		return trace.BadParameter("must provide a valid HostUUID to GCPKMSConfig")
	}
	return nil
}

type gcpKMSKeyStore struct {
	hostUUID        string
	keyRing         string
	protectionLevel kmspb.ProtectionLevel
	kmsClient       *kms.KeyManagementClient
	log             logrus.FieldLogger
	clock           faketime.Clock
	waiting         chan struct{}
}

// newGCPKMSKeyStore returns a new keystore configured to use a GCP KMS keyring
// to manage all key material.
func newGCPKMSKeyStore(ctx context.Context, cfg *GCPKMSConfig, logger logrus.FieldLogger) (*gcpKMSKeyStore, error) {
	var kmsClient *kms.KeyManagementClient
	if cfg.kmsClientOverride != nil {
		kmsClient = cfg.kmsClientOverride
	} else {
		var err error
		kmsClient, err = kms.NewKeyManagementClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var clock faketime.Clock
	if cfg.clockOverride == nil {
		clock = faketime.NewRealClock()
	} else {
		clock = cfg.clockOverride
	}

	logger = logger.WithFields(logrus.Fields{trace.Component: "GCPKMSKeyStore"})

	return &gcpKMSKeyStore{
		hostUUID:        cfg.HostUUID,
		keyRing:         cfg.KeyRing,
		protectionLevel: gcpKMSProtectionLevels[cfg.ProtectionLevel],
		kmsClient:       kmsClient,
		log:             logger,
		clock:           clock,
		waiting:         make(chan struct{}),
	}, nil
}

// generateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier for gcpKMSKeyStore encoded the full
// GCP KMS key version name, and can be passed to getSigner later to get the same
// crypto.Signer.
func (g *gcpKMSKeyStore) generateRSA(ctx context.Context, opts ...RSAKeyOption) ([]byte, crypto.Signer, error) {
	options := &RSAKeyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var alg kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm
	switch options.DigestAlgorithm {
	case crypto.SHA256, 0:
		alg = kmspb.CryptoKeyVersion_RSA_SIGN_PKCS1_2048_SHA256
	case crypto.SHA512:
		alg = kmspb.CryptoKeyVersion_RSA_SIGN_PKCS1_4096_SHA512
	default:
		return nil, nil, trace.BadParameter("unsupported digest algorithm: %v", options.DigestAlgorithm)
	}

	keyUUID := uuid.NewString()

	req := &kmspb.CreateCryptoKeyRequest{
		Parent:      g.keyRing,
		CryptoKeyId: keyUUID,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_SIGN,
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
		return nil, nil, trace.Wrap(err, "error while attempting to generate new GCP KMS key")
	}

	keyID := gcpKMSKeyID{
		keyVersionName: resp.Name + keyVersionSuffix,
	}

	signer, err := g.newKmsSigner(ctx, keyID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID.marshal(), signer, nil
}

// getSigner returns a crypto.Signer for the given pem-encoded private key.
func (g *gcpKMSKeyStore) getSigner(ctx context.Context, rawKey []byte) (crypto.Signer, error) {
	keyID, err := parseGCPKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := g.newKmsSigner(ctx, keyID)
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

// canSignWithKey returns true if given a GCP_KMS key in the same key ring
// managed by this keystore. This means that it's possible (and expected) for
// multiple auth servers in a cluster to sign with the same KMS keys if they are
// configured with the same keyring. This is a divergence from the PKCS#11
// keystore where different auth servers will always create their own keys even
// if configured to use the same HSM
func (g *gcpKMSKeyStore) canSignWithKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error) {
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

// DeleteUnusedKeys deletes all keys from KMS if they are:
// 1. Labeled by this server (matching HostUUID) when they were created
// 2. Not included in the argument activeKeys
func (g *gcpKMSKeyStore) DeleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	// Make a map of currently active key versions, this is used for lookups to
	// check which keys in KMS are unused, and holds a count of how many times
	// they are found in KMS. If any active keys are not found in KMS, we are in
	// a bad state, so key deletion will be aborted.
	activeKmsKeyVersions := make(map[string]int)
	for _, activeKey := range activeKeys {
		keyID, err := parseGCPKMSKeyID(activeKey)
		if err != nil {
			// could be a different type of key
			continue
		}
		activeKmsKeyVersions[keyID.keyVersionName] = 0
	}

	var unusedKeyIDs []gcpKMSKeyID

	listKeyRequest := &kmspb.ListCryptoKeysRequest{
		Parent: g.keyRing,
		Filter: fmt.Sprintf("labels.%s=%s", hostLabel, g.hostUUID),
	}
	iter := g.kmsClient.ListCryptoKeys(ctx, listKeyRequest)
	key, err := iter.Next()
	for err == nil {
		keyVersionName := key.Name + keyVersionSuffix
		if _, active := activeKmsKeyVersions[keyVersionName]; active {
			activeKmsKeyVersions[keyVersionName]++
		} else {
			unusedKeyIDs = append(unusedKeyIDs, gcpKMSKeyID{
				keyVersionName: keyVersionName,
			})
		}
		key, err = iter.Next()
	}
	if err != nil && !errors.Is(err, iterator.Done) {
		return trace.Wrap(err)
	}

	for keyVersion, found := range activeKmsKeyVersions {
		if found == 0 {
			// Failed to find a currently active key owned by this host.
			// The cluster is in a bad state, refuse to delete any keys.
			return trace.NotFound(
				"cannot find currently active CA key in %q GCP KMS, aborting attempt to delete unused keys",
				keyVersion)
		}
	}

	for _, unusedKey := range unusedKeyIDs {
		g.log.WithFields(logrus.Fields{"key_version": unusedKey.keyVersionName}).Info("deleting unused GCP KMS key created by this server")
		err := g.deleteKey(ctx, unusedKey.marshal())
		// Ignore errors where we can't destroy because the state is already
		// DESTROYED or DESTROY_SCHEDULED
		if err != nil && !strings.Contains(err.Error(), "has value DESTROY") {
			g.log.WithFields(logrus.Fields{"key_version": unusedKey.keyVersionName}).WithError(err).Warn("error deleting unused GCP KMS key")
			return trace.Wrap(err)
		}
	}
	return nil
}

// kmsSigner implements the crypto.Signer interface
type kmsSigner struct {
	ctx    context.Context
	g      *gcpKMSKeyStore
	keyID  gcpKMSKeyID
	public crypto.PublicKey
}

func (g *gcpKMSKeyStore) newKmsSigner(ctx context.Context, keyID gcpKMSKeyID) (*kmsSigner, error) {
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
		return nil, trace.Wrap(err, "unexpected error parsing public key pem")
	}

	return &kmsSigner{
		ctx:    ctx,
		g:      g,
		keyID:  keyID,
		public: pub,
	}, nil
}

func (s *kmsSigner) Public() crypto.PublicKey {
	return s.public
}

func (s *kmsSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	requestDigest := &kmspb.Digest{}
	switch opts.HashFunc() {
	case crypto.SHA256:
		requestDigest.Digest = &kmspb.Digest_Sha256{
			Sha256: digest,
		}
	case crypto.SHA512:
		requestDigest.Digest = &kmspb.Digest_Sha512{
			Sha512: digest,
		}
	default:
		return nil, trace.BadParameter("unsupported hash func for GCP KMS signer: %v", opts.HashFunc())
	}

	resp, err := doGCPRequest(s.ctx, s.g, s.g.kmsClient.AsymmetricSign, &kmspb.AsymmetricSignRequest{
		Name:   s.keyID.keyVersionName,
		Digest: requestDigest,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error while attempting GCP KMS signing operation")
	}
	return resp.Signature, nil
}

type gcpKMSKeyID struct {
	keyVersionName string
}

func (g gcpKMSKeyID) marshal() []byte {
	var buf bytes.Buffer
	buf.WriteString(gcpkmsPrefix)
	buf.WriteString(g.keyVersionName)
	return buf.Bytes()
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
