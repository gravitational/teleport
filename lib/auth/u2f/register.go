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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"
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
//
// TODO(awly): implement RegisterSignChallenge

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
	UpsertU2FRegisterChallenge(key string, challenge *Challenge) error
	GetU2FRegisterChallenge(key string) (*Challenge, error)

	UpsertU2FRegistration(key string, reg *Registration) error
	UpsertU2FRegistrationCounter(key string, counter uint32) error
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

// RegisterInitParams are the parameters for verifying the
// RegisterChallengeResponse.
type RegisterVerifyParams struct {
	Resp                   RegisterChallengeResponse
	ChallengeStorageKey    string
	RegistrationStorageKey string
	Storage                RegistrationStorage
}

// RegisterVerify is the last step in the registration sequence. It runs on the
// server and verifies the RegisterChallengeResponse returned by the client.
func RegisterVerify(params RegisterVerifyParams) error {
	challenge, err := params.Storage.GetU2FRegisterChallenge(params.ChallengeStorageKey)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set SkipAttestationVerify because we don't yet know what vendor CAs to
	// trust. For now, this means accepting U2F devices created by anyone.
	reg, err := u2f.Register(params.Resp, *challenge, &u2f.Config{SkipAttestationVerify: true})
	if err != nil {
		// U2F is a 3rd party library and sends back a string based error. Wrap this error with a
		// trace.BadParameter error to allow the Web UI to unmarshal it correctly.
		return trace.BadParameter(err.Error())
	}

	if err := params.Storage.UpsertU2FRegistration(params.RegistrationStorageKey, reg); err != nil {
		return trace.Wrap(err)
	}
	if err := params.Storage.UpsertU2FRegistrationCounter(params.RegistrationStorageKey, 0); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
