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

package auth

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// SSODiagContext is a helper type for accumulating the SSO diagnostic info prior to writing it to the backend.
type SSODiagContext struct {
	// AuthKind is auth kind such as types.KindSAML
	AuthKind string
	// DiagService is the SSODiagService that will record our diagnostic info in the backend.
	DiagService SSODiagService
	// RequestID is the ID of the auth request being processed.
	RequestID string
	// Info accumulates SSO diagnostic Info
	Info types.SSODiagnosticInfo
}

// SSODiagService is a thin slice of services.Identity required by SSODiagContext
// to record the SSO diagnostic info in a store.
type SSODiagService interface {
	// CreateSSODiagnosticInfo creates new SSO diagnostic info record.
	CreateSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error
}

// SSODiagServiceFunc is an adaptor allowing a function to be used in place
// of the SSODiagService interface.
type SSODiagServiceFunc func(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error

func (f SSODiagServiceFunc) CreateSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error {
	return f(ctx, authKind, authRequestID, entry)
}

// WriteToBackend saves the accumulated SSO diagnostic information to the backend.
func (c *SSODiagContext) WriteToBackend(ctx context.Context) {
	if c.Info.TestFlow {
		err := c.DiagService.CreateSSODiagnosticInfo(ctx, c.AuthKind, c.RequestID, c.Info)
		if err != nil {
			logger.WarnContext(ctx, "failed to write SSO diag info data",
				"error", err,
				"request_id", c.RequestID,
			)
		}
	}
}

// NewSSODiagContext returns new ssoDiagContext referencing particular Server.
// authKind must be one of supported auth kinds (e.g. types.KindSAML).
func NewSSODiagContext(authKind string, diagSvc SSODiagService) *SSODiagContext {
	return &SSODiagContext{
		AuthKind:    authKind,
		DiagService: diagSvc,
	}
}
