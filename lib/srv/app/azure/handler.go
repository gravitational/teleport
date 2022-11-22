// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/oxy/forward"
	oxyutils "github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// ForwarderConfig is the Forwarder configuration.
type ForwarderConfig struct {
	// Client is an HTTP client instance used for HTTP calls.
	Client *http.Client
	// Log is the Logger.
	Log logrus.FieldLogger
	// Clock is used to override time in tests.
	Clock clockwork.Clock

	// getAccessToken is a function for getting access token, pluggable for the sake of testing.
	getAccessToken getAccessTokenFunc
}

// CheckAndSetDefaults validates the ForwarderConfig config.
func (s *ForwarderConfig) CheckAndSetDefaults() error {
	if s.Client == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		s.Client = &http.Client{
			Transport: tr,
		}
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.Log == nil {
		s.Log = logrus.WithField(trace.Component, "azure:fwd")
	}
	if s.getAccessToken == nil {
		s.getAccessToken = getAccessTokenManagedIdentity
		if func() string { return runtime.GOOS }() != "linux" {
			s.getAccessToken = getAccessTokenRemoteServer
		}
	}
	return nil
}

// TODO: remove testing code
func getAccessTokenRemoteServer(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
	cmd := exec.Command("ssh", "azureuser@13.74.28.87", "/home/azureuser/teleport/build/tazure", "managed_id_token", managedIdentity, scope)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var token azcore.AccessToken
	err = json.Unmarshal(out, &token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &token, nil
}

// Forwarder is an AWS CLI proxy service that signs AWS requests
// based on user identity.
type Forwarder struct {
	// ForwarderConfig is the Forwarder configuration.
	ForwarderConfig

	// Forwarder signs and forwards the request to Azure API.
	*forward.Forwarder

	// tokenCache caches access tokens.
	tokenCache *utils.FnCache
}

// NewForwarder creates a new instance of Forwarder.
func NewForwarder(ctx context.Context, config ForwarderConfig) (*Forwarder, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	tokenCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     time.Second * 60,
		Clock:   config.Clock,
		Context: ctx,
	})

	svc := &Forwarder{
		ForwarderConfig: config,
		tokenCache:      tokenCache,
	}

	fwd, err := forward.New(
		forward.RoundTripper(svc),
		forward.ErrorHandler(oxyutils.ErrorHandlerFunc(svc.formatForwardResponseError)),
		forward.PassHostHeader(true),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svc.Forwarder = fwd
	return svc, nil
}

// RoundTrip handles incoming requests and forwards them to the proper API.
func (s *Forwarder) RoundTrip(req *http.Request) (*http.Response, error) {
	sessionCtx, err := common.GetSessionContext(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwdRequest, err := s.prepareForwardRequest(req, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := s.Client.Do(fwdRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := sessionCtx.Audit.OnRequest(req.Context(), sessionCtx, fwdRequest, resp, nil); err != nil {
		s.Log.WithError(err).Warn("Failed to emit audit event.")
	}

	return resp, nil
}

func (s *Forwarder) formatForwardResponseError(rw http.ResponseWriter, r *http.Request, err error) {
	switch trace.Unwrap(err).(type) {
	case *trace.BadParameterError:
		s.Log.Debugf("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusBadRequest)
	case *trace.AccessDeniedError:
		s.Log.Infof("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusForbidden)
	default:
		s.Log.Warnf("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusInternalServerError)
	}
}

// prepareForwardRequest prepares a request for forwarding, updating headers and target host. Several checks are made along the way.
func (s *Forwarder) prepareForwardRequest(r *http.Request, sessionCtx *common.SessionContext) (*http.Request, error) {
	forwardedHost := r.Header.Get("X-Forwarded-Host")
	if !azure.IsAzureEndpoint(forwardedHost) {
		return nil, trace.AccessDenied("%q is not an Azure endpoint", forwardedHost)
	}

	payload, err := awsutils.GetAndReplaceReqBody(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy, err := http.NewRequest(r.Method, r.URL.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = forwardedHost

	copyHeaders(r, reqCopy)

	err = s.replaceAuthHeaders(r.Context(), sessionCtx, reqCopy)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return reqCopy, trace.Wrap(err)
}

func (s *Forwarder) replaceAuthHeaders(ctx context.Context, sessionCtx *common.SessionContext, reqCopy *http.Request) error {
	auth := reqCopy.Header.Get("Authorization")
	if auth == "" {
		s.Log.Debugf("No Authorization header present, skipping replacement.")
		return nil
	}

	scope, teleportUUID, err := parseAuthHeader(auth)
	if err != nil {
		return trace.Wrap(err, "failed to parse Authorization header")
	}

	s.Log.Debugf("Processing request, scope = %q, fingerprint = %q", scope, teleportUUID)

	identity := sessionCtx.Identity.RouteToApp.AzureIdentity
	token, err := s.getToken(ctx, identity, scope)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set new authorization
	reqCopy.Header.Set("Authorization", "Bearer "+token.Token)
	return nil
}

func parseAuthHeader(auth string) (string, string, error) {
	// TODO: implement parseAuthHeader in a less hackish way.
	parts := strings.Split(auth, ".")
	if len(parts) != 3 {
		return "", "", trace.BadParameter("wrong number of parts in Auth header")
	}

	claimsJSON, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", "", trace.Wrap(err)
	}

	scopeClaim, ok := claims["resource"]
	if !ok {
		return "", "", trace.BadParameter("missing 'resource' in claims")
	}
	scope, ok := scopeClaim.(string)
	if !ok {
		return "", "", trace.BadParameter("wrong type for 'resource' claim: %T", scopeClaim)
	}

	teleportUUIDClaim, ok := claims["teleport_mark"]
	if !ok {
		return "", "", trace.BadParameter("missing 'teleport_mark' in claims")
	}
	teleportUUID, ok := teleportUUIDClaim.(string)
	if !ok {
		return "", "", trace.BadParameter("wrong type for 'teleport_mark' claim: %T", teleportUUIDClaim)
	}

	return scope, teleportUUID, nil
}

type getAccessTokenFunc func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error)

func getAccessTokenManagedIdentity(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
	identityCredential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{ID: azidentity.ResourceID(managedIdentity)})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := policy.TokenRequestOptions{Scopes: []string{scope}}
	token, err := identityCredential.GetToken(ctx, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &token, nil
}

type cacheKey struct {
	managedIdentity string
	scope           string
}

func (s *Forwarder) getToken(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
	key := cacheKey{managedIdentity, scope}

	return utils.FnCacheGet(ctx, s.tokenCache, key, func(ctx context.Context) (*azcore.AccessToken, error) {
		return s.getAccessToken(ctx, managedIdentity, scope)
	})
}

func copyHeaders(r *http.Request, reqCopy *http.Request) {
	for key, values := range r.Header {
		// Remove Teleport app headers.
		if common.IsReservedHeader(key) {
			continue
		}

		for _, v := range values {
			reqCopy.Header.Add(key, v)
		}
	}
	reqCopy.Header.Del("Content-Length")
}
