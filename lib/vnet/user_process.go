// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
)

// ClientApplication is the common interface implemented by each VNet client
// application: Connect and tsh. It provides methods to list user profiles, get
// cluster clients, issue app certificates, and report metrics and errors -
// anything that uses the user's credentials or a Teleport client.
// The name "client application" refers to a user-facing client application, in
// contrast to the MacOS daemon or Windows service.
type ClientApplication interface {
	// ListProfiles lists the names of all profiles saved for the user.
	ListProfiles() ([]string, error)

	// GetCachedClient returns a [*client.ClusterClient] for the given profile and leaf cluster.
	// [leafClusterName] may be empty when requesting a client for the root cluster. Returned clients are
	// expected to be cached, as this may be called frequently.
	GetCachedClient(ctx context.Context, profileName, leafClusterName string) (ClusterClient, error)

	// ReissueAppCert issues a new cert for the target app.
	ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error)

	// UserTLSCert returns the user TLS certificate for the given profile.
	UserTLSCert(ctx context.Context, profileName string) (tls.Certificate, error)

	// GetDialOptions returns ALPN dial options for the profile.
	GetDialOptions(ctx context.Context, profileName string) (*vnetv1.DialOptions, error)

	// OnNewConnection gets called whenever a new connection is about to be established through VNet.
	// By the time OnNewConnection, VNet has already verified that the user holds a valid cert for the
	// app.
	//
	// The connection won't be established until OnNewConnection returns. Returning an error prevents
	// the connection from being made.
	OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error

	// OnInvalidLocalPort gets called before VNet refuses to handle a connection to a multi-port TCP app
	// because the provided port does not match any of the TCP ports in the app spec.
	OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16)
}

// ClusterClient is an interface defining the subset of [client.ClusterClient]
// methods used by via [ClientApplication].
type ClusterClient interface {
	CurrentCluster() authclient.ClientI
	ClusterName() string
	RootClusterName() string
	SessionSSHKeyRing(ctx context.Context, user string, target client.NodeDetails) (keyRing *client.KeyRing, completedMFA bool, err error)
}

// RunUserProcess is the entry point called by all VNet client applications
// (tsh, Connect) to start and run all VNet tasks.
//
// ctx is used for setup steps that happen before RunUserProcess passes control
// to the process manager. Canceling ctx after RunUserProcess returns will _not_
// cancel the background tasks.
//
// If [RunUserProcess] returns without error the caller is expected to
// eventually call Close on the UserProcess to clean up any resources, terminate
// all child processes, and remove any OS configuration used for actively
// running VNet. Callers should also call Wait to get notified if the process
// terminates early due to an error.
func RunUserProcess(ctx context.Context, clientApplication ClientApplication) (*UserProcess, error) {
	clock := clockwork.NewRealClock()
	clusterConfigCache := NewClusterConfigCache(clock)
	leafClusterCache, err := newLeafClusterCache(clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fqdnResolver := newFQDNResolver(&fqdnResolverConfig{
		clientApplication:  clientApplication,
		clusterConfigCache: clusterConfigCache,
		leafClusterCache:   leafClusterCache,
	})
	osConfigProvider := NewLocalOSConfigProvider(&LocalOSConfigProviderConfig{
		clientApplication:  clientApplication,
		clusterConfigCache: clusterConfigCache,
		leafClusterCache:   leafClusterCache,
	})
	clientApplicationService, err := newClientApplicationService(&clientApplicationServiceConfig{
		clientApplication:     clientApplication,
		fqdnResolver:          fqdnResolver,
		localOSConfigProvider: osConfigProvider,
		clock:                 clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	processManager, processCtx := newProcessManager()
	sshConfigurator := newSSHConfigurator(sshConfiguratorConfig{
		clientApplication: clientApplication,
	})
	processManager.AddCriticalBackgroundTask("SSH configuration loop", func() error {
		return trace.Wrap(sshConfigurator.runConfigurationLoop(processCtx))
	})

	userProcess := &UserProcess{
		clientApplication:        clientApplication,
		osConfigProvider:         osConfigProvider,
		clientApplicationService: clientApplicationService,
		clock:                    clock,
		processManager:           processManager,
	}
	if err := userProcess.runPlatformUserProcess(processCtx); err != nil {
		return nil, trace.Wrap(err)
	}
	return userProcess, nil
}

// UserProcess holds the state of the VNet user process, see the comment on
// [RunUserProcess] for usage details.
type UserProcess struct {
	clientApplication ClientApplication

	clock                    clockwork.Clock
	osConfigProvider         *LocalOSConfigProvider
	clientApplicationService *clientApplicationService

	processManager   *ProcessManager
	networkStackInfo *vnetv1.NetworkStackInfo
}

func (p *UserProcess) Close() {
	p.processManager.Close()
}

func (p *UserProcess) Wait() error {
	return p.processManager.Wait()
}

func (p *UserProcess) NetworkStackInfo() *vnetv1.NetworkStackInfo {
	return p.networkStackInfo
}

// GetTargetOSConfiguration returns the LocalOSConfigProvider which clients may
// use to report the proxied DNS zones, run diagnostics, etc. The returned
// *LocalOSConfigProvider will remain valid even if the UserProcess is closed.
func (p *UserProcess) GetOSConfigProvider() *LocalOSConfigProvider {
	return p.osConfigProvider
}
