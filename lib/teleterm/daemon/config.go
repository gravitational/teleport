/*
Copyright 2021 Gravitational, Inc.

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

package daemon

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
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
		c.Log = logrus.NewEntry(logrus.StandardLogger()).WithField(trace.Component, "daemon")
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
	return nil
}
