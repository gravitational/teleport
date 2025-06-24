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

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// GCPInstaller handles running commands that install Teleport on GCP
// virtual machines.
type GCPInstaller struct {
	Emitter apievents.Emitter
}

// GCPRunRequest combines parameters for running commands on a set of GCP
// virtual machines.
type GCPRunRequest struct {
	Client          gcp.InstancesClient
	Instances       []*gcpimds.Instance
	Params          []string
	Zone            string
	ProjectID       string
	ScriptName      string
	PublicProxyAddr string
	SSHKeyAlgo      cryptosuites.Algorithm
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (gi *GCPInstaller) Run(ctx context.Context, req GCPRunRequest) error {
	g, ctx := errgroup.WithContext(ctx)
	// Somewhat arbitrary limit to make sure Teleport doesn't have to install
	// hundreds of nodes at once.
	g.SetLimit(10)

	for _, inst := range req.Instances {
		g.Go(func() error {
			runRequest := gcp.RunCommandRequest{
				Client: req.Client,
				InstanceRequest: gcpimds.InstanceRequest{
					ProjectID: inst.ProjectID,
					Zone:      inst.Zone,
					Name:      inst.Name,
				},
				Script: getGCPInstallerScript(
					req.ScriptName,
					req.PublicProxyAddr,
					req.Params,
				),
				SSHKeyAlgo: req.SSHKeyAlgo,
			}
			return trace.Wrap(gcp.RunCommand(ctx, &runRequest))
		})
	}
	return trace.Wrap(g.Wait())
}

func getGCPInstallerScript(installerName, publicProxyAddr string, params []string) string {
	return fmt.Sprintf("curl -s -L https://%s/v1/webapi/scripts/installer/%s | bash -s %s",
		publicProxyAddr,
		installerName,
		strings.Join(params, " "),
	)
}
