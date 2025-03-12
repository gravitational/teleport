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

package daemon

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/services/connectmycomputer"
)

// Storage defines an interface for cluster profile storage.
type Storage interface {
	clusters.Resolver

	ListProfileNames() ([]string, error)
	ListRootClusters() ([]*clusters.Cluster, error)
	Add(ctx context.Context, webProxyAddress string) (*clusters.Cluster, *client.TeleportClient, error)
	Remove(ctx context.Context, profileName string) error
	GetByResourceURI(resourceURI uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error)
	CurrentClusterURI() (uri.ResourceURI, error)
}

// Config is the cluster service config
type Config struct {
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// Storage is a storage service that reads/writes to tsh profiles
	Storage Storage
	// Logger is a component logger
	Logger *slog.Logger
	// PrehogAddr is the URL where prehog events should be submitted.
	PrehogAddr string
	// KubeconfigsDir is the directory containing kubeconfigs for Kubernetes
	// Acesss.
	KubeconfigsDir string
	// AgentsDir contains agent config files and data directories for Connect My Computer.
	AgentsDir string

	GatewayCreator GatewayCreator
	// CreateTshdEventsClientCredsFunc lazily creates creds for the tshd events server ran by the
	// Electron app. This is to ensure that the server public key is written to the disk under the
	// expected location by the time we get around to creating the client.
	CreateTshdEventsClientCredsFunc CreateTshdEventsClientCredsFunc

	ConnectMyComputerRoleSetup        *connectmycomputer.RoleSetup
	ConnectMyComputerTokenProvisioner *connectmycomputer.TokenProvisioner
	ConnectMyComputerNodeJoinWait     *connectmycomputer.NodeJoinWait
	ConnectMyComputerNodeDelete       *connectmycomputer.NodeDelete
	ConnectMyComputerNodeName         *connectmycomputer.NodeName

	CreateClientCacheFunc func(resolver clientcache.NewClientFunc) (ClientCache, error)
	// ClusterIDCache gets updated whenever daemon.Service.ResolveClusterWithDetails gets called.
	// Since that method is called by the Electron app only for root clusters and typically only once
	// after a successful login, this cache doesn't have to be cleared.
	ClusterIDCache *clusteridcache.Cache
}

// ResolveClusterFunc returns a cluster by URI.
type ResolveClusterFunc func(uri uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error)

// ClientCache stores clients keyed by cluster URI.
type ClientCache interface {
	// Get returns a client from the cache if there is one,
	// otherwise it dials the remote server.
	// The caller should not close the returned client.
	Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error)
	// ClearForRoot closes and removes clients from the cache
	// for the root cluster and its leaf clusters.
	ClearForRoot(profileName string) error
	// Clear closes and removes all clients.
	Clear() error
}

type CreateTshdEventsClientCredsFunc func() (grpc.DialOption, error)

// CheckAndSetDefaults checks the configuration for its validity and sets default values if needed
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Storage == nil {
		return trace.BadParameter("missing cluster storage")
	}

	if c.KubeconfigsDir == "" {
		return trace.BadParameter("missing kubeconfigs directory")
	}

	if c.AgentsDir == "" {
		return trace.BadParameter("missing agents directory")
	}

	if c.GatewayCreator == nil {
		c.GatewayCreator = clusters.NewGatewayCreator(c.Storage)
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "daemon")
	}

	if c.ConnectMyComputerRoleSetup == nil {
		roleSetup, err := connectmycomputer.NewRoleSetup(&connectmycomputer.RoleSetupConfig{})
		if err != nil {
			return trace.Wrap(err)
		}
		c.ConnectMyComputerRoleSetup = roleSetup
	}

	if c.ConnectMyComputerTokenProvisioner == nil {
		c.ConnectMyComputerTokenProvisioner = connectmycomputer.NewTokenProvisioner(&connectmycomputer.TokenProvisionerConfig{Clock: c.Clock})
	}

	if c.ConnectMyComputerNodeJoinWait == nil {
		nodeJoinWait, err := connectmycomputer.NewNodeJoinWait(&connectmycomputer.NodeJoinWaitConfig{
			AgentsDir: c.AgentsDir,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		c.ConnectMyComputerNodeJoinWait = nodeJoinWait
	}

	if c.ConnectMyComputerNodeDelete == nil {
		nodeDelete, err := connectmycomputer.NewNodeDelete(&connectmycomputer.NodeDeleteConfig{AgentsDir: c.AgentsDir})
		if err != nil {
			return trace.Wrap(err)
		}

		c.ConnectMyComputerNodeDelete = nodeDelete
	}

	if c.ConnectMyComputerNodeName == nil {
		nodeName, err := connectmycomputer.NewNodeName(&connectmycomputer.NodeNameConfig{AgentsDir: c.AgentsDir})
		if err != nil {
			return trace.Wrap(err)
		}

		c.ConnectMyComputerNodeName = nodeName
	}

	if c.CreateClientCacheFunc == nil {
		c.CreateClientCacheFunc = func(newClientFunc clientcache.NewClientFunc) (ClientCache, error) {
			retryWithRelogin := func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
				return clusters.AddMetadataToRetryableError(ctx, fn)
			}
			return clientcache.New(clientcache.Config{
				Logger:               c.Logger,
				NewClientFunc:        newClientFunc,
				RetryWithReloginFunc: clientcache.RetryWithReloginFunc(retryWithRelogin),
			})
		}
	}

	if c.ClusterIDCache == nil {
		c.ClusterIDCache = &clusteridcache.Cache{}
	}

	return nil
}
