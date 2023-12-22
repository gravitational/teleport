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
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
)

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request, p httprouter.Params, session *session) error {
	// Remove the session from the backend.
	err := h.c.AuthClient.DeleteAppSession(context.Background(), types.DeleteAppSessionRequest{
		SessionID: session.ws.GetName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// delete these cookies on logout.
	expireCookie(w, CookieName)
	expireCookie(w, SubjectCookieName)
	http.Error(w, "Logged out.", http.StatusOK)
	return nil
}

func expireCookie(w http.ResponseWriter, cookieName string) {
	// Set Max-Age to 0 to tell the browser to delete these cookies.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteLaxMode,
	})
}
