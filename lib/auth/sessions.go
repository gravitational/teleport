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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

func (s *AuthServer) createAppSession(ctx context.Context, identity tlsca.Identity, req services.CreateAppSessionRequest) (services.WebSession, error) {
	// Build the access checked based off the logged in identity of caller.
	checker, err := services.FetchRoles(identity.Groups, s.Access, identity.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if the caller has access to the app requested.
	//app, err := s.cache.GetApp(ctx, defaults.Namespace, req.AppName)
	app, err := s.Presence.GetApp(ctx, defaults.Namespace, req.AppName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = checker.CheckAccessToApp(app)
	if err != nil {
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
	parentHash := sha256.Sum256([]byte(parentSession.GetName()))
	session.SetParentHash(hex.EncodeToString(parentHash[:]))
	session.SetType(services.WebSessionSpecV2_App)
	// TODO: Set websession kind different?
	// TODO: Set expiry to be whatever the role says.

	// Create session in backend.
	err = s.Identity.UpsertWebSession(identity.Username, session.GetName(), session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO this will be used to check if the session exists.
	// TODO: GetAppSession()

	return session, nil
}
