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
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ThalesIgnite/crypto11"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/miekg/pkcs11"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

var pkcs11Prefix = []byte("pkcs11:")

type pkcs11KeyStore struct {
	ctx       *crypto11.Context
	hostUUID  string
	log       *slog.Logger
	isYubiHSM bool
	semaphore chan struct{}
}

func newPKCS11KeyStore(config *servicecfg.PKCS11Config, opts *Options) (*pkcs11KeyStore, error) {
	cryptoConfig := &crypto11.Config{
		Path:        config.Path,
		TokenLabel:  config.TokenLabel,
		SlotNumber:  config.SlotNumber,
		Pin:         config.PIN,
		MaxSessions: config.MaxSessions,
	}

	ctx, err := crypto11.Configure(cryptoConfig)
	if err != nil {
		return nil, trace.Wrap(err, "configuring PKCS#11 library")
	}

	pkcs11Ctx := pkcs11.New(config.Path)
	info, err := pkcs11Ctx.GetInfo()
	if err != nil {
		return nil, trace.Wrap(err, "getting PKCS#11 module info")
	}

	return &pkcs11KeyStore{
		ctx:       ctx,
		hostUUID:  opts.HostUUID,
		log:       opts.Logger,
		isYubiHSM: strings.HasPrefix(info.ManufacturerID, "Yubico"),
		semaphore: make(chan struct{}, 1),
	}, nil
}

func (p *pkcs11KeyStore) name() string {
	return storePKCS11
}

// keyTypeDescription returns a human-readable description of the types of keys
// this backend uses.
func (p *pkcs11KeyStore) keyTypeDescription() string {
	return fmt.Sprintf("PKCS#11 HSM keys created by %s", p.hostUUID)
}

func (p *pkcs11KeyStore) findUnusedID() (keyID, error) {
	if !p.isYubiHSM {
		id, err := uuid.NewRandom()
		if err != nil {
			return keyID{}, trace.Wrap(err, "generating UUID")
		}
		return keyID{
			HostID: p.hostUUID,
			KeyID:  id.String(),
		}, nil
	}

	// YubiHSM2 only supports two byte CKA_ID values.
	// ID 0 and 0xffff are reserved for internal objects by Yubico
	// https://developers.yubico.com/YubiHSM2/Concepts/Object_ID.html
	for id := uint16(1); id < 0xffff; id++ {
		idBytes := []byte{byte((id >> 8) & 0xff), byte(id & 0xff)}
		existingSigner, err := p.ctx.FindKeyPair(idBytes, nil /*label*/)
		// FindKeyPair is expected to return nil, nil if the id is not found,
		// any error is unexpected.
		if err != nil {
			return keyID{}, trace.Wrap(err)
		}
		if existingSigner == nil {
			// There is no existing keypair with this ID
			return keyID{
				HostID: p.hostUUID,
				KeyID:  fmt.Sprintf("%04x", id),
			}, nil
		}
	}
	return keyID{}, trace.AlreadyExists("failed to find unused CKA_ID for HSM")
}

// generateRSA creates a new RSAprivate key and returns its identifier and a crypto.Signer. The returned
// identifier can be passed to getSigner later to get an equivalent crypto.Signer.
func (p *pkcs11KeyStore) generateRSA(ctx context.Context, _ ...rsaKeyOption) ([]byte, crypto.Signer, error) {
	// the key identifiers are not created in a thread safe
	// manner so all calls are serialized to prevent races.
	p.semaphore <- struct{}{}
	defer func() {
		<-p.semaphore
	}()

	id, err := p.findUnusedID()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	p.log.DebugContext(ctx, "Creating new HSM keypair.", "id", id)

	ckaID, err := id.pkcs11Key(p.isYubiHSM)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer, err := p.ctx.GenerateRSAKeyPairWithLabel(ckaID, []byte(p.hostUUID), constants.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating RSA key pair")
	}

	keyID, err := id.marshal()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID, signer, nil
}

// getSigner returns a crypto.Signer for the given key identifier, if it is found.
func (p *pkcs11KeyStore) getSigner(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey) (crypto.Signer, error) {
	return p.getSignerWithoutPublicKey(ctx, rawKey)
}

func (p *pkcs11KeyStore) getSignerWithoutPublicKey(ctx context.Context, rawKey []byte) (crypto.Signer, error) {
	if t := keyType(rawKey); t != types.PrivateKeyType_PKCS11 {
		return nil, trace.BadParameter("pkcs11KeyStore cannot get signer for key type %s", t.String())
	}
	keyID, err := parsePKCS11KeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if keyID.HostID != p.hostUUID {
		return nil, trace.NotFound("given pkcs11 key is for host: %q, but this host is: %q", keyID.HostID, p.hostUUID)
	}
	pkcs11ID, err := keyID.pkcs11Key(p.isYubiHSM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := p.ctx.FindKeyPair(pkcs11ID, []byte(p.hostUUID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer == nil {
		return nil, trace.NotFound("failed to find keypair with id %v", keyID)
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
	keyID, err := parsePKCS11KeyID(raw)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return keyID.HostID == p.hostUUID, nil
}

// deleteKey deletes the given key from the HSM
func (p *pkcs11KeyStore) deleteKey(_ context.Context, rawKey []byte) error {
	keyID, err := parsePKCS11KeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if keyID.HostID != p.hostUUID {
		return trace.NotFound("pkcs11 key is for different host")
	}
	pkcs11ID, err := keyID.pkcs11Key(p.isYubiHSM)
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

// deleteUnusedKeys deletes all keys from the KeyStore if they are:
// 1. Labeled with the local HostUUID when they were created
// 2. Not included in the argument activeKeys
// This is meant to delete unused keys after they have been rotated out by a CA
// rotation.
func (p *pkcs11KeyStore) deleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	p.log.DebugContext(ctx, "Deleting unused keys from HSM.")

	// It's necessary to fetch all PublicKeys for the known activeKeys in order to
	// compare with the signers returned by FindKeyPairs below. We have no way
	// to find the CKA_ID of an unused key if it is not known.
	var activePublicKeys []*rsa.PublicKey
	for _, activeKey := range activeKeys {
		if keyType(activeKey) != types.PrivateKeyType_PKCS11 {
			continue
		}
		keyID, err := parsePKCS11KeyID(activeKey)
		if err != nil {
			return trace.Wrap(err)
		}
		if keyID.HostID != p.hostUUID {
			// This key was labeled with a foreign host UUID, it is likely not
			// present on the attached HSM and definitely will not be returned
			// by FindKeyPairs below which queries by host UUID.
			continue
		}
		signer, err := p.getSignerWithoutPublicKey(ctx, activeKey)
		if trace.IsNotFound(err) {
			// Failed to find a currently active key owned by this host.
			// The cluster is in a bad state, refuse to delete any keys.
			return trace.NotFound(
				"cannot find currently active CA key %q in HSM, aborting attempt to delete unused keys",
				keyID.KeyID)
		}
		if err != nil {
			return trace.Wrap(err)
		}
		rsaPublicKey, ok := signer.Public().(*rsa.PublicKey)
		if !ok {
			return trace.BadParameter("unknown public key type: %T", signer.Public())
		}
		activePublicKeys = append(activePublicKeys, rsaPublicKey)
	}
	keyIsActive := func(signer crypto.Signer) bool {
		rsaPublicKey, ok := signer.Public().(*rsa.PublicKey)
		if !ok {
			// unknown key type... we don't know what this is, so don't delete it
			return true
		}
		for _, k := range activePublicKeys {
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
		if keyIsActive(signer) {
			continue
		}
		p.log.InfoContext(ctx, "Deleting unused key from HSM.")
		if err := signer.Delete(); err != nil {
			// Key deletion is best-effort, log a warning on errors, and
			// continue trying to delete other keys. Errors have been observed
			// when FindKeyPairs returns duplicate keys.
			p.log.WarnContext(ctx, "Failed deleting unused key from HSM.", "error", err)
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

func (k keyID) pkcs11Key(isYubiHSM bool) ([]byte, error) {
	// YubiHSM IDs are 16 bits, stored as a hex string. In older Teleport
	// versions these keys were stored as normal UUIDs and the YubiHSM SDK
	// silently truncated them to two bytes. The first two bytes of a UUID are
	// still normal hex.
	if isYubiHSM {
		id, err := hex.DecodeString(k.KeyID[:4])
		if err != nil {
			return nil, trace.BadParameter("parsing key ID from hex: %v", err)
		}
		return id, nil
	}
	// All other IDs are UUIDs, stored in UUID string format, and the raw bytes
	// are used as the CKA_ID for the HSM.
	id, err := uuid.Parse(k.KeyID)
	if err != nil {
		return nil, trace.BadParameter("parsing key ID as UUID: %v", err)
	}
	return id[:], nil
}

func parsePKCS11KeyID(key []byte) (keyID, error) {
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
