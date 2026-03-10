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
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func onBeamsAdd(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	stopCreating := startBeamSpinner(cf.Stdout(), "creating...")
	var (
		beamID   string
		beamNode string
	)
	createErr := client.RetryWithRelogin(cf.Context, tc, func() error {
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
		beamNode = beam.GetStatus().GetNodeId()
		return nil
	})
	if createErr != nil {
		stopCreating("")
		return trace.Wrap(createErr)
	}
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	diamondStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	stopCreating(fmt.Sprintf("%s created %s", diamondStyle.Render("◆"), idStyle.Render(beamID)))

	if cf.BeamNoConsole {
		return nil
	}

	stopConnecting := startBeamSpinner(cf.Stdout(), "connecting...")
	err = connectToNodeSSH(cf, tc, beamNode, nil)
	if err != nil {
		stopConnecting("")
		return trace.Wrap(err)
	}
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	stopConnecting(fmt.Sprintf("%s ready", arrowStyle.Render("↳")))
	return nil
}

func onBeamsConsole(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true
	nodeID, err := getBeamNodeID(cf.Context, tc, cf.BeamID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(connectToNodeSSH(cf, tc, nodeID, nil))
}

func onBeamsExec(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true
	nodeID, err := getBeamNodeID(cf.Context, tc, cf.BeamID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(connectToNodeSSH(cf, tc, nodeID, cf.RemoteCommand))
}

func onBeamsList(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	var beams []*beamsv1.Beam
	if err := tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
		beams, err = stream.Collect(clientutils.Resources(cf.Context, func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
			resp, err := clt.BeamsServiceClient().ListBeams(ctx, &beamsv1.ListBeamsRequest{
				PageSize:  int32(pageSize),
				PageToken: pageToken,
			})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return resp.GetBeams(), resp.GetNextPageToken(), nil
		}))
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	slices.SortFunc(beams, func(a, b *beamsv1.Beam) int {
		return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	fmt.Fprint(cf.Stdout(), renderBeamsTable(beams, tc.WebProxyHost()))
	return nil
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

func onBeamsPublish(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	protocol := beamsv1.Protocol_PROTOCOL_HTTP
	if cf.BeamTCP {
		protocol = beamsv1.Protocol_PROTOCOL_TCP
	}

	var addr string
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		resp, err := clusterClient.AuthClient.BeamsServiceClient().Publish(cf.Context, &beamsv1.PublishRequest{
			BeamId:   cf.BeamID,
			Port:     8080,
			Protocol: protocol,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		addr = resp.GetAddr()
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(cf.Stdout(), addr)
	return nil
}

func getBeamNodeID(ctx context.Context, tc *client.TeleportClient, beamID string) (string, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer clusterClient.Close()

	beam, err := clusterClient.AuthClient.BeamsServiceClient().GetBeam(ctx, &beamsv1.GetBeamRequest{
		BeamId: beamID,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	nodeID := beam.GetStatus().GetNodeId()
	if nodeID == "" {
		return "", trace.NotFound("beam %q has no node", beamID)
	}
	return nodeID, nil
}

func connectToNodeSSH(cf *CLIConf, tc *client.TeleportClient, nodeID string, remoteCommand []string) error {
	tc.HostLogin = "root"
	if cf.NodeLogin != "" {
		tc.HostLogin = cf.NodeLogin
	}
	target := beamNodeTarget(nodeID)

	tc.Stdin = cf.Stdin()
	sshFunc := func() error {
		return tc.SSH(cf.Context, remoteCommand, client.WithHostAddress(target.Addr))
	}
	if !cf.Relogin {
		return trace.Wrap(sshFunc())
	}
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, sshFunc))
}

func renderBeamsTable(beams []*beamsv1.Beam, proxyHost string) string {
	table := asciitable.MakeTable([]string{"Name", "Expiry"})
	for _, beam := range beams {
		table.AddRow([]string{
			beam.GetMetadata().GetName(),
			beamExpiry(beam),
		})
	}
	return table.AsBuffer().String()
}

func beamExpiry(beam *beamsv1.Beam) string {
	expires := beam.GetMetadata().GetExpires()
	if expires == nil {
		return ""
	}
	t := expires.AsTime()
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// startBeamSpinner prints an animated braille spinner with msg to w.
// Call the returned stop function with a finalLine to replace the spinner
// line in-place. Pass an empty string to just clear the line. stop blocks
// until the goroutine has finished writing.
func startBeamSpinner(w io.Writer, msg string) func(finalLine string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	done := make(chan string)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case finalLine := <-done:
				fmt.Fprintf(w, "\r%s\r%s\n", strings.Repeat(" ", 40), finalLine)
				return
			case <-ticker.C:
				fmt.Fprintf(w, "\r%s %s", spinStyle.Render(frames[i%len(frames)]), msg)
				i++
			}
		}
	}()
	return func(finalLine string) {
		done <- finalLine
		<-stopped
	}
}

func beamNodeTarget(nodeID string) *client.TargetNode {
	return &client.TargetNode{
		Hostname: nodeID,
		Addr:     nodeID + ":0",
	}
}
