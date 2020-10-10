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
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/wrappers"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// CreateAppWebSession creates and inserts a services.WebSession into the
// backend. The existence of a services.WebSession does not imply the user has
// access to any service (SSH, Kubernetes, or applications). A
// services.WebSession is used to match session cookies with a certificate.
// The certificate is used for all access requests, which is where access
// control is enforced.
func (s *AuthServer) CreateAppWebSession(ctx context.Context, req services.CreateAppWebSessionRequest, user services.User, checker services.AccessChecker) (services.WebSession, error) {
	// Check that a matching web session exists in the backend.
	parentSession, err := s.GetWebSession(req.Username, req.ParentSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Don't let the TTL of the child certificate go longer than the parent.
	ttl := checker.AdjustSessionTTL(parentSession.GetExpiryTime().Sub(s.clock.Now()))

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
		// Set the login to be a random string. Application certificates are never
		// used to log into servers but SSH certificate generation code requires a
		// principal be in the certificate.
		traits: wrappers.Traits(map[string][]string{
			teleport.TraitLogins: []string{uuid.New()},
		}),
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
	session := services.NewWebSession(sessionID, services.KindAppWebSession, services.WebSessionSpecV2{
		User:    req.Username,
		Priv:    privateKey,
		Pub:     certs.ssh,
		TLSCert: certs.tls,
		Expires: s.clock.Now().Add(ttl),
	})
	if err = s.Identity.UpsertAppWebSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated application web session for %v with TTL %v.", req.Username, ttl)

	return session, nil
}

// generateJWT generates an JWT token that will be passed along with every
// application request.
func (s *AuthServer) generateJWT(username string, roles []string, uri string, expires time.Time) (string, error) {
	// Get the clusters CA.
	clusterName, err := s.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	ca, err := s.GetCertAuthority(services.CertAuthID{
		Type:       services.JWTSigner,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Extract the JWT signing key and sign the claims.
	privateKey, err := ca.JWTSigner()
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

// TODO(russjones): Remove all code after here.
func (s *AuthServer) CreateAppSession(ctx context.Context, req services.CreateAppSessionRequest, user services.User, checker services.AccessChecker) (services.AppSession, error) {
	return nil, nil
	//if err := req.Check(); err != nil {
	//	return nil, trace.Wrap(err)
	//}

	//// Fetch the application the caller is requesting.
	//app, server, err := s.getApp(ctx, req.PublicAddr)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}

	//// Check if the caller has access to the requested application.
	//if err := checker.CheckAccessToApp(server.GetNamespace(), app); err != nil {
	//	log.Warnf("Access to %v denied: %v.", req.PublicAddr, err)
	//	// TODO(russjones): Hook audit log here.

	//	return nil, trace.AccessDenied("access denied")
	//}

	//// Synchronize expiration of JWT token and application session.
	//ttl := checker.AdjustSessionTTL(defaults.CertDuration)
	//expires := s.clock.Now().Add(ttl)

	//// Generate a JWT that can be re-used during the lifetime of this
	//// session to pass authentication information to the target application.
	//jwt, err := s.generateJWT(user.GetName(), user.GetRoles(), app.URI, expires)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}

	//// Create a new application session.
	//session, err := services.NewAppSession(expires, services.AppSessionSpecV3{
	//	PublicAddr: req.PublicAddr,
	//	Username:   user.GetName(),
	//	Roles:      user.GetRoles(),
	//	JWT:        jwt,
	//})
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}
	//if err := s.UpsertAppSession(ctx, session); err != nil {
	//	return nil, trace.Wrap(err)
	//}
	//log.Debugf("Generated application session for %v for %v with TTL %v.", req.PublicAddr, user.GetName(), ttl)
	//return session, nil
}

//// getApp looks for an application registered for the requested public address
//// in the cluster and returns it. In the situation multiple applications match,
//// a random selection is returned. This is done on purpose to support HA to
//// allow multiple application proxy nodes to be run and if one is down, at
//// least the application can be accessible on the other.
////
//// In the future this function should be updated to keep state on application
//// servers that are down and to not route requests to that server.
//func (s *AuthServer) getApp(ctx context.Context, publicAddr string) (*services.App, services.Server, error) {
//	var am []*services.App
//	var sm []services.Server
//
//	servers, err := s.GetAppServers(ctx, defaults.Namespace)
//	if err != nil {
//		return nil, nil, trace.Wrap(err)
//	}
//	for _, server := range servers {
//		for _, app := range server.GetApps() {
//			if app.PublicAddr == publicAddr {
//				am = append(am, app)
//				sm = append(sm, server)
//			}
//		}
//	}
//
//	if len(am) == 0 {
//		return nil, nil, trace.NotFound("%q not found", publicAddr)
//	}
//	index := rand.Intn(len(am))
//	return am[index], sm[index], nil
//}
