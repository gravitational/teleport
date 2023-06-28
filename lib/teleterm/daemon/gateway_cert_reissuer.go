// Copyright 2022 Gravitational, Inc
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

package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GatewayCertReissuer is responsible for managing the process of reissuing a db cert for a gateway
// after the db cert expires.
type GatewayCertReissuer struct {
	// reloginMu is used when a goroutine needs to request a relogin from the Electron app. Since the
	// app can show only one login modal at a time, we need to submit only one request at a time.
	reloginMu sync.Mutex
	Log       *logrus.Entry
}

// DBCertReissuer lets us pass a mock in tests and clusters.Cluster (which makes calls to the
// cluster) in production code.
type DBCertReissuer interface {
	// ReissueDBCerts reaches out to the cluster to get a cert for the specific tlsca.RouteToDatabase
	// and saves it to disk.
	ReissueDBCerts(context.Context, tlsca.RouteToDatabase) error
}

// ReissueCert attempts to contact the cluster to reissue the db cert used by the gateway. If that
// operation fails and the error is resolvable by relogin, ReissueCert tells the Electron app to
// relogin the user. Once that is done, it attempts to reissue the db cert again.
//
// ReissueCert is called by the LocalProxy middleware used by Connect's gateways. The middleware
// calls ReissueCert on an incoming connection to the proxy if the db cert used by the proxy has
// expired.
//
// If the initial call to the cluster fails with an error that is not resolvable by logging in,
// ReissueCert returns with that error.
//
// Any error ReissueCert returns is also forwarded to the Electron app so that it can show an error
// notification. GatewayCertReissuer is typically called from within a goroutine that handles the
// gateway, so without forwarding the error to the app, it would be visible only in the logs.
func (r *GatewayCertReissuer) ReissueCert(ctx context.Context, gateway *gateway.Gateway, dbCertReissuer DBCertReissuer, tshdEventsClient TSHDEventsClient) error {
	if err := r.reissueCert(ctx, gateway, dbCertReissuer, tshdEventsClient); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *GatewayCertReissuer) reissueCert(ctx context.Context, gateway *gateway.Gateway, dbCertReissuer DBCertReissuer, tshdEventsClient TSHDEventsClient) error {
	// Make the first attempt at reissuing the db cert.
	//
	// It might happen that the db cert has expired but the user cert is still active, allowing us to
	// obtain a new db cert without having to relogin first.
	//
	// This can happen if the user cert was refreshed by anything other than the gateway itself. For
	// example, if you execute `tsh ssh` within Connect after your user cert expires or there are two
	// gateways that subsequently go through this flow.
	err := r.reissueAndReloadGatewayCert(ctx, gateway, dbCertReissuer)

	if err == nil {
		return nil
	}

	// Do not ask for relogin if the error cannot be resolved with relogin.
	if !client.IsErrorResolvableWithRelogin(err) {
		return trace.Wrap(err)
	}

	clusterURI, err := uri.ParseClusterURI(gateway.TargetURI())
	if err != nil {
		return trace.Wrap(err)
	}
	rootClusterURI := clusterURI.GetRootClusterURI().String()

	err = r.requestReloginFromElectronApp(ctx,
		&api.ReloginRequest{
			RootClusterUri: rootClusterURI,
			Reason: &api.ReloginRequest_GatewayCertExpired{
				GatewayCertExpired: &api.GatewayCertExpired{
					GatewayUri: gateway.URI().String(),
					TargetUri:  gateway.TargetURI(),
				},
			},
		}, tshdEventsClient)
	if err != nil {
		return trace.Wrap(err)
	}

	err = r.reissueAndReloadGatewayCert(ctx, gateway, dbCertReissuer)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *GatewayCertReissuer) reissueAndReloadGatewayCert(ctx context.Context, gateway *gateway.Gateway, dbCertReissuer DBCertReissuer) error {
	err := dbCertReissuer.ReissueDBCerts(ctx, gateway.RouteToDatabase())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(gateway.ReloadCert())
}

func (r *GatewayCertReissuer) requestReloginFromElectronApp(ctx context.Context, req *api.ReloginRequest, tshdEventsClient TSHDEventsClient) error {
	const reloginUserTimeout = time.Minute
	timeoutCtx, cancelTshdEventsCtx := context.WithTimeout(ctx, reloginUserTimeout)
	defer cancelTshdEventsCtx()

	// The Electron app cannot display two login modals at the same time, so we have to cut short any
	// concurrent relogin requests.
	if !r.reloginMu.TryLock() {
		return trace.AlreadyExists("another relogin request is in progress")
	}
	defer r.reloginMu.Unlock()

	_, err := tshdEventsClient.Relogin(timeoutCtx, req)

	if err != nil {
		if status.Code(err) == codes.DeadlineExceeded {
			return trace.Wrap(err, "the user did not refresh the session within %s", reloginUserTimeout.String())
		}

		return trace.Wrap(err, "could not refresh the session")
	}

	return nil
}
