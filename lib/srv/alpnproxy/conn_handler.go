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

package alpnproxy

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

// ConnectionHandler defines a function for serving incoming connections.
type ConnectionHandler func(ctx context.Context, conn net.Conn) error

// ConnectionHandlerWrapper is a wrapper of ConnectionHandler. This wrapper is
// mainly used as a placeholder to resolve circular dependencies. Therefore, it
// is important to use pointer receivers when defining its functions to make
// sure the updated ConnectionHandler reference is used.
type ConnectionHandlerWrapper struct {
	h ConnectionHandler
}

// Set updates inner ConnectionHandler to use.
func (w *ConnectionHandlerWrapper) Set(h ConnectionHandler) {
	w.h = h
}

// HandleConnection implements ConnectionHandler.
func (w *ConnectionHandlerWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	if w.h == nil {
		return trace.NotFound("missing ConnectionHandler")
	}
	return w.h(ctx, conn)
}
