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
	"crypto"
	"crypto/rsa"
	"encoding/json"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/ThalesIgnite/crypto11"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// HSMConfig is used to pass HSM client configuration parameters.
type HSMConfig struct {
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

type hsmKeyStore struct {
	ctx      *crypto11.Context
	hostUUID string
	log      logrus.FieldLogger
}

func NewHSMKeyStore(config *HSMConfig) (KeyStore, error) {
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

	return &hsmKeyStore{
		ctx:      ctx,
		hostUUID: config.HostUUID,
		log:      logrus.WithFields(logrus.Fields{trace.Component: "HSMKeyStore"}),
	}, nil
}

func (c *hsmKeyStore) findUnusedID() (uuid.UUID, error) {
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
		existingSigner, err := c.ctx.FindKeyPair(id[:], []byte(c.hostUUID))
		if err != nil {
			return id, trace.Wrap(err)
		}
		if existingSigner == nil {
			// failed to find an existing keypair, so this ID is unique
			break
		} else {
			c.log.Warn("Found CKA_ID collision while creating keypair, retrying with new ID")
		}
	}
	if iterations == maxIterations {
		return id, trace.AlreadyExists("failed to find unused CKA_ID for HSM")
	}
	return id, nil
}

// GenerateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier can be passed to GetSigner later to
// get the same crypto.Signer.
func (c *hsmKeyStore) GenerateRSA() ([]byte, crypto.Signer, error) {
	c.log.Debug("Creating new HSM keypair")
	id, err := c.findUnusedID()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := c.ctx.GenerateRSAKeyPairWithLabel(id[:], []byte(c.hostUUID), constants.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	key := keyID{
		HostID: c.hostUUID,
		KeyID:  id.String(),
	}

	keyID, err := key.marshal()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID, signer, nil
}

// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
func (c *hsmKeyStore) GetSigner(rawKey []byte) (crypto.Signer, error) {
	keyType := KeyType(rawKey)
	switch keyType {
	case types.PrivateKeyType_PKCS11:
		keyID, err := parseKeyID(rawKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if keyID.HostID != c.hostUUID {
			return nil, trace.NotFound("given pkcs11 key is for host: %q, but this host is: %q", keyID.HostID, c.hostUUID)
		}
		pkcs11ID, err := keyID.pkcs11Key()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signer, err := c.ctx.FindKeyPair(pkcs11ID, []byte(c.hostUUID))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if signer == nil {
			return nil, trace.NotFound("failed to find keypair for given id")
		}
		return signer, nil
	case types.PrivateKeyType_RAW:
		return nil, trace.BadParameter("cannot get raw signer from HSM KeyStore")
	}
	return nil, trace.BadParameter("unrecognized key type %s", keyType.String())
}

func (c *hsmKeyStore) selectTLSKeyPair(keySet types.CAKeySet) (*types.TLSKeyPair, error) {
	for _, keyPair := range keySet.TLS {
		if keyPair.KeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.Key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no local PKCS#11 TLS key pairs found in CA")
}

// GetTLSCertAndSigner selects the local TLS keypair and returns the raw TLS cert and crypto.Signer.
func (c *hsmKeyStore) GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPair, err := c.selectTLSKeyPair(ca.GetActiveKeys())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := c.GetSigner(keyPair.Key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyPair.Cert, signer, nil
}

func (c *hsmKeyStore) selectSSHKeyPair(keySet types.CAKeySet) (*types.SSHKeyPair, error) {
	for _, keyPair := range keySet.SSH {
		if keyPair.PrivateKeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no local PKCS#11 SSH key pairs found in CA")
}

// GetSSHSigner selects the local SSH keypair and returns an ssh.Signer.
func (c *hsmKeyStore) GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error) {
	keyPair, err := c.selectSSHKeyPair(ca.GetActiveKeys())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := c.GetSigner(keyPair.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshSigner, nil
}

// GetJWTSigner returns the active jwt signer used to sign tokens.
func (c *hsmKeyStore) GetJWTSigner(ca types.CertAuthority) (crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().JWT
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			signer, err := c.GetSigner(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no local PKCS#11 JWT key pairs found in %s CA for %q", ca.GetType(), ca.GetClusterName())
}

// NewSSHKeyPair creates and returns a new HSM-backed SSHKeyPair.
func (c *hsmKeyStore) NewSSHKeyPair() (*types.SSHKeyPair, error) {
	return newSSHKeyPair(c)
}

// NewTLSKeyPair creates and returns a new HSM-backed TLSKeyPair.
func (c *hsmKeyStore) NewTLSKeyPair(clusterName string) (*types.TLSKeyPair, error) {
	return newTLSKeyPair(c, clusterName)
}

// NewJWTKeyPair creates and returns a new HSM-backed JWTKeyPair.
func (c *hsmKeyStore) NewJWTKeyPair() (*types.JWTKeyPair, error) {
	return newJWTKeyPair(c)
}

func (c *hsmKeyStore) keySetHasLocalKeys(keySet types.CAKeySet) bool {
	for _, sshKeyPair := range keySet.SSH {
		if sshKeyPair.PrivateKeyType != types.PrivateKeyType_PKCS11 {
			continue
		}
		keyID, err := parseKeyID(sshKeyPair.PrivateKey)
		if err != nil {
			c.log.WithError(err).Warnf("Failed to parse PKCS#11 key ID")
			continue
		}
		if keyID.HostID == c.hostUUID {
			return true
		}
	}
	for _, tlsKeyPair := range keySet.TLS {
		if tlsKeyPair.KeyType != types.PrivateKeyType_PKCS11 {
			continue
		}
		keyID, err := parseKeyID(tlsKeyPair.Key)
		if err != nil {
			c.log.WithError(err).Warnf("Failed to parse PKCS#11 key ID")
			continue
		}
		if keyID.HostID == c.hostUUID {
			return true
		}
	}
	for _, jwtKeyPair := range keySet.JWT {
		if jwtKeyPair.PrivateKeyType != types.PrivateKeyType_PKCS11 {
			continue
		}
		keyID, err := parseKeyID(jwtKeyPair.PrivateKey)
		if err != nil {
			c.log.WithError(err).Warnf("Failed to parse PKCS#11 key ID")
			continue
		}
		if keyID.HostID == c.hostUUID {
			return true
		}
	}
	return false
}

// HasLocalActiveKeys returns true if the given CA has any active keys that
// are usable with this KeyStore.
func (c *hsmKeyStore) HasLocalActiveKeys(ca types.CertAuthority) bool {
	return c.keySetHasLocalKeys(ca.GetActiveKeys())
}

// HasLocalAdditionalKeys returns true if the given CA has any additional
// trusted keys that are usable with this KeyStore.
func (c *hsmKeyStore) HasLocalAdditionalKeys(ca types.CertAuthority) bool {
	return c.keySetHasLocalKeys(ca.GetAdditionalTrustedKeys())
}

// DeleteKey deletes the given key from the HSM
func (c *hsmKeyStore) DeleteKey(rawKey []byte) error {
	keyID, err := parseKeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if keyID.HostID != c.hostUUID {
		return trace.NotFound("pkcs11 key is for different host")
	}
	pkcs11ID, err := keyID.pkcs11Key()
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := c.ctx.FindKeyPair(pkcs11ID, []byte(c.hostUUID))
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
func (c *hsmKeyStore) DeleteUnusedKeys(usedKeys [][]byte) error {
	c.log.Debug("Deleting unused keys from HSM")
	var usedPublicKeys []*rsa.PublicKey
	for _, usedKey := range usedKeys {
		keyType := KeyType(usedKey)
		if keyType != types.PrivateKeyType_PKCS11 {
			continue
		}
		signer, err := c.GetSigner(usedKey)
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
	signers, err := c.ctx.FindKeyPairs(nil, []byte(c.hostUUID))
	if err != nil {
		return trace.Wrap(err)
	}
	for _, signer := range signers {
		if keyIsUsed(signer) {
			continue
		}
		if err := signer.Delete(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetAdditionalTrustedSSHSigner selects the local SSH keypair from the CA
// AdditionalTrustedKeys and returns an ssh.Signer.
func (c *hsmKeyStore) GetAdditionalTrustedSSHSigner(ca types.CertAuthority) (ssh.Signer, error) {
	keyPair, err := c.selectSSHKeyPair(ca.GetAdditionalTrustedKeys())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := c.GetSigner(keyPair.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshSigner, nil
}

// GetAdditionalTrustedTLSCertAndSigner selects the local TLS keypair from the
// CA AdditionalTrustedKeys and returns the PEM-encoded TLS cert and a
// crypto.Signer.
func (c *hsmKeyStore) GetAdditionalTrustedTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPair, err := c.selectTLSKeyPair(ca.GetAdditionalTrustedKeys())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := c.GetSigner(keyPair.Key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyPair.Cert, signer, nil
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
	if KeyType(key) != types.PrivateKeyType_PKCS11 {
		return keyID, trace.BadParameter("unable to parse invalid pkcs11 key")
	}
	// strip pkcs11: prefix
	key = key[len(pkcs11Prefix):]
	if err := json.Unmarshal(key, &keyID); err != nil {
		return keyID, trace.Wrap(err)
	}
	return keyID, nil
}
