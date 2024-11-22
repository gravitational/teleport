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

package conntest

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/conntest/database"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
)

// databasePinger describes the required methods to test a Database Connection.
type databasePinger interface {
	// Ping tests the connection to the Database with a simple request.
	Ping(ctx context.Context, params database.PingParams) error

	// IsConnectionRefusedError returns whether the error is referring to a connection refused.
	IsConnectionRefusedError(error) bool

	// IsInvalidDatabaseUserError returns whether the error is referring to an invalid (non-existent) user.
	IsInvalidDatabaseUserError(error) bool

	// IsInvalidDatabaseNameError returns whether the error is referring to an invalid (non-existent) database name.
	IsInvalidDatabaseNameError(error) bool
}

// ClientDatabaseConnectionTester contains the required auth.ClientI methods to test a Database Connection
type ClientDatabaseConnectionTester interface {
	client.ALPNAuthClient

	services.ConnectionsDiagnostic
	apiclient.GetResourcesClient
}

// DatabaseConnectionTesterConfig defines the config fields for DatabaseConnectionTester.
type DatabaseConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	UserClient ClientDatabaseConnectionTester

	// PublicProxyAddr is public address of the proxy
	PublicProxyAddr string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool
}

// DatabaseConnectionTester implements the ConnectionTester interface for Testing Database access.
type DatabaseConnectionTester struct {
	cfg DatabaseConnectionTesterConfig
}

// NewDatabaseConnectionTester returns a new DatabaseConnectionTester
func NewDatabaseConnectionTester(cfg DatabaseConnectionTesterConfig) (*DatabaseConnectionTester, error) {
	return &DatabaseConnectionTester{
		cfg: cfg,
	}, nil
}

// TestConnection tests the access to a database using:
// - auth Client using the User access
// - the resource name
// - database user and database name to connect to
//
// A new ConnectionDiagnostic is created and used to store the traces as it goes through the checkpoints
// To connect to the Database, we will create a cert-key pair and setup a Database client back to Teleport Proxy.
// The following checkpoints are reported:
// - database server for the requested database exists / the user's roles can access it
// - the user can use the requested database user and database name (per their roles)
// - the database is acessible and accepting connections from the database server
// - the database has the database user and database name that was requested
func (s *DatabaseConnectionTester) TestConnection(ctx context.Context, req TestConnectionRequest) (types.ConnectionDiagnostic, error) {
	if req.ResourceKind != types.KindDatabase {
		return nil, trace.BadParameter("invalid value for ResourceKind, expected %q got %q", types.KindDatabase, req.ResourceKind)
	}

	connectionDiagnosticID := uuid.NewString()
	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(
		connectionDiagnosticID,
		map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			// We start with a failed state so that we don't need to set it to each return statement once an error is returned.
			// if the test reaches the end, we force the test to be a success by calling
			// 	connectionDiagnostic.SetMessage(types.DiagnosticMessageSuccess)
			//	connectionDiagnostic.SetSuccess(true)
			Message: types.DiagnosticMessageFailed,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.UserClient.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	databaseServers, err := s.getDatabaseServers(ctx, req.ResourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(databaseServers) == 0 {
		connDiag, err := s.appendDiagnosticTrace(ctx,
			connectionDiagnosticID,
			types.ConnectionDiagnosticTrace_RBAC_DATABASE,
			"Database not found. "+
				"Ensure your role grants access by adding it to the 'db_labels' property. "+
				"This can also happen when you don't have a Teleport Database Service proxying the database - "+
				"you can fix that by adding the database labels to the 'db_service.resources.labels' in 'teleport.yaml' file of the Database Service.",
			trace.NotFound("%s not found", req.ResourceName),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	databaseServer := databaseServers[0]
	routeToDatabase := proto.RouteToDatabase{
		ServiceName: databaseServer.GetName(),
		Protocol:    databaseServer.GetDatabase().GetProtocol(),
		Username:    req.DatabaseUser,
		Database:    req.DatabaseName,
	}

	databasePinger, err := getDatabaseConnTester(routeToDatabase.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkDatabaseLogin(routeToDatabase.Protocol, req.DatabaseUser, req.DatabaseName); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := s.appendDiagnosticTrace(ctx,
		connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_RBAC_DATABASE,
		"A Teleport Database Service is available to proxy the connection to the Database.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := s.runALPNTunnel(ctx, req, routeToDatabase, connectionDiagnosticID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer listener.Close()

	ping, err := newPing(listener.Addr().String(), req.DatabaseUser, req.DatabaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
	defer cancel()
	if pingErr := databasePinger.Ping(pingCtx, ping); pingErr != nil {
		connDiag, err := s.handlePingError(ctx, connectionDiagnosticID, pingErr, databasePinger)
		return connDiag, trace.Wrap(err)
	}

	return s.handlePingSuccess(ctx, connectionDiagnosticID)
}

func (s *DatabaseConnectionTester) runALPNTunnel(ctx context.Context, req TestConnectionRequest, routeToDatabase proto.RouteToDatabase, connectionDiagnosticID string) (net.Listener, error) {
	list, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	alpnProtocol, err := alpn.ToALPNProtocol(routeToDatabase.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResponse, err := req.MFAResponse.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = client.RunALPNAuthTunnel(ctx, client.ALPNAuthTunnelConfig{
		AuthClient:             s.cfg.UserClient,
		Listener:               list,
		Protocol:               alpnProtocol,
		Expires:                time.Now().Add(time.Minute).UTC(),
		PublicProxyAddr:        s.cfg.PublicProxyAddr,
		ConnectionDiagnosticID: connectionDiagnosticID,
		RouteToDatabase:        routeToDatabase,
		InsecureSkipVerify:     req.InsecureSkipVerify,
		MFAResponse:            mfaResponse,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return list, nil
}

func (s *DatabaseConnectionTester) getDatabaseServers(ctx context.Context, databaseName string) ([]types.DatabaseServer, error) {
	// Lookup the Database Server that's proxying the requested Database.
	page, err := apiclient.GetResourcePage[types.DatabaseServer](ctx, s.cfg.UserClient, &proto.ListResourcesRequest{
		PredicateExpression: fmt.Sprintf(`name == "%s"`, databaseName),
		ResourceType:        types.KindDatabaseServer,
		Limit:               defaults.MaxIterationLimit,
	})
	return page.Resources, trace.Wrap(err)
}

func checkDatabaseLogin(protocol, databaseUser, databaseName string) error {
	needUser := role.RequireDatabaseUserMatcher(protocol)
	needDatabase := role.RequireDatabaseNameMatcher(protocol)

	if needUser && databaseUser == "" {
		return trace.BadParameter("missing required parameter Database User")
	}

	if needDatabase && databaseName == "" {
		return trace.BadParameter("missing required parameter Database Name")
	}

	return nil
}

func newPing(alpnProxyAddr, databaseUser, databaseName string) (database.PingParams, error) {
	proxyHost, proxyPortStr, err := net.SplitHostPort(alpnProxyAddr)
	if err != nil {
		return database.PingParams{}, trace.Wrap(err)
	}

	proxyPort, err := strconv.Atoi(proxyPortStr)
	if err != nil {
		return database.PingParams{}, trace.Wrap(err)
	}

	return database.PingParams{
		Host:         proxyHost,
		Port:         proxyPort,
		Username:     databaseUser,
		DatabaseName: databaseName,
	}, nil
}

func (s DatabaseConnectionTester) handlePingSuccess(ctx context.Context, connectionDiagnosticID string) (types.ConnectionDiagnostic, error) {
	if _, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_CONNECTIVITY,
		"Database is accessible from the Teleport Database Service.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_RBAC_DATABASE_LOGIN,
		"Access to Database User and Database Name granted.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_DATABASE_DB_USER,
		"Database User exists in the Database.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_DATABASE_DB_NAME,
		"Database Name exists in the Database.",
		nil,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.cfg.UserClient.UpdateConnectionDiagnostic(ctx, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func errorFromDatabaseService(pingErr error) bool {
	// If the requested DB User/Name can't be used per RBAC checks, the Database Agent returns an error which gets here.
	if strings.Contains(pingErr.Error(), "access to db denied. User does not have permissions. Confirm database user and name.") {
		return true
	}

	// When there's an error when trying to use RDS IAM auth.
	if strings.Contains(pingErr.Error(), "FATAL: PAM authentication failed for user") &&
		strings.Contains(pingErr.Error(), "IAM policy") {
		return true
	}

	return false
}

func (s DatabaseConnectionTester) handlePingError(ctx context.Context, connectionDiagnosticID string, pingErr error, databasePinger databasePinger) (types.ConnectionDiagnostic, error) {
	// The Database Agent (lib/srv/db/server.go) might add an trace in some cases.
	// Here, it must be ignored to prevent multiple failed traces.
	if errorFromDatabaseService(pingErr) {
		connDiag, err := s.cfg.UserClient.GetConnectionDiagnostic(ctx, connectionDiagnosticID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	if databasePinger.IsConnectionRefusedError(pingErr) || strings.Contains(pingErr.Error(), "context deadline exceeded") {
		connDiag, err := s.appendDiagnosticTrace(ctx,
			connectionDiagnosticID,
			types.ConnectionDiagnosticTrace_CONNECTIVITY,
			"There was a connection problem between the Teleport Database Service and the database. "+
				"Ensure the database is running and accessible from the Database Service over the network.",
			pingErr,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	// Requested DB User is allowed per RBAC rules, but those entities don't exist in the Database itself.
	if databasePinger.IsInvalidDatabaseUserError(pingErr) {
		connDiag, err := s.appendDiagnosticTrace(ctx,
			connectionDiagnosticID,
			types.ConnectionDiagnosticTrace_DATABASE_DB_USER,
			"The Database rejected the provided Database User. Ensure that the database user exists.",
			pingErr,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	// Requested DB Name is allowed per RBAC rules, but those entities don't exist in the Database itself.
	if databasePinger.IsInvalidDatabaseNameError(pingErr) {
		connDiag, err := s.appendDiagnosticTrace(ctx,
			connectionDiagnosticID,
			types.ConnectionDiagnosticTrace_DATABASE_DB_NAME,
			"The Database rejected the provided Database Name. Ensure that the database name exists.",
			pingErr,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	connDiag, err := s.appendDiagnosticTrace(ctx,
		connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_UNKNOWN_ERROR,
		fmt.Sprintf("Unknown error. %v", pingErr),
		pingErr,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s DatabaseConnectionTester) appendDiagnosticTrace(ctx context.Context, connectionDiagnosticID string, traceType types.ConnectionDiagnosticTrace_TraceType, message string, err error) (types.ConnectionDiagnostic, error) {
	connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(
		ctx,
		connectionDiagnosticID,
		types.NewTraceDiagnosticConnection(
			traceType,
			message,
			err,
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func getDatabaseConnTester(protocol string) (databasePinger, error) {
	switch protocol {
	case defaults.ProtocolPostgres:
		return &database.PostgresPinger{}, nil
	case defaults.ProtocolMySQL:
		return &database.MySQLPinger{}, nil
	case defaults.ProtocolSQLServer:
		return &database.SQLServerPinger{}, nil
	}
	return nil, trace.NotImplemented("database protocol %q is not supported yet for testing connection", protocol)
}
