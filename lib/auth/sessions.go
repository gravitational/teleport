/*
Copyright 2020 Gravitational, Inc.

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

package auth

import (
	"context"
	"crypto/subtle"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

// TODO(russjones): The caller of this function that is sitting in the proxy
// needs to connect to the remote cluster to verify if the application being
// requested even exists.
//
// Does this really even matter? If the does not exist, the session will be
// created but reversetunnel subsystem will return an error. If it does exist
// but the caller does not have access, same issue.
func (s *AuthServer) createAppSession(ctx context.Context, identity tlsca.Identity, req services.CreateAppSessionRequest) (services.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that a matching web session exists in the backend.
	parentSession, err := s.GetWebSession(identity.Username, req.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if subtle.ConstantTimeCompare([]byte(parentSession.GetBearerToken()), []byte(req.BearerToken)) == 0 {
		return nil, trace.BadParameter("invalid session")
	}

	// TODO(russjones): Should Kind field on resource be a different kind or is KindWebSession okay?
	// Create a new session for the application.
	session, err := s.NewWebSession(identity.Username, identity.Groups, identity.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session.SetType(services.WebSessionSpecV2_App)
	session.SetPublicAddr(req.PublicAddr)
	session.SetParentHash(services.SessionHash(parentSession.GetName()))
	session.SetClusterName(req.ClusterName)

	// TODO(russjones): Figure this out, we don't actually have an access checker
	// here, so we can't adjust the session TTL. The only one that can pass us
	// this information is the proxy?
	//
	// Maybe roll that check plus "does this application exist?" check into one
	// and pass them in to this function in the CreateAppSessionRequest.
	//session.SetExpiryTime(s.clock.Now().Add(checker.AdjustSessionTTL(defaults.MaxCertDuration)))
	session.SetExpiryTime(s.clock.Now().Add(defaults.CertDuration))

	// Create session in backend.
	err = s.Identity.UpsertAppSession(ctx, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}
