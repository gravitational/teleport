/*
Copyright 2023 Gravitational, Inc.

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

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/gcp"
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
	Instances       []*gcp.Instance
	Params          []string
	Zone            string
	ProjectID       string
	ScriptName      string
	PublicProxyAddr string
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (gi *GCPInstaller) Run(ctx context.Context, req GCPRunRequest) error {
	g, ctx := errgroup.WithContext(ctx)
	// Somewhat arbitrary limit to make sure Teleport doesn't have to install
	// hundreds of nodes at once.
	g.SetLimit(10)

	for _, inst := range req.Instances {
		inst := inst
		g.Go(func() error {
			runRequest := gcp.RunCommandRequest{
				Client:          req.Client,
				InstanceRequest: inst.InstanceRequest(),
				Script: getGCPInstallerScript(
					req.ScriptName,
					req.PublicProxyAddr,
					req.Params,
				),
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
