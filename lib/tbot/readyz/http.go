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

	"github.com/gravitational/trace"
)

// ReadOnlyRegistry is a version of the Registry which can only be read from,
// not reported to.
type ReadOnlyRegistry interface {
	ServiceStatus(name string) (*ServiceStatus, bool)
	OverallStatus() *OverallStatus
	AllServicesReported() <-chan struct{}
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
			if err := writeJSONError(w, fmt.Sprintf("Service named %q not found.", r.PathValue("service"))); err != nil {
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

// HTTPWaitHandler returns an HTTP handler that implements tbot's
// `/wait(/{service})` endpoints.
func HTTPWaitHandler(reg ReadOnlyRegistry) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/wait/{service}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("service")

		w.Header().Set("Content-Type", "application/json")

		status, ok := reg.ServiceStatus(name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			if err := writeJSONError(w, fmt.Sprintf("Service named %q not found.", r.PathValue("service"))); err != nil {
				slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
			}
			return
		}

		select {
		case <-status.Wait():
		case <-r.Context().Done():
			// Client went away
			slog.WarnContext(r.Context(), "Client went away while waiting", "service", name)
			return
		}

		// Fetch the status again as the cloned initial status may be out of
		// date.
		status, ok = reg.ServiceStatus(name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			if err := writeJSONError(w, fmt.Sprintf("Service named %q not found.", r.PathValue("service"))); err != nil {
				slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
			}
			return
		}

		w.WriteHeader(status.Status.HTTPStatusCode())
		if err := writeJSON(w, status); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
		}
	}))

	mux.Handle("/wait", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-reg.AllServicesReported():
			// All services ready, return status as usual.
		case <-r.Context().Done():
			// Client went away, no need to report an error.
			return
		}

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

func writeJSONError(w io.Writer, text string) error {
	return trace.Wrap(writeJSON(w, struct {
		Error string `json:"error"`
	}{
		text,
	}))
}
