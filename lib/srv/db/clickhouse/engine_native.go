/*
Copyright 2023 Gravitational, Inc.

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

package clickhouse

import (
	"context"
	"crypto/tls"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

func (e *Engine) handleNativeConnection(ctx context.Context, sessionCtx *common.Session) error {
	u, err := url.Parse(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverConn, err := tls.Dial("tcp", u.Host, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	if err := utils.ProxyConn(ctx, e.clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) sendErrorNative(err error) {
	// TODO: Support clickhouse native wire protocol error messages.
	e.Log.Debugf("Clickhouse client connection error: %v.", err)
}
