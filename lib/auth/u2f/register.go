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
	"encoding/json"
	"io"
	"os/exec"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mailgun/ttlmap"
	"github.com/tstranex/u2f"

	"github.com/gravitational/teleport/api/types"
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
	challenges *ttlmap.TtlMap
}

// InMemoryRegistrationStorage returns a new RegistrationStorage that stores
// registration challenges in the current process memory.
//
// Updates to existing devices are forwarded to ds.
func InMemoryRegistrationStorage(ds DeviceStorage) (RegistrationStorage, error) {
	m, err := ttlmap.NewMap(inMemoryChallengeCapacity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inMemoryRegistrationStorage{DeviceStorage: ds, challenges: m}, nil
}

func (s inMemoryRegistrationStorage) UpsertU2FRegisterChallenge(key string, c *Challenge) error {
	return s.challenges.Set(key, c, int(inMemoryChallengeTTL.Seconds()))
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
	// Pass the JSON-encoded data undecoded to the u2f-host binary
	challengeRaw, err := json.Marshal(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd := exec.CommandContext(ctx, "u2f-host", "--action=register", "--origin="+facet)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		// If we returned before cmd.Wait was called, clean up the spawned
		// process. ProcessState will be empty until cmd.Wait or cmd.Run
		// return.
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			cmd.Process.Kill()
		}
	}()
	_, err = stdin.Write(challengeRaw)
	stdin.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 16kB ought to be enough for anybody.
	signResponseBuf := make([]byte, 16*1024)
	n, err := io.ReadFull(stdout, signResponseBuf)
	// unexpected EOF means we have read the data completely.
	if err == nil {
		return nil, trace.LimitExceeded("u2f sign response exceeded buffer size")
	}
	signResponse := signResponseBuf[:n]

	// Read error message (if any). 1kB is more than enough for any error message u2f-host outputs
	errMsgBuf := make([]byte, 1024)
	n, err = io.ReadFull(stderr, errMsgBuf)
	if err == nil {
		return nil, trace.LimitExceeded("u2f error message exceeded buffer size")
	}
	errMsg := string(errMsgBuf[:n])

	err = cmd.Wait()
	if err != nil {
		return nil, trace.AccessDenied("u2f-host returned error: %s", errMsg)
	} else if len(signResponse) == 0 {
		return nil, trace.NotFound("u2f-host returned no error and no sign response")
	}

	var resp RegisterChallengeResponse
	if err := json.Unmarshal(signResponse, &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resp, nil
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
}

// RegisterVerify is the last step in the registration sequence. It runs on the
// server and verifies the RegisterChallengeResponse returned by the client.
func RegisterVerify(ctx context.Context, params RegisterVerifyParams) (*types.MFADevice, error) {
	// TODO(awly): mfa: prevent the same key being registered twice.
	challenge, err := params.Storage.GetU2FRegisterChallenge(params.ChallengeStorageKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set SkipAttestationVerify because we don't yet know what vendor CAs to
	// trust. For now, this means accepting U2F devices created by anyone.
	reg, err := u2f.Register(params.Resp, *challenge, &u2f.Config{SkipAttestationVerify: true})
	if err != nil {
		// U2F is a 3rd party library and sends back a string based error. Wrap this error with a
		// trace.BadParameter error to allow the Web UI to unmarshal it correctly.
		return nil, trace.BadParameter(err.Error())
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
