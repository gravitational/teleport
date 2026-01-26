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

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
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
	Client            gcp.InstancesClient
	Instances         []*gcpimds.Instance
	InstallerParams   *types.InstallerParams
	Zone              string
	ProjectID         string
	SSHKeyAlgo        cryptosuites.Algorithm
	PublicProxyGetter func(context.Context) (string, error)
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (gi *GCPInstaller) Run(ctx context.Context, req GCPRunRequest) error {
	script, err := installerScript(ctx, req.InstallerParams, withProxyAddrGetter(req.PublicProxyGetter))
	if err != nil {
		return trace.Wrap(err)
	}

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
				Script:     script,
				SSHKeyAlgo: req.SSHKeyAlgo,
			}
			return trace.Wrap(gcp.RunCommand(ctx, &runRequest))
		})
	}
	return trace.Wrap(g.Wait())
}
