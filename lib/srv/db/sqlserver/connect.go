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

package sqlserver

import (
	"context"
	"io"
	"net"
	"strconv"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/azuread"
	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
)

const (
	// ResourceIDDSNKey represents the resource ID DSN parameter key. This value
	// is defined by the go-mssqldb library.
	ResourceIDDSNKey = "resource id"
	// FederatedAuthDSNKey represents the federated auth DSN parameter key. This
	// value is defined by the go-mssqldb library.
	FederatedAuthDSNKey = "fedauth"
)

// Connector defines an interface for connecting to a SQL Server so it can be
// swapped out in tests.
type Connector interface {
	Connect(context.Context, *common.Session, *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error)
}

type connector struct {
	// Auth is the database auth client
	DBAuth common.Auth

	kerberos kerberos.ClientProvider
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

	tlsConfig, err := c.DBAuth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Pass all login options from the client to the server.
	options := msdsn.LoginOptions{
		OptionFlags1: loginPacket.OptionFlags1(),
		OptionFlags2: loginPacket.OptionFlags2(),
		TypeFlags:    loginPacket.TypeFlags(),
	}

	dsnConfig := msdsn.Config{
		Host:         host,
		Port:         portI,
		Database:     sessionCtx.DatabaseName,
		LoginOptions: options,
		Encryption:   msdsn.EncryptionRequired,
		TLSConfig:    tlsConfig,
		PacketSize:   loginPacket.PacketSize(),
		Protocols:    []string{"tcp"},
	}

	var connector *mssql.Connector
	switch {
	case sessionCtx.Database.IsAzure() && sessionCtx.Database.GetAD().Domain == "":
		// If the client is connecting to Azure SQL, and no AD configuration is
		// provided, authenticate using the Azure AD Integrated authentication
		// method.
		connector, err = c.getAzureConnector(ctx, sessionCtx, dsnConfig)
	case sessionCtx.Database.GetType() == types.DatabaseTypeRDSProxy:
		connector, err = c.getAccessTokenConnector(ctx, sessionCtx, dsnConfig)
	default:
		connector, err = c.getKerberosConnector(ctx, sessionCtx, dsnConfig)
	}
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

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

// getKerberosConnector generates a Kerberos connector using proper Kerberos
// client.
func (c *connector) getKerberosConnector(ctx context.Context, sessionCtx *common.Session, dsnConfig msdsn.Config) (*mssql.Connector, error) {
	kc, err := c.kerberos.GetKerberosClient(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbAuth, err := c.getAuth(sessionCtx, kc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return mssql.NewConnectorConfig(dsnConfig, dbAuth), nil
}

// getAzureConnector generates a connector that authenticates using Azure AD.
func (c *connector) getAzureConnector(ctx context.Context, sessionCtx *common.Session, dsnConfig msdsn.Config) (*mssql.Connector, error) {
	managedIdentityID, err := c.DBAuth.GetAzureIdentityResourceID(ctx, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dsnConfig.Parameters = map[string]string{
		FederatedAuthDSNKey: azuread.ActiveDirectoryManagedIdentity,
		ResourceIDDSNKey:    managedIdentityID,
	}

	connector, err := azuread.NewConnectorFromConfig(dsnConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connector, nil
}

// getAccessTokenConnector generates a connector that uses a token to
// authenticate.
func (c *connector) getAccessTokenConnector(ctx context.Context, sessionCtx *common.Session, dsnConfig msdsn.Config) (*mssql.Connector, error) {
	return mssql.NewSecurityTokenConnector(dsnConfig, func(ctx context.Context) (string, error) {
		return c.DBAuth.GetRDSAuthToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
	})
}
