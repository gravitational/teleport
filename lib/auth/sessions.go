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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
)

// CreateAppSession creates and inserts a services.WebSession into the
// backend with the identity of the caller used to generate the certificate.
// The certificate is used for all access requests, which is where access
// control is enforced.
func (s *Server) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest, user types.User, identity tlsca.Identity, checker services.AccessChecker) (types.WebSession, error) {
	if !modules.GetModules().Features().App {
		return nil, trace.AccessDenied(
			"this Teleport cluster doesn't support application access, please contact the cluster administrator")
	}

	// Don't let the app session go longer than the identity expiration,
	// which matches the parent web session TTL as well.
	//
	// When using web-based app access, the browser will send a cookie with
	// sessionID which will be used to fetch services.WebSession which
	// contains a certificate whose life matches the life of the session
	// that will be used to establish the connection.
	ttl := checker.AdjustSessionTTL(identity.Expires.Sub(s.clock.Now()))

	// Encode user traits in the app access certificate. This will allow to
	// pass user traits when talking to app servers in leaf clusters.
	_, traits, err := services.ExtractFromIdentity(s, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create certificate for this session.
	privateKey, publicKey, err := s.GetNewKeyPairFromPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := s.generateUserCert(certRequest{
		user:      user,
		publicKey: publicKey,
		checker:   checker,
		ttl:       ttl,
		traits:    traits,
		// Only allow this certificate to be used for applications.
		usage: []string{teleport.UsageAppsOnly},
		// Add in the application routing information.
		appSessionID:   uuid.New(),
		appPublicAddr:  req.PublicAddr,
		appClusterName: req.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session := types.NewWebSession(sessionID, services.KindWebSession, services.KindAppSession, types.WebSessionSpecV2{
		User:    req.Username,
		Priv:    privateKey,
		Pub:     certs.ssh,
		TLSCert: certs.tls,
		Expires: s.clock.Now().Add(ttl),
	})
	if err = s.Identity.UpsertAppSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated application web session for %v with TTL %v.", req.Username, ttl)
	UserLoginCount.Inc()
	return session, nil
}

// WaitForAppSession will block until the requested application session shows up in the
// cache or a timeout occurs.
func WaitForAppSession(ctx context.Context, sessionID, user string, ap AccessPoint) error {
	_, err := ap.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: sessionID})
	if err == nil {
		return nil
	}
	logger := log.WithField("session", sessionID)
	if !trace.IsNotFound(err) {
		logger.WithError(err).Debug("Failed to query application session.")
	}
	// Establish a watch on application session.
	watcher, err := ap.NewWatcher(ctx, types.Watch{
		Name: teleport.ComponentAppProxy,
		Kinds: []types.WatchKind{
			{
				Kind:    services.KindWebSession,
				SubKind: services.KindAppSession,
				Filter:  (&types.WebSessionFilter{User: user}).IntoMap(),
			},
		},
		MetricComponent: teleport.ComponentAppProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	matchEvent := func(event types.Event) (types.Resource, error) {
		if event.Type == backend.OpPut &&
			event.Resource.GetKind() == services.KindWebSession &&
			event.Resource.GetSubKind() == services.KindAppSession &&
			event.Resource.GetName() == sessionID {
			return event.Resource, nil
		}
		return nil, trace.CompareFailed("no match")
	}
	_, err = local.WaitForEvent(ctx, watcher, local.EventMatcherFunc(matchEvent), clockwork.NewRealClock())
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for application session.")
		// See again if we maybe missed the event but the session was actually created.
		if _, err := ap.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: sessionID}); err == nil {
			return nil
		}
	}
	return trace.Wrap(err)
}

// generateAppToken generates an JWT token that will be passed along with every
// application request.
func (s *Server) generateAppToken(username string, roles []string, uri string, expires time.Time) (string, error) {
	// Get the clusters CA.
	clusterName, err := s.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	ca, err := s.GetCertAuthority(types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Extract the JWT signing key and sign the claims.
	privateKey, err := services.GetJWTSigner(ca, s.clock)
	if err != nil {
		return "", trace.Wrap(err)
	}
	token, err := privateKey.Sign(jwt.SignParams{
		Username: username,
		Roles:    roles,
		URI:      uri,
		Expires:  expires,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

func (s *Server) createWebSession(ctx context.Context, req types.NewWebSessionRequest) (types.WebSession, error) {
	// It's safe to extract the roles and traits directly from services.User
	// because this occurs during the user creation process and services.User
	// is not fetched from the backend.
	session, err := s.NewWebSession(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.upsertWebSession(ctx, req.User, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *Server) createSessionCert(user types.User, sessionTTL time.Duration, publicKey []byte, compatibility, routeToCluster, kubernetesCluster string) ([]byte, []byte, error) {
	// It's safe to extract the roles and traits directly from services.User
	// because this occurs during the user creation process and services.User
	// is not fetched from the backend.
	checker, err := services.FetchRoles(user.GetRoles(), s.Access, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := s.generateUserCert(certRequest{
		user:              user,
		ttl:               sessionTTL,
		publicKey:         publicKey,
		compatibility:     compatibility,
		checker:           checker,
		traits:            user.GetTraits(),
		routeToCluster:    routeToCluster,
		kubernetesCluster: kubernetesCluster,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return certs.ssh, certs.tls, nil
}
