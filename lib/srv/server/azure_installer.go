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
	"crypto/rand"
	"fmt"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstaller handles running commands that install Teleport on Azure
// virtual machines.
type AzureInstaller struct {
	Emitter apievents.Emitter
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
	ClientID        string
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (ai *AzureInstaller) Run(ctx context.Context, req AzureRunRequest) error {
	script, err := getInstallerScript(req.ScriptName, req.PublicProxyAddr, req.ClientID)
	if err != nil {
		return trace.Wrap(err)
	}
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
				VMName:        azure.StringVal(inst.Name),
				Parameters:    req.Params,
				Script:        script,
			}
			return trace.Wrap(req.Client.Run(ctx, runRequest))
		})
	}
	return trace.Wrap(g.Wait())
}

func getInstallerScript(installerName, publicProxyAddr, clientID string) (string, error) {
	installerURL, err := url.Parse(fmt.Sprintf("https://%s/v1/webapi/scripts/installer/%v", publicProxyAddr, installerName))
	if err != nil {
		return "", trace.Wrap(err)
	}
	if clientID != "" {
		q := installerURL.Query()
		q.Set("azure-client-id", clientID)
		installerURL.RawQuery = q.Encode()
	}

	// Azure treats scripts with the same content as the same invocation and
	// won't run them more than once. This is fine when the installer script
	// succeeds, but it makes troubleshooting much harder when it fails. To
	// work around this, we generate a random string and append it as a comment
	// to the script, forcing Azure to see each invocation as unique.
	nonce := make([]byte, 8)
	// No big deal if rand.Read fails, the script is still valid.
	_, _ = rand.Read(nonce)
	script := fmt.Sprintf("curl -s -L %s| bash -s $@ #%x", installerURL, nonce)
	return script, nil
}
