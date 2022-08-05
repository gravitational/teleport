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

package u2f

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/flynn/u2f/u2ftoken"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

// Registration sequence:
//
//    *client*                 *messages over network*         *server*
//
//                                                         RegisterInit()
//                         <------ RegisterChallenge -----
// RegisterSignChallenge()
//                         -- RegisterChallengeResponse ->
//                                                         RegisterVerify()

type (
	// RegisterChallenge is the first message in registration sequence. It's
	// sent from the server to the client.
	RegisterChallenge = u2f.RegisterRequest
	// RegisterChallengeResponse is the second message in registration
	// sequence. It's sent from the client to the server in response to
	// RegisterChallenge.
	RegisterChallengeResponse = u2f.RegisterResponse
	// Registration is the data about the client U2F token that should be
	// stored by the server. It's created during registration sequence and is
	// needed to initiate authentication sequence.
	Registration = u2f.Registration
	// Challenge is the challenge data persisted on the server. It's used for
	// both registrations and authentications.
	Challenge = u2f.Challenge
)

// NewChallenge creates a new Challenge object for the given app.
func NewChallenge(appID string, trustedFacets []string) (*Challenge, error) {
	return u2f.NewChallenge(appID, trustedFacets)
}

// RegistrationStorage is the persistent storage needed to store temporary
// state (challenge) during the registration sequence.
type RegistrationStorage interface {
	DeviceStorage

	UpsertU2FRegisterChallenge(key string, challenge *Challenge) error
	GetU2FRegisterChallenge(key string) (*Challenge, error)
}

type inMemoryRegistrationStorage struct {
	DeviceStorage
	challenges *ttlmap.TTLMap
}

// InMemoryRegistrationStorage returns a new RegistrationStorage that stores
// registration challenges in the current process memory.
//
// Updates to existing devices are forwarded to ds.
func InMemoryRegistrationStorage(ds DeviceStorage) (RegistrationStorage, error) {
	m, err := ttlmap.New(inMemoryChallengeCapacity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inMemoryRegistrationStorage{DeviceStorage: ds, challenges: m}, nil
}

func (s inMemoryRegistrationStorage) UpsertU2FRegisterChallenge(key string, c *Challenge) error {
	return s.challenges.Set(key, c, inMemoryChallengeTTL)
}

func (s inMemoryRegistrationStorage) GetU2FRegisterChallenge(key string) (*Challenge, error) {
	v, ok := s.challenges.Get(key)
	if !ok {
		return nil, trace.NotFound("U2F challenge not found or expired")
	}
	c, ok := v.(*Challenge)
	if !ok {
		return nil, trace.NotFound("bug: U2F challenge storage returned %T instead of *u2f.Challenge", v)
	}
	return c, nil
}

// RegisterInitParams are the parameters for initiating the registration
// sequence.
type RegisterInitParams struct {
	AppConfig  types.U2F
	StorageKey string
	Storage    RegistrationStorage
}

// RegisterInit is the first step in the registration sequence. It runs on the
// server and the returned RegisterChallenge must be sent to the client.
func RegisterInit(params RegisterInitParams) (*RegisterChallenge, error) {
	c, err := NewChallenge(params.AppConfig.AppID, params.AppConfig.Facets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = params.Storage.UpsertU2FRegisterChallenge(params.StorageKey, c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := c.RegisterRequest()
	return request, nil
}

// RegisterSignChallenge is the second step in the registration sequence.  It
// runs on the client and the returned RegisterChallengeResponse must be sent
// to the server.
//
// Note: the caller must prompt the user to tap the U2F token.
func RegisterSignChallenge(ctx context.Context, c RegisterChallenge, facet string) (*RegisterChallengeResponse, error) {
	// Convert from JS-centric github.com/tstranex/u2f format to a more
	// wire-centric github.com/flynn/u2f format.
	appHash := sha256.Sum256([]byte(c.AppID))
	cd := u2f.ClientData{
		Challenge: c.Challenge,
		Origin:    facet,
		Typ:       "navigator.id.finishEnrollment",
	}
	cdRaw, err := json.Marshal(cd)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chHash := sha256.Sum256(cdRaw)
	regReq := u2ftoken.RegisterRequest{
		Challenge:   chHash[:],
		Application: appHash[:],
	}

	var regRespRaw []byte
	if err := wancli.RunOnU2FDevices(ctx, func(t wancli.Token) error {
		var err error
		regRespRaw, err = t.Register(regReq)
		return err
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(regRespRaw) == 0 {
		// This shouldn't happen if the loop above works correctly, but check
		// just in case.
		return nil, trace.CompareFailed("failed getting a registration response from a U2F device")
	}

	// Convert back from github.com/flynn/u2f to github.com/tstranex/u2f
	// format.
	base64 := base64.URLEncoding.WithPadding(base64.NoPadding)
	return &RegisterChallengeResponse{
		RegistrationData: base64.EncodeToString(regRespRaw),
		ClientData:       base64.EncodeToString(cdRaw),
	}, nil
}

// RegisterInitParams are the parameters for verifying the
// RegisterChallengeResponse.
type RegisterVerifyParams struct {
	Resp                   RegisterChallengeResponse
	DevName                string
	ChallengeStorageKey    string
	RegistrationStorageKey string
	Storage                RegistrationStorage
	Clock                  clockwork.Clock
	AttestationCAs         []string
}

// RegisterVerify is the last step in the registration sequence. It runs on the
// server and verifies the RegisterChallengeResponse returned by the client.
func RegisterVerify(ctx context.Context, params RegisterVerifyParams) (*types.MFADevice, error) {
	// TODO(awly): mfa: prevent the same key being registered twice.
	challenge, err := params.Storage.GetU2FRegisterChallenge(params.ChallengeStorageKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set SkipAttestationVerify because the u2f library has a small hardcoded
	// list of attestation CAs that's likely out of date. We'll verify the
	// attestation cert below if needed.
	reg, err := u2f.Register(params.Resp, *challenge, &u2f.Config{SkipAttestationVerify: true})
	if err != nil {
		// U2F is a 3rd party library and sends back a string based error. Wrap this error with a
		// trace.BadParameter error to allow the Web UI to unmarshal it correctly.
		return nil, trace.BadParameter(err.Error())
	}

	// Verify attestation cert if cluster config specifies CAs to trust.
	// Otherwise, accept a U2F device from any manufacturer.
	if len(params.AttestationCAs) > 0 {
		if reg.AttestationCert == nil {
			return nil, trace.BadParameter("U2F device did not return an attestation certificate during registration; make sure you're using a U2F device from a trusted manufacturer")
		}

		caPool, caNames, err := attestationCAPool(params.AttestationCAs)
		if err != nil {
			return nil, trace.BadParameter("failed parsing U2F device attestation CAs: %v", err)
		}
		if _, err := reg.AttestationCert.Verify(x509.VerifyOptions{Roots: caPool}); err != nil {
			if errors.As(err, &x509.UnknownAuthorityError{}) {
				return nil, trace.BadParameter("U2F device attestation certificate is signed by %q, but this cluster only accepts certificates from %q; make sure you're using a U2F device from a trusted manufacturer", reg.AttestationCert.Issuer, caNames)
			}
			return nil, trace.BadParameter("failed to verify U2F device attestation certificate (%v); make sure you're using a U2F device from a trusted manufacturer", err)
		}
	}

	dev, err := NewDevice(params.DevName, reg, params.Clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := params.Storage.UpsertMFADevice(ctx, params.RegistrationStorageKey, dev); err != nil {
		return nil, trace.Wrap(err)
	}

	return dev, nil
}

func attestationCAPool(cas []string) (pool *x509.CertPool, names []string, err error) {
	if len(cas) == 0 {
		return nil, nil, trace.NotFound("no attestation CAs provided")
	}
	pool = x509.NewCertPool()
	for _, ca := range cas {
		crt, err := tlsutils.ParseCertificatePEM([]byte(ca))
		if err != nil {
			return nil, nil, trace.BadParameter("U2F config has an invalid attestation CA: %v", err)
		}
		pool.AddCert(crt)
		names = append(names, crt.Subject.String())
	}
	return pool, names, nil
}
