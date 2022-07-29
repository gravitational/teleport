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

package common

import (
	"context"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

// SessionContext contains common context parameters for an App session.
type SessionContext struct {
	// Identity is the requested identity.
	Identity *tlsca.Identity
	// App is the requested identity.
	App types.Application
}

// WithSessionContext adds session context to provided request.
func WithSessionContext(r *http.Request, sessionCtx *SessionContext) *http.Request {
	return r.WithContext(context.WithValue(
		r.Context(),
		contextSessionKey,
		sessionCtx,
	))
}

// GetSessionContext retrieves the session context from a request.
func GetSessionContext(r *http.Request) (*SessionContext, error) {
	sessionCtxValue := r.Context().Value(contextSessionKey)
	sessionCtx, ok := sessionCtxValue.(*SessionContext)
	if !ok {
		return nil, trace.BadParameter("failed to get session context")
	}
	return sessionCtx, nil
}

type contextKey string

const (
	// contextSessionKey is the context key for the session context.
	contextSessionKey contextKey = "app-session-context"
)
