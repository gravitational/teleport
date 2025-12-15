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

package readyz

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// ReadOnlyRegistry is a version of the Registry which can only be read from,
// not reported to.
type ReadOnlyRegistry interface {
	ServiceStatus(name string) (*ServiceStatus, bool)
	OverallStatus() *OverallStatus
}

// HTTPHandler returns an HTTP handler that implements tbot's
// /readyz(/{service}) endpoints.
func HTTPHandler(reg ReadOnlyRegistry) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/readyz/{service}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status, ok := reg.ServiceStatus(r.PathValue("service"))
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			if err := writeJSON(w, struct {
				Error string `json:"error"`
			}{
				fmt.Sprintf("Service named %q not found.", r.PathValue("service")),
			}); err != nil {
				slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
			}
			return
		}

		w.WriteHeader(status.Status.HTTPStatusCode())
		if err := writeJSON(w, status); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
		}
	}))

	mux.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := reg.OverallStatus()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status.Status.HTTPStatusCode())

		if err := writeJSON(w, status); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
		}
	}))

	return mux
}

func writeJSON(w io.Writer, v any) error {
	output, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(output); err != nil {
		return err
	}
	return nil
}
