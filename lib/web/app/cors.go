/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// Watcher defines an interface for an AppServer watcher.
type Watcher interface {
	CurrentResourcesWithFilter(ctx context.Context, filter func(readonly.AppServer) bool) ([]types.AppServer, error)
}

// HandlePreflight responds with CORS headers derived from the target app's
// CORS policy.
func (h *Handler) HandlePreflight(w http.ResponseWriter, r *http.Request) {
	raddr, err := utils.ParseAddr(r.Host)
	if err != nil {
		return
	}
	publicAddr := raddr.Host()

	servers, err := h.c.AppServerWatcher.CurrentResourcesWithFilter(r.Context(), MatchPublicAddr(publicAddr))
	if err != nil {
		h.logger.InfoContext(r.Context(), "failed to match application with public addr", "public_addr", publicAddr)
		return
	}
	if len(servers) == 0 {
		h.logger.InfoContext(r.Context(), "failed to match application with public addr", "public_addr", publicAddr)
		return
	}

	foundApp := servers[rand.N(len(servers))].GetApp()
	corsPolicy := foundApp.GetCORS()
	if corsPolicy == nil {
		return
	}

	origin := r.Header.Get("Origin")
	// The Access-Control-Allow-Origin can only include one origin or a wildcard. However,
	// any request which includes credentials _must_ return an origin and not a wildcard.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#sect2
	if slices.Contains(corsPolicy.AllowedOrigins, "*") || slices.Contains(corsPolicy.AllowedOrigins, origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		return
	}

	if len(corsPolicy.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(corsPolicy.AllowedMethods, ","))
	}

	// This is a list of headers that are allowed in the spec. Wildcards are allowed.
	// Note: "Authorization" headers must be explicitly listed and cannot be wildcarded
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers#sect2
	if len(corsPolicy.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(corsPolicy.AllowedHeaders, ","))
	}

	if len(corsPolicy.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(corsPolicy.ExposedHeaders, ","))
	}

	// The only valid value for this header is "true", so we will only set it if configured to true
	if corsPolicy.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// This will allow preflight responses to be cached for the specified duration
	if corsPolicy.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsPolicy.MaxAge))
	}

	w.WriteHeader(http.StatusOK)
}
