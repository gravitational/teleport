/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
)

const (
	beamAddPollInterval = 500 * time.Millisecond
	beamAddPollTimeout  = 5 * time.Minute
)

func onBeamsAdd(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	var beamID string
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		beam, err := clusterClient.AuthClient.BeamsServiceClient().CreateBeam(cf.Context, &beamsv1.CreateBeamRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		beamID = beam.GetMetadata().GetName()
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	printBeamCreating(cf.Stdout(), beamID)

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	tc.Host = ""
	tc.Labels = nil
	tc.SearchKeywords = nil
	tc.PredicateExpression = fmt.Sprintf(`labels["teleport.internal/beam/id"]==%q`, beamID)
	tc.HostLogin = cmp.Or(cf.NodeLogin, "root")

	stop := startBeamConnecting(cf.Stdout())
	target, err := waitForBeamNode(cf.Context, tc, clusterClient.AuthClient)
	stop()
	if err != nil {
		return trace.Wrap(err)
	}
	printBeamReady(cf.Stdout())

	tc.Stdin = cf.Stdin()
	sshFunc := func() error {
		return tc.SSH(cf.Context, nil, client.WithHostAddress(target.Addr))
	}
	if !cf.Relogin {
		return trace.Wrap(sshFunc())
	}
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, sshFunc))
}

func onBeamsAllow(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		_, err = clusterClient.AuthClient.BeamsServiceClient().AllowDomain(cf.Context, &beamsv1.AllowDomainRequest{
			BeamId: cf.BeamID,
			Fqdns:  []string{cf.BeamDomain},
		})
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Allowed domain %q for beam %q\n", cf.BeamDomain, cf.BeamID)
	return nil
}

func printBeamCreating(w io.Writer, beamID string) {
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	diamondStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	fmt.Fprintf(w, "%s creating %s\n", diamondStyle.Render("◆"), idStyle.Render(beamID))
}

func printBeamReady(w io.Writer) {
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	fmt.Fprintf(w, "%s ready\n", arrowStyle.Render("↳"))
}

// startBeamConnecting prints an animated braille spinner to w while connecting.
// Call the returned stop function to end the spinner.
func startBeamConnecting(w io.Writer) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-done:
				fmt.Fprintf(w, "\n")
				return
			case <-ticker.C:
				fmt.Fprintf(w, "\r%s connecting...", spinStyle.Render(frames[i%len(frames)]))
				i++
			}
		}
	}()
	return func() { close(done) }
}

func waitForBeamNode(ctx context.Context, tc *client.TeleportClient, authClient authclient.ClientI) (*client.TargetNode, error) {
	pollCtx, cancel := context.WithTimeout(ctx, beamAddPollTimeout)
	defer cancel()

	ticker := time.NewTicker(beamAddPollInterval)
	defer ticker.Stop()

	for {
		page, _, err := apiclient.GetUnifiedResourcePage(pollCtx, authClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{types.KindNode},
			SortBy:              types.SortBy{Field: types.ResourceMetadataName},
			PredicateExpression: tc.PredicateExpression,
			Limit:               1,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(page) > 0 {
			node, ok := page[0].ResourceWithLabels.(types.Server)
			if !ok {
				return nil, trace.BadParameter("expected node resource, got %T", page[0].ResourceWithLabels)
			}
			return &client.TargetNode{
				Hostname: node.GetHostname(),
				Addr:     node.GetName() + ":0",
			}, nil
		}

		select {
		case <-ticker.C:
		case <-pollCtx.Done():
			return nil, trace.LimitExceeded("timed out waiting for beam node to register")
		}
	}
}
