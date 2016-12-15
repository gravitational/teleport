// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"time"
)

// RegisterRequest creates a request to enrol a new token.
func (c *Challenge) RegisterRequest() *RegisterRequest {
	var rr RegisterRequest
	rr.Version = u2fVersion
	rr.AppID = c.AppID
	rr.Challenge = encodeBase64(c.Challenge)
	return &rr
}

// Registration represents a single enrolment or pairing between an
// application and a token. This data will typically be stored in a database.
type Registration struct {
	// Raw serialized registration data as received from the token.
	Raw []byte

	KeyHandle []byte
	PubKey    ecdsa.PublicKey

	// AttestationCert can be nil for Authenticate requests.
	AttestationCert *x509.Certificate
}

type Config struct {
	// SkipAttestationVerify controls whether the token attestation
	// certificate should be verified on registration. Ideally it should
	// always be verified. However, there is currently no public list of
	// trusted attestation root certificates so it may be necessary to skip.
	SkipAttestationVerify bool
}

// Register validates a RegisterResponse message to enrol a new token.
// An error is returned if any part of the response fails to validate.
// The returned Registration should be stored by the caller.
func Register(resp RegisterResponse, c Challenge, config *Config) (*Registration, error) {
	if config == nil {
		config = &Config{}
	}

	if time.Now().Sub(c.Timestamp) > timeout {
		return nil, errors.New("u2f: challenge has expired")
	}

	regData, err := decodeBase64(resp.RegistrationData)
	if err != nil {
		return nil, err
	}

	clientData, err := decodeBase64(resp.ClientData)
	if err != nil {
		return nil, err
	}

	reg, sig, err := parseRegistration(regData)
	if err != nil {
		return nil, err
	}

	if err := verifyClientData(clientData, c); err != nil {
		return nil, err
	}

	if err := verifyAttestationCert(*reg, config); err != nil {
		return nil, err
	}

	if err := verifyRegistrationSignature(*reg, sig, c.AppID, clientData); err != nil {
		return nil, err
	}

	return reg, nil
}

func parseRegistration(buf []byte) (*Registration, []byte, error) {
	if len(buf) < 1+65+1+1+1 {
		return nil, nil, errors.New("u2f: data is too short")
	}

	var r Registration
	r.Raw = buf

	if buf[0] != 0x05 {
		return nil, nil, errors.New("u2f: invalid reserved byte")
	}
	buf = buf[1:]

	x, y := elliptic.Unmarshal(elliptic.P256(), buf[:65])
	if x == nil {
		return nil, nil, errors.New("u2f: invalid public key")
	}
	r.PubKey.Curve = elliptic.P256()
	r.PubKey.X = x
	r.PubKey.Y = y
	buf = buf[65:]

	khLen := int(buf[0])
	buf = buf[1:]
	if len(buf) < khLen {
		return nil, nil, errors.New("u2f: invalid key handle")
	}
	r.KeyHandle = buf[:khLen]
	buf = buf[khLen:]

	// The length of the x509 cert isn't specified so it has to be inferred
	// by parsing. We can't use x509.ParseCertificate yet because it returns
	// an error if there are any trailing bytes. So parse raw asn1 as a
	// workaround to get the length.
	sig, err := asn1.Unmarshal(buf, &asn1.RawValue{})
	if err != nil {
		return nil, nil, err
	}

	buf = buf[:len(buf)-len(sig)]
	cert, err := x509.ParseCertificate(buf)
	if err != nil {
		return nil, nil, err
	}
	r.AttestationCert = cert

	return &r, sig, nil
}

// Implements encoding.BinaryMarshaler.
func (r *Registration) UnmarshalBinary(data []byte) error {
	reg, _, err := parseRegistration(data)
	if err != nil {
		return err
	}
	*r = *reg
	return nil
}

// Implements encoding.BinaryUnmarshaler.
func (r *Registration) MarshalBinary() ([]byte, error) {
	return r.Raw, nil
}

func verifyAttestationCert(r Registration, config *Config) error {
	if config.SkipAttestationVerify {
		return nil
	}

	opts := x509.VerifyOptions{Roots: roots}
	_, err := r.AttestationCert.Verify(opts)
	return err
}

func verifyRegistrationSignature(
	r Registration, signature []byte, appid string, clientData []byte) error {

	appParam := sha256.Sum256([]byte(appid))
	challenge := sha256.Sum256(clientData)

	buf := []byte{0}
	buf = append(buf, appParam[:]...)
	buf = append(buf, challenge[:]...)
	buf = append(buf, r.KeyHandle...)
	pk := elliptic.Marshal(r.PubKey.Curve, r.PubKey.X, r.PubKey.Y)
	buf = append(buf, pk...)

	return r.AttestationCert.CheckSignature(
		x509.ECDSAWithSHA256, buf, signature)
}
