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

package sqlserver

import (
	"context"
	"io"
	"net"
	"strconv"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
)

// Connector defines an interface for connecting to a SQL Server so it can be
// swapped out in tests.
type Connector interface {
	Connect(context.Context, *common.Session, *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error)
}

type connector struct {
	Auth common.Auth
}

// Connect connects to the target SQL Server with Kerberos authentication.
func (c *connector) Connect(ctx context.Context, sessionCtx *common.Session, loginPacket *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error) {
	host, port, err := net.SplitHostPort(sessionCtx.Database.GetURI())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	portI, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsConfig, err := c.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Pass all login options from the client to the server.
	options := msdsn.LoginOptions{
		OptionFlags1: loginPacket.OptionFlags1(),
		OptionFlags2: loginPacket.OptionFlags2(),
		TypeFlags:    loginPacket.TypeFlags(),
	}

	auth, err := c.getAuth(sessionCtx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	connector := mssql.NewConnectorConfig(msdsn.Config{
		Host:         host,
		Port:         portI,
		Database:     sessionCtx.DatabaseName,
		LoginOptions: options,
		Encryption:   msdsn.EncryptionRequired,
		TLSConfig:    tlsConfig,
		PacketSize:   loginPacket.PacketSize(),
	}, auth)

	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	mssqlConn, ok := conn.(*mssql.Conn)
	if !ok {
		return nil, nil, trace.BadParameter("expected *mssql.Conn, got: %T", conn)
	}

	// Return all login flags returned by the server so that they can be passed
	// back to the client.
	return mssqlConn.GetUnderlyingConn(), mssqlConn.GetLoginFlags(), nil
}
