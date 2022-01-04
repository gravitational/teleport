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

package redis

import (
	"crypto/tls"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/sirupsen/logrus"
)

type Proxy struct {
	// TLSConfig is the proxy TLS configuration.
	TLSConfig *tls.Config
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// Service is used to connect to a remote database service.
	Service common.Service
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection accepts connection from a MySQL client, authenticates
// it and proxies it to an appropriate database service.
//func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
//	// Wrap the client connection in the connection that can detect the protocol
//	// by peeking into the first few bytes. This is needed to be able to detect
//	// proxy protocol which otherwise would interfere with MySQL protocol.
//	conn := multiplexer.NewConn(clientConn)
//	server := p.makeServer(conn)
//	// If any error happens, make sure to send it back to the client, so it
//	// has a chance to close the connection from its side.
//	defer func() {
//		if r := recover(); r != nil {
//			p.Log.Warnf("Recovered in Redis proxy while handling connection from %v: %v.", clientConn.RemoteAddr(), r)
//			err = trace.BadParameter("failed to handle Redis client connection")
//		}
//		if err != nil {
//			if writeErr := server.WriteError(err); writeErr != nil {
//				p.Log.WithError(writeErr).Debugf("Failed to send error %q to MySQL client.", err)
//			}
//		}
//	}()
//	// Perform first part of the handshake, up to the point where client sends
//	// us certificate and connection upgrades to TLS.
//	tlsConn, err := p.performHandshake(conn, server)
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	ctx, err = p.Middleware.WrapContextWithUser(ctx, tlsConn)
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	serviceConn, authContext, err := p.Service.Connect(ctx, server.GetUser(), "")
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	defer serviceConn.Close()
//	// Before replying OK to the client which would make the client consider
//	// auth completed, wait for OK packet from db service indicating auth
//	// success.
//	err = p.waitForOK(server, serviceConn)
//	if err != nil {
//		return trace.Wrap(err)
//	}
//
//	// Auth has completed, the client enters command phase, start proxying
//	// all messages back-and-forth.
//	err = p.Service.Proxy(ctx, authContext, tlsConn, serviceConn)
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	return nil
//}
