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

package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

type fragmentRequest struct {
	CookieValue string `json:"cookie_value"`
}

// handleFragment handles fragment authentication. Returns a Javascript
// application that reads in the fragment which submits an POST request to
// the same handler which can validate and set the cookie.
func (h *Handler) handleFragment(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	switch r.Method {
	case http.MethodGet:
		nonce, err := utils.CryptoRandomHex(auth.TokenLenBytes)
		if err != nil {
			h.log.WithError(err).Debugf("Failed to generate and encode random numbers.")
			return trace.AccessDenied("access denied")
		}
		setRedirectPageHeaders(w.Header(), nonce)
		fmt.Fprintf(w, js, nonce)
	case http.MethodPost:
		httplib.SetNoCacheHeaders(w.Header())
		var req fragmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return trace.Wrap(err)
		}

		// Validate that the caller is asking for a session that exists.
		_, err := h.c.AccessPoint.GetAppSession(r.Context(), services.GetAppSessionRequest{
			SessionID: req.CookieValue,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Set the "Set-Cookie" header on the response.
		http.SetCookie(w, &http.Cookie{
			Name:     CookieName,
			Value:    req.CookieValue,
			HttpOnly: true,
			Secure:   true,
			// Set Same-Site policy for the session cookie to None in order to
			// support redirects that identity providers do during SSO auth.
			// Otherwise the session cookie won't be sent and the user will
			// get redirected to the application launcher.
			//
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie/SameSite
			SameSite: http.SameSiteNoneMode,
		})
	default:
		return trace.BadParameter("unsupported method %q", r.Method)
	}
	return nil
}
