/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package agent

import (
	"log/slog"
	"net/http"
)

type ErrorCode int

const (
	ErrorCodeNoSpaceLeft ErrorCode = 1 << iota
	ErrorCodeSystemdReload
	ErrorCodeSystemdNotInstalled
)

// RegisterUpdaterHandlers registers updater handlers to a given multiplexer.
// This allows to dynamically update the binary updater status after update schedule.
func RegisterUpdaterHandlers(mux *http.ServeMux, logger *slog.Logger) {
	mux.Handle("POST /updater-status", handleUpdateStatus(logger))
}

// handleUpdateStatus returns the http update status handler.
func handleUpdateStatus(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Downstream initiate state update from teleport-update config file.
		w.Write([]byte("Status is updated"))
	}
}
