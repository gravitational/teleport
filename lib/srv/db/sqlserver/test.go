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
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/gravitational/trace"
)

// MakeTestClient returns SQL Server client used in tests.
func MakeTestClient(ctx context.Context, config common.TestClientConfig) (*mssql.Conn, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	portI, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector := mssql.NewConnectorConfig(msdsn.Config{
		Host:       host,
		Port:       portI,
		User:       config.RouteToDatabase.Username,
		Database:   config.RouteToDatabase.Database,
		Encryption: msdsn.EncryptionDisabled,
	}, nil)

	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mssqlConn, ok := conn.(*mssql.Conn)
	if !ok {
		return nil, trace.BadParameter("expected *mssql.Conn, got: %T", conn)
	}

	return mssqlConn, nil
}

// TestConnector is used in tests to mock connections to SQL Server.
type TestConnector struct{}

// Connect simulates successful connection to a SQL Server.
func (c *TestConnector) Connect(ctx context.Context, sessionCtx *common.Session, loginPacket *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error) {
	return &fakeConn{}, []mssql.Token{
		mssql.LoginAckToken(),
		mssql.DoneToken(),
	}, nil
}

type fakeConn struct {
	m sync.Mutex
	b bytes.Buffer
}

func (b *fakeConn) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *fakeConn) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *fakeConn) Close() error {
	return nil
}
