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

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// withRouterAuth authenticates requests then hands the request to a
// httprouter.Handler handler.
func (h *Handler) withRouterAuth(handler routerAuthFunc) httprouter.Handle {
	return makeRouterHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		session, err := h.authenticate(r.Context(), r)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := handler(w, r, p, session); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

// withAuth authenticates requests then hands the request to a http.HandlerFunc
// handler.
func (h *Handler) withAuth(handler handlerAuthFunc) http.HandlerFunc {
	return makeHandler(func(w http.ResponseWriter, r *http.Request) error {
		// If the caller fails to authenticate, redirect the caller to Teleport.
		session, err := h.authenticate(r.Context(), r)
		if err != nil {
			if redirectErr := h.redirectToLauncher(w, r, launcherURLParams{}); redirectErr == nil {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := handler(w, r, session); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

// redirectToLauncher redirects to the proxy web's app launcher if the public
// address of the proxy is set.
func (h *Handler) redirectToLauncher(w http.ResponseWriter, r *http.Request, p launcherURLParams) error {
	if p.stateToken == "" && !HasSessionCookie(r) {
		// Reaching this block means the application was accessed through the CLI (eg: tsh app login)
		// and there was a forwarding error and we could not renew the app web session.
		// Since we can't redirect the user to the app launcher from the CLI,
		// we just return an error instead.
		return trace.BadParameter("redirecting to launcher when using client certificate, is not allowed")
	}

	if h.c.WebPublicAddr == "" {
		// The error below tends to be swallowed by the Web UI, so log a warning for
		// admins as well.
		h.log.Error("" +
			"Application Service requires public_addr to be set in the Teleport Proxy Service configuration. " +
			"Please contact your Teleport cluster administrator or refer to " +
			"https://goteleport.com/docs/application-access/guides/connecting-apps/#start-authproxy-service.")
		return trace.BadParameter("public address of the proxy is not set")
	}
	addr, err := utils.ParseAddr(r.Host)
	if err != nil {
		return trace.Wrap(err)
	}

	urlString := makeAppRedirectURL(r, h.c.WebPublicAddr, addr.Host(), p)
	http.Redirect(w, r, urlString, http.StatusFound)
	return nil
}

// DELETE IN 17.0 along with blocks of code that uses it.
// Kept for legacy app access.
func (h *Handler) withCustomCORS(handle routerFunc) routerFunc {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {

		// There can be two types of POST app launcher request.
		//  1): legacy app access
		//  2): new app access
		// Legacy app access will send a POST request with an empty body.
		if r.Method == http.MethodPost && r.Body != http.NoBody {
			body, err := utils.ReadAtMost(r.Body, teleport.MaxHTTPRequestSize)
			if err != nil {
				return trace.Wrap(err)
			}

			var req fragmentRequest
			if err := json.Unmarshal(body, &req); err != nil {
				h.log.Warn("Failed to decode JSON from request body")
				return trace.AccessDenied("access denied")
			}
			// Replace the body with a new reader, allows re-reading the body.
			// (the handler `completeAppAuthExchange` will also read the body)
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			if req.CookieValue != "" && req.StateValue != "" && req.SubjectCookieValue != "" {
				return h.completeAppAuthExchange(w, r, p)
			}

			h.log.Warn("Missing fields from parsed JSON request body")
			h.emitErrorEventAndDeleteAppSession(r, emitErrorEventFields{
				sessionID: req.CookieValue,
				err:       "missing required fields in JSON request body",
			})
			return trace.AccessDenied("access denied")
		}

		// Allow minimal CORS from only the proxy origin
		// This allows for requests from the proxy to `POST` to `/x-teleport-auth` and only
		// permits the headers `X-Cookie-Value` and `X-Subject-Cookie-Value`.
		// This is for the web UI to post a request to the application to get the proper app session
		// cookie set on the right application subdomain.
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "X-Cookie-Value, X-Subject-Cookie-Value")

		// Validate that the origin for the request matches any of the public proxy addresses.
		// This is instead of protecting via CORS headers, as that only supports a single domain.
		originValue := r.Header.Get("Origin")
		origin, err := url.Parse(originValue)
		if err != nil {
			return trace.BadParameter("malformed Origin header: %v", err)
		}

		var match bool
		originPort := origin.Port()
		if originPort == "" {
			originPort = "443"
		}

		for _, addr := range h.c.ProxyPublicAddrs {
			if strconv.Itoa(addr.Port(0)) == originPort && addr.Host() == origin.Hostname() {
				match = true
				break
			}
		}

		if !match {
			return trace.AccessDenied("port or hostname did not match")
		}

		// As we've already checked the origin matches a public proxy address, we can allow requests from that origin
		// We do this dynamically as this header can only contain one value
		w.Header().Set("Access-Control-Allow-Origin", originValue)
		if handle != nil {
			return handle(w, r, p)
		}

		return nil
	}
}

// makeRouterHandler creates a httprouter.Handle.
func makeRouterHandler(handler routerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := handler(w, r, p); err != nil {
			writeError(w, err)
			return
		}
	}
}

// makeHandler creates a http.HandlerFunc.
func makeHandler(handler handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			writeError(w, err)
			return
		}
	}
}

// writeError gets the HTTP status code from trace and writes the error to the
// response writer.
func writeError(w http.ResponseWriter, err error) {
	code := trace.ErrorToCode(err)
	http.Error(w, http.StatusText(code), code)
}

type routerFunc func(http.ResponseWriter, *http.Request, httprouter.Params) error
type routerAuthFunc func(http.ResponseWriter, *http.Request, httprouter.Params, *session) error

type handlerAuthFunc func(http.ResponseWriter, *http.Request, *session) error
type handlerFunc func(http.ResponseWriter, *http.Request) error

type launcherURLParams struct {
	// clusterName is the cluster within which this application is running.
	clusterName string
	// publicAddr is the public address of this application.
	publicAddr string
	// arn is the AWS role name, defined only when accessing AWS management console.
	arn string
	// stateToken if defined means initiating an app access auth exchange.
	stateToken string
	// path is the application URL path.
	// It is only defined if an application was accessed without the web launcher
	// (e.g: clicking on a bookmarked URL).
	// This field is used to preserve the original requested path through
	// the app access authentication redirections.
	path string
}
