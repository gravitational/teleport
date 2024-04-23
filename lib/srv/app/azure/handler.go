/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package azure

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
)

// ComponentKey is the Teleport component key for this handler.
const ComponentKey = "azure:fwd"

// HandlerConfig is the configuration for an Azure app-access handler.
type HandlerConfig struct {
	// RoundTripper is the underlying transport given to an oxy Forwarder.
	RoundTripper http.RoundTripper
	// Log is the Logger.
	// TODO(greedy52) replace with slog.
	Log logrus.FieldLogger
	// Logger is the slog.Logger.
	Logger *slog.Logger
	// Clock is used to override time in tests.
	Clock clockwork.Clock

	// getAccessToken is a function for getting access token, pluggable for the sake of testing.
	getAccessToken getAccessTokenFunc
}

// CheckAndSetDefaults validates the HandlerConfig.
func (s *HandlerConfig) CheckAndSetDefaults(ctx context.Context) error {
	if s.RoundTripper == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		s.RoundTripper = tr
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.Log == nil {
		s.Log = logrus.WithField(teleport.ComponentKey, ComponentKey)
	}
	if s.Logger == nil {
		s.Logger = slog.Default().With(teleport.ComponentKey, ComponentKey)
	}
	if s.getAccessToken == nil {
		credProvider, err := findDefaultCredentialProvider(ctx, s.Logger)
		if err != nil {
			return trace.Wrap(err)
		}
		s.getAccessToken = getAccessTokenFromCredentialProvider(credProvider)
	}
	return nil
}

// handler is an Azure CLI proxy service handler that forwards the requests to Azure API, but updates the authorization headers
// based on user identity.
type handler struct {
	// config is the handler configuration.
	HandlerConfig

	// fwd is used to forward requests to Azure API after the handler has rewritten them.
	fwd *reverseproxy.Forwarder

	// tokenCache caches access tokens.
	tokenCache *utils.FnCache
}

// NewAzureHandler creates a new instance of a http.Handler for Azure requests.
func NewAzureHandler(ctx context.Context, config HandlerConfig) (http.Handler, error) {
	return newAzureHandler(ctx, config)
}

// newAzureHandler creates a new instance of a handler for Azure requests. Used by NewAzureHandler and in tests.
func newAzureHandler(ctx context.Context, config HandlerConfig) (*handler, error) {
	if err := config.CheckAndSetDefaults(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	tokenCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     time.Second * 60,
		Clock:   config.Clock,
		Context: ctx,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svc := &handler{
		HandlerConfig: config,
		tokenCache:    tokenCache,
	}

	svc.fwd, err = reverseproxy.New(
		reverseproxy.WithRoundTripper(config.RoundTripper),
		reverseproxy.WithLogger(config.Log),
		reverseproxy.WithErrorHandler(svc.formatForwardResponseError),
	)

	return svc, trace.Wrap(err)
}

// RoundTrip handles incoming requests and forwards them to the proper API.
func (s *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Body != nil {
		req.Body = utils.MaxBytesReader(w, req.Body, teleport.MaxHTTPRequestSize)
	}
	if err := s.serveHTTP(w, req); err != nil {
		s.formatForwardResponseError(w, req, err)
		return
	}
}

// serveHTTP is a helper to simplify error handling in ServeHTTP.
func (s *handler) serveHTTP(w http.ResponseWriter, req *http.Request) error {
	sessionCtx, err := common.GetSessionContext(req)
	if err != nil {
		return trace.Wrap(err)
	}
	fwdRequest, err := s.prepareForwardRequest(req, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	recorder := httplib.NewResponseStatusRecorder(w)
	s.fwd.ServeHTTP(recorder, fwdRequest)
	status := uint32(recorder.Status())

	if err := sessionCtx.Audit.OnRequest(req.Context(), sessionCtx, fwdRequest, status, nil); err != nil {
		// log but don't return the error, because we already handed off request/response handling to the oxy forwarder.
		s.Log.WithError(err).Warn("Failed to emit audit event.")
	}
	return nil
}

func (s *handler) formatForwardResponseError(rw http.ResponseWriter, r *http.Request, err error) {
	s.Log.WithError(err).Debugf("Failed to process request.")
	common.SetTeleportAPIErrorHeader(rw, err)

	// Convert trace error type to HTTP and write response.
	code := trace.ErrorToCode(err)
	http.Error(rw, http.StatusText(code), code)
}

// prepareForwardRequest prepares a request for forwarding, updating headers and target host. Several checks are made along the way.
func (s *handler) prepareForwardRequest(r *http.Request, sessionCtx *common.SessionContext) (*http.Request, error) {
	forwardedHost, err := utils.GetSingleHeader(r.Header, "X-Forwarded-Host")
	if err != nil {
		return nil, trace.AccessDenied(err.Error())
	} else if !azure.IsAzureEndpoint(forwardedHost) {
		return nil, trace.AccessDenied("%q is not an Azure endpoint", forwardedHost)
	}

	payload, err := utils.GetAndReplaceRequestBody(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy, err := http.NewRequest(r.Method, r.URL.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = forwardedHost
	reqCopy.Header = r.Header.Clone()

	err = s.replaceAuthHeaders(r, sessionCtx, reqCopy)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return reqCopy, trace.Wrap(err)
}

func getPeerKey(certs []*x509.Certificate) (crypto.PublicKey, error) {
	if len(certs) != 1 {
		return nil, trace.BadParameter("unexpected number of peer certificates: %v", len(certs))
	}

	cert := certs[0]

	pk, ok := cert.PublicKey.(crypto.PublicKey)
	if !ok {
		return nil, trace.BadParameter("peer cert public key not a crypto.Signer")
	}

	return pk, nil
}

func (s *handler) replaceAuthHeaders(r *http.Request, sessionCtx *common.SessionContext, reqCopy *http.Request) error {
	auth := reqCopy.Header.Get("Authorization")
	if auth == "" {
		s.Log.Debugf("No Authorization header present, skipping replacement.")
		return nil
	}

	pubKey, err := getPeerKey(r.TLS.PeerCertificates)
	if err != nil {
		return trace.Wrap(err)
	}

	claims, err := s.parseAuthHeader(auth, pubKey)
	if err != nil {
		return trace.Wrap(err, "failed to parse Authorization header")
	}

	s.Log.Debugf("Processing request, sessionId = %q, azureIdentity = %q, claims = %v", sessionCtx.Identity.RouteToApp.SessionID, sessionCtx.Identity.RouteToApp.AzureIdentity, claims)
	token, err := s.getToken(r.Context(), sessionCtx.Identity.RouteToApp.AzureIdentity, claims.Resource)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set new authorization
	reqCopy.Header.Set("Authorization", "Bearer "+token.Token)
	return nil
}

func (s *handler) parseAuthHeader(token string, pubKey crypto.PublicKey) (*jwt.AzureTokenClaims, error) {
	before, after, found := strings.Cut(token, " ")
	if !found {
		return nil, trace.BadParameter("Unable to parse auth header")
	}
	if before != "Bearer" {
		return nil, trace.BadParameter("Unable to parse auth header")
	}

	// Create a new key that can sign and verify tokens.
	key, err := jwt.New(&jwt.Config{
		Clock:       s.Clock,
		PublicKey:   pubKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: types.TeleportAzureMSIEndpoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key.VerifyAzureToken(after)
}

type getAccessTokenFunc func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error)

type cacheKey struct {
	managedIdentity string
	scope           string
}

const getTokenTimeout = time.Second * 5

func (s *handler) getToken(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
	key := cacheKey{managedIdentity, scope}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var tokenResult *azcore.AccessToken
	var errorResult error

	// call Clock.After() before FnCacheGet gets called in a different go-routine.
	// this ensures there is no race condition in the timeout tests, as
	// getAccessToken() ends up calling Clock.Advance() there
	timeoutChan := s.Clock.After(getTokenTimeout)

	go func() {
		tokenResult, errorResult = utils.FnCacheGet(cancelCtx, s.tokenCache, key, func(ctx context.Context) (*azcore.AccessToken, error) {
			return s.getAccessToken(ctx, managedIdentity, scope)
		})
		cancel()
	}()

	select {
	case <-timeoutChan:
		return nil, trace.Wrap(context.DeadlineExceeded, "timeout waiting for access token for %v", getTokenTimeout)
	case <-cancelCtx.Done():
		return tokenResult, errorResult
	}
}
