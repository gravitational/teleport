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
			log.WithError(err).WithField("requestID", c.RequestID).Warn("failed to write SSO diag info data")
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
