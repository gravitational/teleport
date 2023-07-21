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
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/cmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/services/connectmycomputer"
)

// Config is the cluster service config
type Config struct {
	// Storage is a storage service that reads/writes to tsh profiles
	Storage *clusters.Storage
	// Log is a component logger
	Log *logrus.Entry
	// PrehogAddr is the URL where prehog events should be submitted.
	PrehogAddr string

	GatewayCreator         GatewayCreator
	DBCLICommandProvider   gateway.CLICommandProvider
	KubeCLICommandProvider gateway.CLICommandProvider
	// CreateTshdEventsClientCredsFunc lazily creates creds for the tshd events server ran by the
	// Electron app. This is to ensure that the server public key is written to the disk under the
	// expected location by the time we get around to creating the client.
	CreateTshdEventsClientCredsFunc CreateTshdEventsClientCredsFunc
	ConnectMyComputerRoleSetup      *connectmycomputer.RoleSetup
}

type CreateTshdEventsClientCredsFunc func() (grpc.DialOption, error)

// CheckAndSetDefaults checks the configuration for its validity and sets default values if needed
func (c *Config) CheckAndSetDefaults() error {
	if c.Storage == nil {
		return trace.BadParameter("missing cluster storage")
	}

	if c.GatewayCreator == nil {
		c.GatewayCreator = clusters.NewGatewayCreator(c.Storage)
	}

	if c.Log == nil {
		c.Log = logrus.NewEntry(logrus.StandardLogger()).WithField(trace.Component, "daemon")
	}

	if c.DBCLICommandProvider == nil {
		c.DBCLICommandProvider = cmd.NewDBCLICommandProvider(c.Storage, dbcmd.SystemExecer{})
	}

	if c.KubeCLICommandProvider == nil {
		c.KubeCLICommandProvider = cmd.NewKubeCLICommandProvider()
	}

	if c.ConnectMyComputerRoleSetup == nil {
		roleSetup, err := connectmycomputer.NewRoleSetup(&connectmycomputer.RoleSetupConfig{})
		if err != nil {
			return trace.Wrap(err)
		}
		c.ConnectMyComputerRoleSetup = roleSetup
	}

	return nil
}
