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

package db

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// retrieveDatabaseCertificates issues user database certificates. Same flow as
// `tsh db login`.
func retrieveDatabaseCertificates(ctx context.Context, tc *client.TeleportClient, db types.Database, dbUser, dbName string) (tls.Certificate, error) {
	profile, err := tc.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	key, err := tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
			Username:    dbUser,
			Database:    dbName,
		},
		AccessRequests: profile.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := key.DBTLSCert(db.GetName())
	return dbCert, trace.Wrap(err)
}

// getDatabase loads the database which the name matches.
func getDatabase(ctx context.Context, tc *client.TeleportClient, serviceName string, protocol string) (types.Database, error) {
	databases, err := tc.ListDatabases(ctx, &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s" && resource.spec.protocol == "%s"`, serviceName, protocol),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(databases) != 1 {
		return nil, trace.NotFound("no database with name %q found", serviceName)
	}

	return databases[0], nil
}

// startLocalProxy starts a local proxy (tunneled) which will be used to
// establish a new database connection.
func startLocalProxy(ctx context.Context, insecureSkipVerify bool, tc *client.TeleportClient, dbProtocol string, dbCert tls.Certificate) (*alpnproxy.LocalProxy, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithDatabaseProtocol(dbProtocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(ctx, tc.RootClusterCACertPool),
		alpnproxy.WithClientCert(dbCert),
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         tc.WebProxyAddr,
		InsecureSkipVerify:      insecureSkipVerify,
		ParentContext:           ctx,
		Listener:                listener,
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
	}, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer listener.Close()
		_ = lp.Start(ctx)
	}()
	return lp, nil
}
