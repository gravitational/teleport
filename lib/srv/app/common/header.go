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

package common

import (
	"errors"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/app"
)

// SetTeleportAPIErrorHeader saves the provided error in X-Teleport-API-Error header of response.
func SetTeleportAPIErrorHeader(rw http.ResponseWriter, err error) {
	var debugErr trace.DebugReporter
	if !errors.As(err, &debugErr) {
		debugErr = &trace.TraceErr{Err: err}
	}
	rw.Header().Set(app.TeleportAPIErrorHeader, debugErr.DebugReport())
}
