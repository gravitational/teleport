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
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

// AppRequestContext contains common context parameters for an App request.
type AppRequestContext struct {
	// Identity is the requested identity.
	Identity *tlsca.Identity
	// App is the requested identity.
	App types.Application
	// Emitter is the audit log emitter
	Emitter events.Emitter
}

// WithAppRequestContext adds provided app request context.
func WithAppRequestContext(r *http.Request, requestCtx *AppRequestContext) *http.Request {
	return r.WithContext(context.WithValue(
		r.Context(),
		contextKeyAppRequstContext,
		requestCtx,
	))
}

// GetAppRequestContext retrieves the App request context.
func GetAppRequestContext(r *http.Request) (*AppRequestContext, error) {
	requestCtxValue := r.Context().Value(contextKeyAppRequstContext)
	requestCtx, ok := requestCtxValue.(*AppRequestContext)
	if !ok {
		return nil, trace.BadParameter("failed to get app request context")
	}
	return requestCtx, nil
}

type contextKey string

const (
	// contextKeyAppRequstContext is the context key for the App request context.
	contextKeyAppRequstContext contextKey = "app-request-context"
)
