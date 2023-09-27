/*
Copyright 2022 Gravitational, Inc.

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
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
)

// handleAuth handles authentication for an app
// When a `POST` request comes in from a trusted proxy address, it'll set the value from the
// `X-Cookie-Value` header to the `__Host-grv_app_session` cookie.
func (h *Handler) handleAuth(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	httplib.SetNoCacheHeaders(w.Header())

	cookieValue := r.Header.Get("X-Cookie-Value")
	if cookieValue == "" {
		return trace.AccessDenied("access denied")
	}

	subjectCookieValue := r.Header.Get("X-Subject-Cookie-Value")
	if cookieValue == "" {
		return trace.BadParameter("X-Subject-Cookie-Value header missing")
	}

	// Validate that the caller is asking for a session that exists.
	ws, err := h.c.AccessPoint.GetAppSession(r.Context(), types.GetAppSessionRequest{
		SessionID: cookieValue,
	})
	if err != nil {
		h.log.WithError(err).Warn("Request failed: unable to get app session")
		return trace.AccessDenied("access denied")
	}

	if err := checkSubjectToken(subjectCookieValue, ws); err != nil {
		h.log.Warnf("Request failed: %v.", err)

		h.c.AuthClient.EmitAuditEvent(h.closeContext, &apievents.AuthAttempt{
			Metadata: apievents.Metadata{
				Type: events.AuthAttemptEvent,
				Code: events.AuthAttemptFailureCode,
			},
			UserMetadata: apievents.UserMetadata{
				Login: ws.GetUser(),
				User:  "unknown",
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				LocalAddr:  r.Host,
				RemoteAddr: r.RemoteAddr,
			},
			Status: apievents.Status{
				Success: false,
				Error:   err.Error(),
			},
		})

		return trace.AccessDenied("access denied")
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     SubjectCookieName,
		Value:    subjectCookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "__Host-grv_app_last_active",
		Value:    fmt.Sprintf("%v", h.c.Clock.Now().UnixMilli()),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	return nil
}

func checkSubjectToken(subjectCookieValue string, ws types.WebSession) error {
	if subjectCookieValue == "" {
		return trace.AccessDenied("subject session token is not set")
	}
	if subtle.ConstantTimeCompare([]byte(subjectCookieValue), []byte(ws.GetBearerToken())) != 1 {
		return trace.AccessDenied("subject session token does not match")
	}
	return nil
}
