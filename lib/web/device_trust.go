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
		h.log.
			WithError(err).
			WithField("user", sessionCtx.GetUser()).
			Warn("Device web authentication confirm failed")
		// err swallowed on purpose.
	default:
		// Preemptively release session from cache, as its certificates are now
		// updated.
		// The WebSession watcher takes care of this in other proxy instances
		// (see [sessionCache.watchWebSessions]).
		h.auth.releaseResources(sessionCtx.GetUser(), sessionCtx.GetSessionID())
	}

	// Always redirect back to the dashboard, regardless of outcome.
	app.SetRedirectPageHeaders(w.Header(), "" /* nonce */)

	redirectTo, err := h.getRedirectPath(unsafeRedirectURI)
	if err != nil {
		h.log.
			WithError(err).
			WithField("redirect_uri", unsafeRedirectURI).
			Debug("Unable to parse redirectURI")
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)

	return nil, nil
}

// getRedirectPath tries to parse the given redirectURI. It will always return a redirect url
// even if the parse fails (in case of failture, the returned string is "/web")
func (h *Handler) getRedirectPath(redirectURI string) (string, error) {
	const basePath = "/web"

	if redirectURI == "" {
		return basePath, nil
	}

	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return basePath, trace.Wrap(err)
	}

	cleanPath := path.Clean(parsedURL.Path)
	// helps in situations where there is no path such as https://example.com
	if cleanPath == "." || cleanPath == ".." {
		cleanPath = "/"
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// Prepend "/web" only if it's not already present
	if !strings.HasPrefix(cleanPath, basePath) {
		return path.Join(basePath, cleanPath), nil
	}
	return cleanPath, nil
}
