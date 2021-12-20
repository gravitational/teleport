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
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/flynn/u2f/u2ftoken"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

// Authentication sequence:
//
//    *client*                      *messages over network*            *server*
//
//                                                                 AuthenticateInit()
//                             <------ AuthenticateChallenge -----
// AuthenticateSignChallenge()
//                             -- AuthenticateChallengeResponse ->
//                                                                 AuthenticateVerify()

type (
	// AuthenticateChallenge is the first message in authentication sequence.
	// It's sent from the server to the client.
	AuthenticateChallenge = u2f.SignRequest
	// AuthenticateChallengeResponse is the second message in authentication
	// sequence. It's sent from the client to the server in response to
	// AuthenticateChallenge.
	AuthenticateChallengeResponse = u2f.SignResponse
)

// AuthenticationStorage is the persistent storage needed to store state
// (challenges and counters) during the authentication sequence.
type AuthenticationStorage interface {
	DeviceStorage

	UpsertU2FSignChallenge(user string, c *Challenge) error
	GetU2FSignChallenge(user string) (*Challenge, error)
}

const (
	// Set capacity at 6000. With 60s TTLs on challenges, this allows roughly
	// 100 U2F authentications/registration per second. Any larger burst or
	// sustained rate will evict oldest challenges.
	inMemoryChallengeCapacity = 6000
	inMemoryChallengeTTL      = 60 * time.Second
)

type inMemoryAuthenticationStorage struct {
	DeviceStorage
	challenges *ttlmap.TTLMap
}

// InMemoryAuthenticationStorage returns a new AuthenticationStorage that
// stores authentication challenges in the current process memory.
//
// Updates to existing devices are forwarded to ds.
func InMemoryAuthenticationStorage(ds DeviceStorage) (AuthenticationStorage, error) {
	m, err := ttlmap.New(inMemoryChallengeCapacity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inMemoryAuthenticationStorage{DeviceStorage: ds, challenges: m}, nil
}

func (s inMemoryAuthenticationStorage) UpsertU2FSignChallenge(user string, c *Challenge) error {
	return s.challenges.Set(user, c, inMemoryChallengeTTL)
}

func (s inMemoryAuthenticationStorage) GetU2FSignChallenge(user string) (*Challenge, error) {
	v, ok := s.challenges.Get(user)
	if !ok {
		return nil, trace.NotFound("U2F challenge not found or expired")
	}
	c, ok := v.(*Challenge)
	if !ok {
		return nil, trace.NotFound("bug: U2F challenge storage returned %T instead of *u2f.Challenge", v)
	}
	return c, nil
}

// AuthenticateInitParams are the parameters for initiating the authentication
// sequence.
type AuthenticateInitParams struct {
	AppConfig  types.U2F
	Devs       []*types.MFADevice
	StorageKey string
	Storage    AuthenticationStorage
}

// AuthenticateInit is the first step in the authentication sequence. It runs
// on the server and the returned AuthenticateChallenge must be sent to the
// client.
func AuthenticateInit(ctx context.Context, params AuthenticateInitParams) ([]*AuthenticateChallenge, error) {
	if len(params.Devs) == 0 {
		return nil, trace.BadParameter("bug: missing Devs field in u2f.AuthenticateInitParams")
	}

	challenge, err := NewChallenge(params.AppConfig.AppID, params.AppConfig.Facets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = params.Storage.UpsertU2FSignChallenge(params.StorageKey, challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	challenges := make([]*AuthenticateChallenge, 0, len(params.Devs))
	for _, dev := range params.Devs {
		u2fDev := dev.GetU2F()
		if u2fDev == nil {
			return nil, trace.BadParameter("bug: u2f.AuthenticateInit called with %T instead of MFADevice_U2F", dev)
		}
		reg, err := DeviceToRegistration(u2fDev)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challenges = append(challenges, challenge.SignRequest(*reg))
	}

	return challenges, nil
}

// AuthenticateSignChallenge is the second step in the authentication sequence.
// It runs on the client and the returned AuthenticationChallengeResponse must
// be sent to the server.
//
// Note: the caller must prompt the user to tap the U2F token.
func AuthenticateSignChallenge(ctx context.Context, facet string, challenges ...AuthenticateChallenge) (*AuthenticateChallengeResponse, error) {
	base64 := base64.URLEncoding.WithPadding(base64.NoPadding)

	// Convert from JS-centric github.com/tstranex/u2f format to a more
	// wire-centric github.com/flynn/u2f format.
	type authenticateRequest struct {
		orig       AuthenticateChallenge
		clientData []byte
		converted  u2ftoken.AuthenticateRequest
	}
	authRequests := make([]authenticateRequest, 0, len(challenges))
	for _, chal := range challenges {
		kh, err := base64.DecodeString(chal.KeyHandle)
		if err != nil {
			return nil, trace.BadParameter("invalid KeyHandle %q in AuthenticateChallenge: %v", chal.KeyHandle, err)
		}
		cd := u2f.ClientData{
			Challenge: chal.Challenge,
			Origin:    facet,
			Typ:       "navigator.id.getAssertion",
		}
		cdRaw, err := json.Marshal(cd)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		chHash := sha256.Sum256(cdRaw)
		appHash := sha256.Sum256([]byte(chal.AppID))
		authRequests = append(authRequests, authenticateRequest{
			orig:       chal,
			clientData: cdRaw,
			converted: u2ftoken.AuthenticateRequest{
				KeyHandle:   kh,
				Challenge:   chHash[:],
				Application: appHash[:],
			},
		})
	}

	var matchedAuthReq *authenticateRequest
	var authResp *u2ftoken.AuthenticateResponse
	if err := wancli.RunOnU2FDevices(ctx, func(t wancli.Token) error {
		var errs []error
		for _, req := range authRequests {
			if err := t.CheckAuthenticate(req.converted); err != nil {
				if err != u2ftoken.ErrUnknownKeyHandle {
					errs = append(errs, trace.Wrap(err))
				}
				continue
			}
			res, err := t.Authenticate(req.converted)
			if err == u2ftoken.ErrPresenceRequired {
				continue
			} else if err != nil {
				errs = append(errs, trace.Wrap(err))
				continue
			}
			matchedAuthReq = &req
			authResp = res
			return nil
		}
		if len(errs) > 0 {
			return trace.NewAggregate(errs...)
		}
		return errAuthNoKeyOrUserPresence
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	if authResp == nil || matchedAuthReq == nil {
		// This shouldn't happen if the loop above works correctly, but check
		// just in case.
		return nil, trace.CompareFailed("failed getting an authentication response from a U2F device")
	}

	// Convert back from github.com/flynn/u2f to github.com/tstranex/u2f
	// format.
	return &AuthenticateChallengeResponse{
		KeyHandle:     matchedAuthReq.orig.KeyHandle,
		SignatureData: base64.EncodeToString(authResp.RawResponse),
		ClientData:    base64.EncodeToString(matchedAuthReq.clientData),
	}, nil
}

var errAuthNoKeyOrUserPresence = errors.New("no U2F keys for the challenge found or user hasn't tapped the key yet")

// AuthenticateVerifyParams are the parameters for verifying the
// AuthenticationChallengeResponse.
type AuthenticateVerifyParams struct {
	Dev        *types.MFADevice
	Resp       AuthenticateChallengeResponse
	StorageKey string
	Storage    AuthenticationStorage
	Clock      clockwork.Clock
}

// AuthenticateVerify is the last step in the authentication sequence. It runs
// on the server and verifies the AuthenticateChallengeResponse returned by the
// client.
func AuthenticateVerify(ctx context.Context, params AuthenticateVerifyParams) error {
	if params.Dev == nil {
		return trace.BadParameter("no MFADevice provided")
	}
	dev := params.Dev.GetU2F()
	if dev == nil {
		return trace.BadParameter("provided MFADevice is not a U2FDevice: %T", params.Dev.Device)
	}
	reg, err := DeviceToRegistration(dev)
	if err != nil {
		return trace.Wrap(err)
	}
	challenge, err := params.Storage.GetU2FSignChallenge(params.StorageKey)
	if err != nil {
		return trace.Wrap(err)
	}
	dev.Counter, err = reg.Authenticate(params.Resp, *challenge, dev.Counter)
	if err != nil {
		return trace.Wrap(err)
	}
	params.Dev.LastUsed = params.Clock.Now()
	if err := params.Storage.UpsertMFADevice(ctx, params.StorageKey, params.Dev); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
