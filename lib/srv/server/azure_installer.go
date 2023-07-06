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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstaller handles running commands that install Teleport on Azure
// virtual machines.
type AzureInstaller struct {
	Emitter     apievents.Emitter
	AccessPoint auth.DiscoveryAccessPoint
}

// AzureRunRequest combines parameters for running commands on a set of Azure
// virtual machines.
type AzureRunRequest struct {
	Client          azure.RunCommandClient
	Instances       []*armcompute.VirtualMachine
	Params          []string
	Region          string
	ResourceGroup   string
	ScriptName      string
	PublicProxyAddr string
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (ai *AzureInstaller) Run(ctx context.Context, req AzureRunRequest) error {
	g, ctx := errgroup.WithContext(ctx)
	// Somewhat arbitrary limit to make sure Teleport doesn't have to install
	// hundreds of nodes at once.
	g.SetLimit(10)

	for _, inst := range req.Instances {
		inst := inst
		g.Go(func() error {
			runRequest := azure.RunCommandRequest{
				Region:        req.Region,
				ResourceGroup: req.ResourceGroup,
				VMName:        aws.StringValue(inst.Name),
				Parameters:    req.Params,
				Script:        getInstallerScript(req.ScriptName, req.PublicProxyAddr),
			}
			return trace.Wrap(req.Client.Run(ctx, runRequest))
		})
	}
	return trace.Wrap(g.Wait())
}

func getInstallerScript(installerName, publicProxyAddr string) string {
	return fmt.Sprintf("curl -s -L https://%s/webapi/scripts/installer/%v | bash -s $@", publicProxyAddr, installerName)
}
