/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keystore

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/json"

	"github.com/ThalesIgnite/crypto11"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

var pkcs11Prefix = []byte("pkcs11:")

// PKCS11Config is used to pass PKCS11 HSM client configuration parameters.
type PKCS11Config struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber is the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string

	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string
}

func (cfg *PKCS11Config) CheckAndSetDefaults() error {
	if cfg.SlotNumber == nil && cfg.TokenLabel == "" {
		return trace.BadParameter("must provide one of SlotNumber or TokenLabel")
	}
	if cfg.HostUUID == "" {
		return trace.BadParameter("must provide HostUUID")
	}
	return nil
}

type pkcs11KeyStore struct {
	ctx      *crypto11.Context
	hostUUID string
	log      logrus.FieldLogger
}

func newPKCS11KeyStore(config *PKCS11Config, logger logrus.FieldLogger) (*pkcs11KeyStore, error) {
	cryptoConfig := &crypto11.Config{
		Path:       config.Path,
		TokenLabel: config.TokenLabel,
		SlotNumber: config.SlotNumber,
		Pin:        config.Pin,
	}
	ctx, err := crypto11.Configure(cryptoConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger = logger.WithFields(logrus.Fields{trace.Component: "PKCS11KeyStore"})

	return &pkcs11KeyStore{
		ctx:      ctx,
		hostUUID: config.HostUUID,
		log:      logger,
	}, nil
}

func (p *pkcs11KeyStore) findUnusedID() (uuid.UUID, error) {
	var id uuid.UUID
	var err error

	// Some HSMs (like YubiHSM2) will silently truncate the passed ID to as few
	// as 2 bytes. There's not a great way to detect this and I don't want to
	// limit the ID to 2 bytes on all systems, so for now we will generate a
	// few random IDs and hope to avoid a collision. Ideally Teleport should be
	// the only thing creating keys for this token and there should only be 10
	// keys per HSM at a given time:
	// 2(rotation phases) * (4(SSH and TLS for User and Host CA) + 1(JWT CA))
	maxIterations := 16
	iterations := 0
	for ; iterations < maxIterations; iterations++ {
		id, err = uuid.NewRandom()
		if err != nil {
			return id, trace.Wrap(err)
		}
		existingSigner, err := p.ctx.FindKeyPair(id[:], []byte(p.hostUUID))
		if err != nil {
			return id, trace.Wrap(err)
		}
		if existingSigner == nil {
			// failed to find an existing keypair, so this ID is unique
			break
		} else {
			p.log.Warn("Found CKA_ID collision while creating keypair, retrying with new ID")
		}
	}
	if iterations == maxIterations {
		return id, trace.AlreadyExists("failed to find unused CKA_ID for HSM")
	}
	return id, nil
}

// generateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier can be passed to getSigner later to
// get the same crypto.Signer.
func (p *pkcs11KeyStore) generateRSA(ctx context.Context, options ...RSAKeyOption) ([]byte, crypto.Signer, error) {
	p.log.Debug("Creating new HSM keypair")
	id, err := p.findUnusedID()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := p.ctx.GenerateRSAKeyPairWithLabel(id[:], []byte(p.hostUUID), constants.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	key := keyID{
		HostID: p.hostUUID,
		KeyID:  id.String(),
	}

	keyID, err := key.marshal()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID, signer, nil
}

// getSigner returns a crypto.Signer for the given key identifier, if it is found.
func (p *pkcs11KeyStore) getSigner(ctx context.Context, rawKey []byte) (crypto.Signer, error) {
	if t := keyType(rawKey); t != types.PrivateKeyType_PKCS11 {
		return nil, trace.BadParameter("pkcs11KeyStore cannot get signer for key type %s", t.String())
	}
	keyID, err := parseKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if keyID.HostID != p.hostUUID {
		return nil, trace.NotFound("given pkcs11 key is for host: %q, but this host is: %q", keyID.HostID, p.hostUUID)
	}
	pkcs11ID, err := keyID.pkcs11Key()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := p.ctx.FindKeyPair(pkcs11ID, []byte(p.hostUUID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer == nil {
		return nil, trace.NotFound("failed to find keypair for given id")
	}
	return signer, nil
}

// canSignWithKey returns true if the given key is PKCS11 and was created by
// this host. If the HSM is disconnected or the key material has been deleted
// the error will not be detected here but when the first signature is
// attempted.
func (p *pkcs11KeyStore) canSignWithKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error) {
	if keyType != types.PrivateKeyType_PKCS11 {
		return false, nil
	}
	keyID, err := parseKeyID(raw)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return keyID.HostID == p.hostUUID, nil
}

// deleteKey deletes the given key from the HSM
func (p *pkcs11KeyStore) deleteKey(_ context.Context, rawKey []byte) error {
	keyID, err := parseKeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if keyID.HostID != p.hostUUID {
		return trace.NotFound("pkcs11 key is for different host")
	}
	pkcs11ID, err := keyID.pkcs11Key()
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := p.ctx.FindKeyPair(pkcs11ID, []byte(p.hostUUID))
	if err != nil {
		return trace.Wrap(err)
	}
	if signer == nil {
		return trace.NotFound("failed to find keypair for given id")
	}
	return trace.Wrap(signer.Delete())
}

// DeleteUnusedKeys deletes all keys from the KeyStore if they are:
// 1. Labeled by this KeyStore when they were created
// 2. Not included in the argument usedKeys
func (p *pkcs11KeyStore) DeleteUnusedKeys(ctx context.Context, usedKeys [][]byte) error {
	p.log.Debug("Deleting unused keys from HSM")
	var usedPublicKeys []*rsa.PublicKey
	for _, usedKey := range usedKeys {
		if keyType(usedKey) != types.PrivateKeyType_PKCS11 {
			continue
		}
		signer, err := p.getSigner(ctx, usedKey)
		if trace.IsNotFound(err) {
			// key is for different host, or truly not found in HSM. Either
			// way, it won't be deleted below.
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}
		rsaPublicKey, ok := signer.Public().(*rsa.PublicKey)
		if !ok {
			return trace.BadParameter("unknown public key type: %T", signer.Public())
		}
		usedPublicKeys = append(usedPublicKeys, rsaPublicKey)
	}
	keyIsUsed := func(signer crypto.Signer) bool {
		rsaPublicKey, ok := signer.Public().(*rsa.PublicKey)
		if !ok {
			// unknown key type... we don't know what this is, so don't delete it
			return true
		}
		for _, k := range usedPublicKeys {
			if rsaPublicKey.Equal(k) {
				return true
			}
		}
		return false
	}
	signers, err := p.ctx.FindKeyPairs(nil, []byte(p.hostUUID))
	if err != nil {
		return trace.Wrap(err)
	}
	for _, signer := range signers {
		if keyIsUsed(signer) {
			continue
		}
		if err := signer.Delete(); err != nil {
			// Key deletion is best-effort, log a warning on errors. Errors have
			// been observed when FindKeyPairs returns duplicate keys.
			p.log.Warnf("failed deleting unused key from HSM: %v", err)
		}
	}
	return nil
}

type keyID struct {
	HostID string `json:"host_id"`
	KeyID  string `json:"key_id"`
}

func (k keyID) marshal() ([]byte, error) {
	buf, err := json.Marshal(k)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf = append(append([]byte{}, pkcs11Prefix...), buf...)
	return buf, nil
}

func (k keyID) pkcs11Key() ([]byte, error) {
	id, err := uuid.Parse(k.KeyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return id[:], nil
}

func parseKeyID(key []byte) (keyID, error) {
	var keyID keyID
	if keyType(key) != types.PrivateKeyType_PKCS11 {
		return keyID, trace.BadParameter("unable to parse invalid pkcs11 key")
	}
	// strip pkcs11: prefix
	key = key[len(pkcs11Prefix):]
	if err := json.Unmarshal(key, &keyID); err != nil {
		return keyID, trace.Wrap(err)
	}
	return keyID, nil
}
