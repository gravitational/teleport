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

	// Create a new session for the application.
	session, err := s.NewWebSession(identity.Username, identity.Groups, identity.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session.SetType(services.WebSessionSpecV2_App)
	session.SetPublicAddr(req.PublicAddr)
	session.SetParentHash(services.SessionHash(parentSession.GetName()))
	session.SetClusterName(req.ClusterName)

	// TODO(russjones): The proxy should use it's access to the AccessPoint of
	// the remote host to provide the maximum length of the session here.
	// However, enforcement of that session length should occur in lib/srv/app.
	session.SetExpiryTime(s.clock.Now().Add(defaults.CertDuration))

	// Create session in backend.
	err = s.Identity.UpsertAppSession(ctx, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}
