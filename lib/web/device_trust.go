// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package web

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/web/app"
)

// deviceWebConfirm is the last step in device web authentication, where the
// "authenticator process" (aka Connect) forwards the DeviceConfirmationToken
// back to the Auth Server, via the Proxy.
//
// GET /webapi/devices/webconfirm?id=a&token=b
//
// - id: ID of the confirmation token.
// - token: raw confirmation token.
//
// The result of this call is a redirect to "/web", regardless of the outcome of
// the ConfirmDeviceWebAuthentication RPC.
func (h *Handler) deviceWebConfirm(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sessionCtx *SessionContext) (interface{}, error) {
	query := r.URL.Query()

	// Read input parameters.
	confirmToken := &devicepb.DeviceConfirmationToken{}
	confirmToken.Id = query.Get("id")
	confirmToken.Token = query.Get("token")
	unsafeRedirectURI := query.Get("redirect_uri")

	switch {
	case confirmToken.Id == "":
		return nil, trace.BadParameter("parameter id required")
	case confirmToken.Token == "":
		return nil, trace.BadParameter("parameter token required")
	}

	// Use the Proxy identity for this call. Only the Proxy is allowed to do it.
	devicesClient := h.GetProxyClient().DevicesClient()
	ctx := r.Context()

	_, err := devicesClient.ConfirmDeviceWebAuthentication(ctx, &devicepb.ConfirmDeviceWebAuthenticationRequest{
		ConfirmationToken:   confirmToken,
		CurrentWebSessionId: sessionCtx.GetSessionID(),
	})
	switch {
	case err != nil:
		h.logger.WarnContext(ctx, "Device web authentication confirm failed",
			"error", err,
			"user", sessionCtx.GetUser(),
		)
		// err swallowed on purpose.
	default:
		// Preemptively release session from cache, as its certificates are now
		// updated.
		// The WebSession watcher takes care of this in other proxy instances
		// (see [sessionCache.watchWebSessions]).
		h.auth.releaseResources(r.Context(), sessionCtx.GetUser(), sessionCtx.GetSessionID())
	}

	// Always redirect back to the dashboard, regardless of outcome.
	app.SetRedirectPageHeaders(w.Header(), "" /* nonce */)

	redirectTo, err := h.getRedirectURL(r.Host, unsafeRedirectURI)
	if err != nil {
		h.logger.DebugContext(ctx, "Unable to parse redirectURI",
			"error", err,
			"redirect_uri", unsafeRedirectURI,
		)
		http.Error(w, http.StatusText(trace.ErrorToCode(err)), trace.ErrorToCode(err))
		return nil, nil
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)

	return nil, nil
}

// getRedirectPath tries to parse the given unsafeRedirectURI.
// It returns a full URL if the unsafeRedirectURI points to SAML IdP SSO endpoint.
// In any other case, as long as the redirect URL is parsable, it returns
// a path ensuring its prefixed with "/web".
func (h *Handler) getRedirectURL(host, unsafeRedirectURI string) (string, error) {
	const (
		basePath                = "/web"
		samlSPInitiatedSSOPath  = "/enterprise/saml-idp/sso"
		samlIDPInitiatedSSOPath = "/enterprise/saml-idp/login"
	)

	if unsafeRedirectURI == "" {
		return basePath, nil
	}

	parsedURL, err := url.Parse(unsafeRedirectURI)
	if err != nil {
		return basePath, trace.BadParameter("invalid redirect URL")
	}

	cleanPath := path.Clean(parsedURL.Path)
	// helps in situations where there is no path such as https://example.com
	if cleanPath == "." || cleanPath == ".." {
		cleanPath = "/"
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// IDP initiated SSO path format: "/enterprise/saml-idp/login/<service provider name>"
	isIdpInitiatedSSOPath := strings.HasPrefix(cleanPath, samlIDPInitiatedSSOPath) && len(strings.Split(cleanPath, "/")) == 5
	if cleanPath == samlSPInitiatedSSOPath || isIdpInitiatedSSOPath {
		if parsedURL.Host != host {
			return "", trace.BadParameter("host mismatch")
		}
		path := samlSPInitiatedSSOPath
		if isIdpInitiatedSSOPath {
			path = cleanPath
		}
		ensuredURL := &url.URL{
			Scheme:   "https",
			Host:     host,
			Path:     path,
			RawQuery: parsedURL.RawQuery,
		}
		return ensuredURL.String(), nil
	}

	// Prepend "/web" only if it's not already present
	if !strings.HasPrefix(cleanPath, basePath) {
		return path.Join(basePath, cleanPath), nil
	}
	return cleanPath, nil
}
