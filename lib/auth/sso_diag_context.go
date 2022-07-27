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

// ssoDiagContext is a helper type for accumulating the SSO diagnostic info prior to writing it to the backend.
type ssoDiagContext struct {
	// authKind is auth kind such as types.KindSAML
	authKind string
	// createSSODiagnosticInfo is a callback to create the types.SSODiagnosticInfo record in the backend.
	createSSODiagnosticInfo func(ctx context.Context, authKind string, authRequestID string, info types.SSODiagnosticInfo) error
	// requestID is the ID of the auth request being processed.
	requestID string
	// info accumulates SSO diagnostic info
	info types.SSODiagnosticInfo
}

// writeToBackend saves the accumulated SSO diagnostic information to the backend.
func (c *ssoDiagContext) writeToBackend(ctx context.Context) {
	if c.info.TestFlow {
		err := c.createSSODiagnosticInfo(ctx, c.authKind, c.requestID, c.info)
		if err != nil {
			log.WithError(err).WithField("requestID", c.requestID).Warn("failed to write SSO diag info data")
		}
	}
}

// newSSODiagContext returns new ssoDiagContext referencing particular Server.
// authKind must be one of supported auth kinds (e.g. types.KindSAML).
func (a *Server) newSSODiagContext(authKind string) *ssoDiagContext {
	return &ssoDiagContext{
		authKind:                authKind,
		createSSODiagnosticInfo: a.CreateSSODiagnosticInfo,
	}
}
