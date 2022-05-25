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

package touchid

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/fxamacker/cbor/v2"
	"github.com/gravitational/trace"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
)

var (
	ErrCredentialNotFound = errors.New("credential not found")
	ErrNotAvailable       = errors.New("touch ID not available")
)

// nativeTID represents the native Touch ID interface.
// Implementors must provide a global variable called `native`.
type nativeTID interface {
	IsAvailable() bool

	Register(rpID, user string, userHandle []byte) (*CredentialInfo, error)
	Authenticate(credentialID string, digest []byte) ([]byte, error)

	// FindCredentials finds credentials without user interaction.
	// An empty user means "all users".
	FindCredentials(rpID, user string) ([]CredentialInfo, error)

	// ListCredentials lists all registered credentials.
	// Requires user interaction.
	ListCredentials() ([]CredentialInfo, error)

	DeleteCredential(credentialID string) error
}

// CredentialInfo holds information about a Secure Enclave credential.
type CredentialInfo struct {
	UserHandle   []byte
	CredentialID string
	RPID         string
	User         string
	PublicKey    *ecdsa.PublicKey

	// publicKeyRaw is used internally to return public key data from native
	// register requests.
	publicKeyRaw []byte
}

// IsAvailable returns true if Touch ID is available in the system.
// Presently, IsAvailable is hidden behind a somewhat cheap check, so it may be
// prone to false positives (for example, a binary compiled with Touch ID
// support but not properly signed/notarized).
// In case of false positives, other Touch IDs should fail gracefully.
func IsAvailable() bool {
	// TODO(codingllama): Consider adding more depth to availability checks.
	//  They are prone to false positives as it stands.
	return native.IsAvailable()
}

// Register creates a new Secure Enclave-backed biometric credential.
func Register(origin string, cc *wanlib.CredentialCreation) (*wanlib.CredentialCreationResponse, error) {
	if !native.IsAvailable() {
		return nil, ErrNotAvailable
	}

	// Ignored cc fields:
	// - Timeout - we don't control touch ID timeouts (also the server is free to
	//   enforce it)
	// - CredentialExcludeList - we always allow re-registering (for various
	//   reasons).
	// - Extensions - none supported
	// - Attestation - we always to our best (packed/self-attestation).
	//   The server is free to ignore/reject.
	switch {
	case origin == "":
		return nil, errors.New("origin required")
	case cc == nil:
		return nil, errors.New("credential creation required")
	case len(cc.Response.Challenge) == 0:
		return nil, errors.New("challenge required")
	// Note: we don't need other RelyingParty fields, but technically they would
	// be required as well.
	case cc.Response.RelyingParty.ID == "":
		return nil, errors.New("relying party ID required")
	case len(cc.Response.User.ID) == 0:
		return nil, errors.New("user ID required")
	case cc.Response.User.Name == "":
		return nil, errors.New("user name required")
	case cc.Response.AuthenticatorSelection.AuthenticatorAttachment == protocol.CrossPlatform:
		return nil, fmt.Errorf("cannot fulfil authenticator attachment %q", cc.Response.AuthenticatorSelection.AuthenticatorAttachment)
	}
	ok := false
	for _, param := range cc.Response.Parameters {
		// ES256 is all we can do.
		if param.Type == protocol.PublicKeyCredentialType && param.Algorithm == webauthncose.AlgES256 {
			ok = true
			break
		}
	}
	if !ok {
		return nil, errors.New("cannot fulfil credential parameters, only ES256 are supported")
	}

	rpID := cc.Response.RelyingParty.ID
	user := cc.Response.User.Name
	userHandle := cc.Response.User.ID

	// TODO(codingllama): Handle double registrations and failures after key
	//  creation.
	resp, err := native.Register(rpID, user, userHandle)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentialID := resp.CredentialID
	pubKeyRaw := resp.publicKeyRaw

	// Parse public key and transform to the required CBOR object.
	pubKey, err := pubKeyFromRawAppleKey(pubKeyRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	x := make([]byte, 32) // x and y must have exactly 32 bytes in EC2PublicKeyData.
	y := make([]byte, 32)
	pubKey.X.FillBytes(x)
	pubKey.Y.FillBytes(y)

	pubKeyCBOR, err := cbor.Marshal(
		&webauthncose.EC2PublicKeyData{
			PublicKeyData: webauthncose.PublicKeyData{
				KeyType:   int64(webauthncose.EllipticKey),
				Algorithm: int64(webauthncose.AlgES256),
			},
			// See https://datatracker.ietf.org/doc/html/rfc8152#section-13.1.
			Curve:  1, // P-256
			XCoord: x,
			YCoord: y,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attData, err := makeAttestationData(
		protocol.CreateCeremony, origin, rpID, cc.Response.Challenge,
		&credentialData{
			id:         credentialID,
			pubKeyCBOR: pubKeyCBOR,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sig, err := native.Authenticate(credentialID, attData.digest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attObj, err := cbor.Marshal(protocol.AttestationObject{
		RawAuthData: attData.rawAuthData,
		Format:      "packed",
		AttStatement: map[string]interface{}{
			"alg": int64(webauthncose.AlgES256),
			"sig": sig,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wanlib.CredentialCreationResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   credentialID,
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: []byte(credentialID),
		},
		AttestationResponse: wanlib.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: attData.ccdJSON,
			},
			AttestationObject: attObj,
		},
	}, nil
}

func pubKeyFromRawAppleKey(pubKeyRaw []byte) (*ecdsa.PublicKey, error) {
	// Verify key length to avoid a potential panic below.
	// 3 is the smallest number that clears it, but in practice 65 is the more
	// common length.
	// Apple's docs make no guarantees, hence no assumptions are made here.
	if len(pubKeyRaw) < 3 {
		return nil, fmt.Errorf("public key representation too small (%v bytes)", len(pubKeyRaw))
	}

	// "For an elliptic curve public key, the format follows the ANSI X9.63
	// standard using a byte string of 04 || X || Y. (...) All of these
	// representations use constant size integers, including leading zeros as
	// needed."
	// https://developer.apple.com/documentation/security/1643698-seckeycopyexternalrepresentation?language=objc
	pubKeyRaw = pubKeyRaw[1:] // skip 0x04
	l := len(pubKeyRaw) / 2
	x := pubKeyRaw[:l]
	y := pubKeyRaw[l:]

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     (&big.Int{}).SetBytes(x),
		Y:     (&big.Int{}).SetBytes(y),
	}, nil
}

type credentialData struct {
	id         string
	pubKeyCBOR []byte
}

type attestationResponse struct {
	ccdJSON     []byte
	rawAuthData []byte
	digest      []byte
}

// TODO(codingllama): Share a single definition with webauthncli / mocku2f.
type collectedClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func makeAttestationData(ceremony protocol.CeremonyType, origin, rpID string, challenge []byte, cred *credentialData) (*attestationResponse, error) {
	// Sanity check.
	isCreate := ceremony == protocol.CreateCeremony
	if isCreate && cred == nil {
		return nil, fmt.Errorf("cred required for %q ceremony", ceremony)
	}

	ccd := &collectedClientData{
		Type:      string(ceremony),
		Challenge: base64.RawURLEncoding.EncodeToString(challenge),
		Origin:    origin,
	}
	ccdJSON, err := json.Marshal(ccd)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccdJSON)
	rpIDHash := sha256.Sum256([]byte(rpID))

	flags := byte(protocol.FlagUserPresent | protocol.FlagUserVerified)
	if isCreate {
		flags |= byte(protocol.FlagAttestedCredentialData)
	}

	authData := &bytes.Buffer{}
	authData.Write(rpIDHash[:])
	authData.WriteByte(flags)
	binary.Write(authData, binary.BigEndian, uint32(0)) // signature counter
	// Attested credential data begins here.
	if isCreate {
		authData.Write(make([]byte, 16))                               // aaguid
		binary.Write(authData, binary.BigEndian, uint16(len(cred.id))) // credentialIdLength
		authData.Write([]byte(cred.id))
		authData.Write(cred.pubKeyCBOR)
	}
	rawAuthData := authData.Bytes()

	dataToSign := append(rawAuthData, ccdHash[:]...)
	digest := sha256.Sum256(dataToSign)
	return &attestationResponse{
		ccdJSON:     ccdJSON,
		rawAuthData: rawAuthData,
		digest:      digest[:],
	}, nil
}

// Login authenticates using a Secure Enclave-backed biometric credential.
// It returns the assertion response and the user that owns the credential to
// sign it.
func Login(origin, user string, assertion *wanlib.CredentialAssertion) (*wanlib.CredentialAssertionResponse, string, error) {
	if !native.IsAvailable() {
		return nil, "", ErrNotAvailable
	}

	// Ignored assertion fields:
	// - Timeout - we don't control touch ID timeouts (also the server is free to
	//   enforce it)
	// - UserVerification - always performed
	// - Extensions - none supported
	switch {
	case origin == "":
		return nil, "", errors.New("origin required")
	case assertion == nil:
		return nil, "", errors.New("assertion required")
	case len(assertion.Response.Challenge) == 0:
		return nil, "", errors.New("challenge required")
	case assertion.Response.RelyingPartyID == "":
		return nil, "", errors.New("relying party ID required")
	}

	// TODO(codingllama): Share the same LAContext between search and
	//  authentication, so we can protect both with user interaction.
	rpID := assertion.Response.RelyingPartyID
	infos, err := native.FindCredentials(rpID, user)
	switch {
	case err != nil:
		return nil, "", trace.Wrap(err)
	case len(infos) == 0:
		return nil, "", ErrCredentialNotFound
	}

	// Verify infos against allowed credentials, if any.
	var cred *CredentialInfo
	if len(assertion.Response.AllowedCredentials) > 0 {
		for _, info := range infos {
			for _, allowedCred := range assertion.Response.AllowedCredentials {
				if info.CredentialID == string(allowedCred.CredentialID) {
					cred = &info
					break
				}
			}
		}
	} else {
		cred = &infos[0]
	}
	if cred == nil {
		return nil, "", ErrCredentialNotFound
	}

	attData, err := makeAttestationData(protocol.AssertCeremony, origin, rpID, assertion.Response.Challenge, nil /* cred */)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	sig, err := native.Authenticate(cred.CredentialID, attData.digest)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   cred.CredentialID,
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: []byte(cred.CredentialID),
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: attData.ccdJSON,
			},
			AuthenticatorData: attData.rawAuthData,
			Signature:         sig,
			UserHandle:        cred.UserHandle,
		},
	}, cred.User, nil
}

// ListCredentials lists all registered Secure Enclave credentials.
// Requires user interaction.
func ListCredentials() ([]CredentialInfo, error) {
	// Skipped IsAvailable check in favor of a direct call to native.
	infos, err := native.ListCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse public keys.
	for i := range infos {
		info := &infos[i]
		key, err := pubKeyFromRawAppleKey(info.publicKeyRaw)
		if err != nil {
			log.Warnf("Failed to convert public key: %v", err)
		}
		info.PublicKey = key // this is OK, even if it's nil
		info.publicKeyRaw = nil
	}

	return infos, nil
}

// DeleteCredential deletes a Secure Enclave credential.
// Requires user interaction.
func DeleteCredential(credentialID string) error {
	// Skipped IsAvailable check in favor of a direct call to native.
	return native.DeleteCredential(credentialID)
}
