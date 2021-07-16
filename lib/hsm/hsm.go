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

package hsm

import (
	"bytes"
	"crypto"
	"encoding/json"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/ThalesIgnite/crypto11"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

var prefix = []byte("pkcs11:")
var label = []byte("teleport")

// RSAKeyPairSource is a function type which returns new RSA keypairs. For use
// when there is no real HSM.
type RSAKeyPairSource func(string) (priv []byte, pub []byte, err error)

// ClientConfig is used to pass HSM client configuration parameters.
type ClientConfig struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber the the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string

	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string

	// RSAKeyPairSource is a function type which returns new RSA keypairs. For
	// use when there is no real HSM.
	RSAKeyPairSource RSAKeyPairSource
}

func (config *ClientConfig) Validate() error {
	if (config.Path == "") == (config.RSAKeyPairSource == nil) {
		return trace.BadParameter("exactly one of Path or RSAKeyPairSource must be provided")
	}
	return nil
}

// Client is an interface for performing HSM actions.
type Client interface {
	// GenerateRSA creates a new RSA private key and returns its identifier and
	// a crypto.Signer. The returned identifier can be passed to GetSigner
	// later to get the same crypto.Signer.
	GenerateRSA() (keyID []byte, signer crypto.Signer, err error)

	// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
	GetSigner(keyID []byte) (crypto.Signer, error)

	// GetTLSCertAndSigner selects the local TLS keypair and returns the raw TLS cert and crypto.Signer.
	GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error)

	// GetSSHSigner selects the local SSH keypair and returns an ssh.Signer.
	GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error)

	// GetSSHSigner selects the local JWT keypair and returns a *jwt.Key.
	GetJWTSigner(ca types.CertAuthority, clock clockwork.Clock) (*jwt.Key, error)

	// DeleteKey deletes the given key from the HSM
	DeleteKey(keyID []byte) error
}

// NewClient returns a Client based on the passed ClientConfig. If there is no
// attached HSM, RSAKeyPairSource should be provided and raw in-memory keys
// will be used.
func NewClient(config *ClientConfig) (Client, error) {
	if err := config.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Path != "" {
		client, err := newPKCS11Client(config)
		return client, trace.Wrap(err)
	}
	return newRawClient(config), nil
}

type pkcs11Client struct {
	ctx      *crypto11.Context
	hostUUID string
}

func newPKCS11Client(config *ClientConfig) (Client, error) {
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

	return &pkcs11Client{
		ctx:      ctx,
		hostUUID: config.HostUUID,
	}, nil
}

// GenerateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier can be passed to GetSigner later to
// get the same crypto.Signer.
func (c *pkcs11Client) GenerateRSA() ([]byte, crypto.Signer, error) {
	var id uuid.UUID
	var signer crypto.Signer
	var err error

	// Some HSMs (like YubiHSM2) will truncate the passed ID to as few as 2
	// bytes. There's not a great way to detect this and I don't want to limit
	// the ID to 2 bytes on all systems, so for now we will generate a few
	// random IDs and hope to avoid a collision. Ideally Teleport should be the
	// only thing creating keys for this token and there should only be 10 keys
	// per HSM at a given time:
	// 2(rotation phases) * (4(SSH and TLS for User and Host CA) + 1(JWT CA))
	for iterations := 0; iterations < 32 && (err != nil || signer == nil); iterations++ {
		id, err = uuid.NewRandom()
		if err != nil {
			return nil, nil, err
		}
		signer, err = c.ctx.GenerateRSAKeyPairWithLabel(id[:], label, teleport.RSAKeySize)
	}
	if signer == nil {
		return nil, nil, trace.Wrap(err, "failed to create RSA key in hsm, resources may be exhausted")
	}

	key := keyID{
		HostID: c.hostUUID,
		KeyID:  id.String(),
	}

	keyID, err := key.marshal()
	if err != nil {
		return nil, nil, err
	}
	return keyID, signer, nil
}

// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
func (c *pkcs11Client) GetSigner(rawKey []byte) (crypto.Signer, error) {
	keyType := KeyType(rawKey)
	switch keyType {
	case types.PrivateKeyType_PKCS11:
		keyID, err := parseKeyID(rawKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if keyID.HostID != c.hostUUID {
			return nil, trace.NotFound("pkcs11 key is for different host")
		}
		pkcs11ID, err := keyID.pkcs11Key()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signer, err := c.ctx.FindKeyPair(pkcs11ID, label)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if signer == nil {
			return nil, trace.NotFound("failed to find keypair for given id")
		}
		return signer, nil
	case types.PrivateKeyType_RAW:
		signer, err := utils.ParsePrivateKeyPEM(rawKey)
		return signer, trace.Wrap(err)
	}
	return nil, trace.BadParameter("unrecognized key type %s", keyType.String())
}

func (c *pkcs11Client) selectTLSKeyPair(ca types.CertAuthority) (*types.TLSKeyPair, error) {
	keyPairs := ca.GetActiveKeys().TLS
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no TLS key pairs found in CA for %q", ca.GetClusterName())
	}
	// prefer hsm key
	for _, keyPair := range keyPairs {
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
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no TLS key pairs found in CA for %q", ca.GetClusterName())
}

// GetTLSCertAndSigner selects the local TLS keypair and returns the raw TLS cert and crypto.Signer.
func (c *pkcs11Client) GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPair, err := c.selectTLSKeyPair(ca)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// if there is no key, this CA may only be used for checking
	if len(keyPair.Key) == 0 {
		return keyPair.Cert, nil, nil
	}

	signer, err := c.GetSigner(keyPair.Key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyPair.Cert, signer, nil
}

func (c *pkcs11Client) selectSSHKeyPair(ca types.CertAuthority) (*types.SSHKeyPair, error) {
	keyPairs := ca.GetActiveKeys().SSH
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no SSH key pairs found in CA for %q", ca.GetClusterName())
	}
	// prefer hsm key
	for _, keyPair := range keyPairs {
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
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no SSH key pairs found in CA for %q", ca.GetClusterName())
}

// GetSSHSigner selects the local SSH keypair and returns an ssh.Signer.
func (c *pkcs11Client) GetSSHSigner(ca types.CertAuthority) (sshSigner ssh.Signer, err error) {
	keyPair, err := c.selectSSHKeyPair(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch keyPair.PrivateKeyType {
	case types.PrivateKeyType_RAW:
		sshSigner, err = ssh.ParsePrivateKey(keyPair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.PrivateKeyType_PKCS11:
		signer, err := c.GetSigner(keyPair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sshSigner, err = ssh.NewSignerFromSigner(signer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unrecognized key type %q", keyPair.PrivateKeyType.String())
	}

	sshSigner = sshutils.AlgSigner(sshSigner, sshutils.GetSigningAlgName(ca))
	return sshSigner, nil
}

func (c *pkcs11Client) selectJWTSigner(ca types.CertAuthority) (crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().JWT
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no JWT keypairs found")
	}
	// prefer hsm key if there is one
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
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no JWT key pairs found in CA for %q", ca.GetClusterName())
}

// GetJWTSigner returns the active JWT key used to sign tokens.
func (c *pkcs11Client) GetJWTSigner(ca types.CertAuthority, clock clockwork.Clock) (*jwt.Key, error) {
	signer, err := c.selectJWTSigner(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := jwt.New(&jwt.Config{
		Clock:       clock,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: ca.GetClusterName(),
		PrivateKey:  signer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// DeleteKey deletes the given key from the HSM
func (c *pkcs11Client) DeleteKey(rawKey []byte) error {
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
	signer, err := c.ctx.FindKeyPair(pkcs11ID, label)
	if err != nil {
		return trace.Wrap(err)
	}
	if signer == nil {
		return trace.NotFound("failed to find keypair for given id")
	}
	return trace.Wrap(signer.Delete())
}

type rawClient struct {
	rsaKeyPairSource RSAKeyPairSource
}

func newRawClient(config *ClientConfig) Client {
	return &rawClient{
		rsaKeyPairSource: config.RSAKeyPairSource,
	}
}

// GenerateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier for rawClient is a pem-encoded
// private key, and can be passed to GetSigner later to get the same
// crypto.Signer.
func (c *rawClient) GenerateRSA() ([]byte, crypto.Signer, error) {
	priv, _, err := c.rsaKeyPairSource("")
	if err != nil {
		return nil, nil, err
	}
	signer, err := c.GetSigner(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, signer, trace.Wrap(err)
}

// GetSigner returns a crypto.Signer for the given pem-encoded private key.
func (c *rawClient) GetSigner(rawKey []byte) (crypto.Signer, error) {
	signer, err := utils.ParsePrivateKeyPEM(rawKey)
	return signer, trace.Wrap(err)
}

// GetTLSCertAndSigner selects the first raw TLS keypair and returns the raw
// TLS cert and a crypto.Signer.
func (c *rawClient) GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().TLS
	if len(keyPairs) == 0 {
		return nil, nil, trace.NotFound("no TLS key pairs found in CA for %q", ca.GetClusterName())
	}

	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			// private key may be nil, the cert will only be used for checking
			if len(keyPair.Key) == 0 {
				return keyPair.Cert, nil, nil
			}
			signer, err := utils.ParsePrivateKeyPEM(keyPair.Key)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return keyPair.Cert, signer, nil
		}
	}
	return nil, nil, trace.NotFound("no matching TLS key pairs found in CA for %q", ca.GetClusterName())
}

// GetSSHSigner selects the first raw SSH keypair and returns an ssh.Signer
func (c *rawClient) GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error) {
	keyPairs := ca.GetActiveKeys().SSH
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no SSH key pairs found in CA for %q", ca.GetClusterName())
	}

	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := ssh.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			signer = sshutils.AlgSigner(signer, sshutils.GetSigningAlgName(ca))
			return signer, nil
		}
	}
	return nil, trace.NotFound("no raw SSH key pairs found in CA for %q", ca.GetClusterName())
}

func (c *rawClient) selectJWTSigner(ca types.CertAuthority) (crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().JWT
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no JWT keypairs found")
	}
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no JWT key pairs found in CA for %q", ca.GetClusterName())
}

// GetJWTSigner returns the active JWT key used to sign tokens.
func (c *rawClient) GetJWTSigner(ca types.CertAuthority, clock clockwork.Clock) (*jwt.Key, error) {
	signer, err := c.selectJWTSigner(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := jwt.New(&jwt.Config{
		Clock:       clock,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: ca.GetClusterName(),
		PrivateKey:  signer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// DeleteKey deletes the given key from the HSM. This is a no-op for rawClient.
func (c *rawClient) DeleteKey(rawKey []byte) error {
	return nil
}

// KeyType returns the type of the given private key.
func KeyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, prefix) {
		return types.PrivateKeyType_PKCS11
	}
	return types.PrivateKeyType_RAW
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
	buf = append(append([]byte{}, prefix...), buf...)
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
	key = key[len(prefix):]
	if err := json.Unmarshal(key, &keyID); err != nil {
		//return keyID, trace.BadParameter("unable to parse invalid pkcs11 key")
		return keyID, trace.Wrap(err)
	}
	return keyID, nil
}
