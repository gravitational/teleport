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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/services/clientcache"
	"github.com/gravitational/teleport/lib/teleterm/services/connectmycomputer"
)

// Storage defines an interface for cluster profile storage.
type Storage interface {
	clusters.Resolver

	ReadAll() ([]*clusters.Cluster, error)
	Add(ctx context.Context, webProxyAddress string) (*clusters.Cluster, *client.TeleportClient, error)
	Remove(ctx context.Context, profileName string) error
	GetByResourceURI(resourceURI uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error)
}

// Config is the cluster service config
type Config struct {
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// Storage is a storage service that reads/writes to tsh profiles
	Storage Storage
	// Log is a component logger
	Log *logrus.Entry
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

	CreateClientCacheFunc func(resolver ResolveClusterFunc) ClientCache
}

// ResolveClusterFunc returns a cluster by URI.
type ResolveClusterFunc func(uri uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error)

// ClientCache stores clients keyed by cluster URI.
type ClientCache interface {
	// Get returns a client from the cache if there is one,
	// otherwise it dials the remote server.
	// The caller should not close the returned client.
	Get(ctx context.Context, clusterURI uri.ResourceURI) (*client.ClusterClient, error)
	// ClearForRoot closes and removes clients from the cache
	// for the root cluster and its leaf clusters.
	ClearForRoot(clusterURI uri.ResourceURI) error
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

	if c.Log == nil {
		c.Log = logrus.NewEntry(logrus.StandardLogger()).WithField(teleport.ComponentKey, "daemon")
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
		c.CreateClientCacheFunc = func(resolver ResolveClusterFunc) ClientCache {
			return clientcache.New(clientcache.Config{
				Log:                c.Log,
				ResolveClusterFunc: clientcache.ResolveClusterFunc(resolver),
			})
		}
	}

	return nil
}
