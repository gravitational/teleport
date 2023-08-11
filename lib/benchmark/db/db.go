// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
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
	}, nil)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	rawCert, ok := key.DBTLSCerts[db.GetName()]
	if !ok {
		return tls.Certificate{}, trace.AccessDenied("failed to retrieve database certificates")
	}

	tlsCert, err := key.TLSCertificate(rawCert)
	return tlsCert, trace.Wrap(err)
}

// getDatabase loads the database which the name matches.
func getDatabase(ctx context.Context, tc *client.TeleportClient, serviceName string, protocol string) (types.Database, error) {
	databases, err := tc.ListDatabases(ctx, &proto.ListResourcesRequest{
		Namespace:           tc.Namespace,
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
		alpnproxy.WithClientCerts(dbCert),
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
