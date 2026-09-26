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

package githttp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

func (h *Handler) handleGitHub(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, gitServer types.Server) error {
	// TODO(greedy52) forward request to GitHub with credential injection.
	logger.DebugContext(ctx, "Git HTTP request authorized",
		"method", r.Method,
		"host", r.Host,
		"path", r.URL.Path,
	)
	http.Error(w, "git HTTP forwarding not yet implemented", http.StatusNotImplemented)
	return nil
}
