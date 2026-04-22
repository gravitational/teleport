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
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/itertools/stream"
	filesftp "github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/teleport/tool/common"
)

func onBeamsAdd(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	// Only show the spinner and SSH into the beam if there's a real terminal
	// connected (i.e. not piping the output somewhere else) and the user hasn't
	// requested JSON or YAML.
	interactive := utils.IsTerminal(cf.Stdout())
	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.JSON, teleport.YAML:
		interactive = false
	}

	stopSpinner := func(message string) {
		if message != "" {
			fmt.Fprintln(cf.Stdout(), message)
		}
	}
	if interactive {
		stopSpinner = startBeamSpinner(cf.Stdout(), "creating...")
	}

	var beam *beamsv1.Beam
	createErr := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		beam, err = clusterClient.AuthClient.BeamsServiceClient().CreateBeam(cf.Context, &beamsv1.CreateBeamRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if createErr != nil {
		stopSpinner("")
		return trace.Wrap(createErr)
	}

	switch format {
	case teleport.JSON:
		if err := common.PrintJSONIndent(cf.Stdout(), serializeBeam(beam)); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case teleport.YAML:
		if err := common.PrintYAML(cf.Stdout(), serializeBeam(beam)); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// TODO(boxofrad): Remove this once all tenants have been updated to a
	// version that supports beam aliases.
	name := beam.GetMetadata().GetName()
	if alias := beam.GetStatus().GetAlias(); alias != "" {
		name = alias
	}
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	diamondStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	stopSpinner(fmt.Sprintf("%s created %s", diamondStyle.Render("◆"), idStyle.Render(name)))

	if !cf.BeamConsole || !interactive {
		return nil
	}

	if err = connectToBeamSSHWithRetry(cf, tc, beam.GetStatus().GetNodeId(), nil); err != nil {
		return trace.Wrap(err)
	}

	reconnectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	fmt.Fprintf(cf.Stdout(), "\nTo reconnect to this beam, run:\n    %s\n",
		reconnectStyle.Render("tsh beams ssh "+name))
	return nil
}

func onBeamsSSH(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true
	nodeID, err := getBeamNodeID(cf.Context, tc, cf.BeamID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(connectToBeamSSHWithRetry(cf, tc, nodeID, nil))
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
	return trace.Wrap(connectToBeamSSHWithRetry(cf, tc, nodeID, cf.RemoteCommand))
}

func onBeamsList(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	var beams []*beamsv1.Beam
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		return trace.Wrap(tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
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
		}))
	}); err != nil {
		return trace.Wrap(err)
	}

	slices.SortFunc(beams, func(a, b *beamsv1.Beam) int {
		return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.JSON:
		_, proxyAddr, err := fetchProxyVersion(cf)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := common.PrintJSONIndent(cf.Stdout(), serializeBeams(beams, proxyAddr)); err != nil {
			return trace.Wrap(err)
		}
	case teleport.YAML:
		_, proxyAddr, err := fetchProxyVersion(cf)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := common.PrintYAML(cf.Stdout(), serializeBeams(beams, proxyAddr)); err != nil {
			return trace.Wrap(err)
		}
	default:
		fmt.Fprint(cf.Stdout(), renderBeamsTable(beams, tc.WebProxyHost()))
	}
	return nil
}

type serializedBeam struct {
	ID             string    `json:"id"`
	Alias          string    `json:"alias"`
	Owner          string    `json:"owner"`
	Expires        time.Time `json:"expires"`
	PublishAddress string    `json:"publish_address,omitempty"`
}

func serializeBeam(beam *beamsv1.Beam) serializedBeam {
	return serializedBeam{
		ID:      beam.GetMetadata().GetName(),
		Alias:   beam.GetStatus().GetAlias(),
		Owner:   beam.GetStatus().GetUser(),
		Expires: beam.GetMetadata().Expires.AsTime(),
	}
}

func serializeBeams(
	beams []*beamsv1.Beam,
	proxyAddr string,
) []serializedBeam {
	return sliceutils.Map(beams, func(beam *beamsv1.Beam) serializedBeam {
		e := serializeBeam(beam)
		if appName := beam.GetStatus().GetAppName(); appName != "" {
			e.PublishAddress = utils.DefaultAppPublicAddr(appName, proxyAddr)
		}
		return e
	})
}

func onBeamsDelete(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	idMap := make(map[string]string, len(cf.BeamIDs))
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		beamSvc := clusterClient.AuthClient.BeamsServiceClient()
		for _, input := range cf.BeamIDs {
			beam, err := beamSvc.GetBeam(cf.Context, getBeamRequest(input))
			if err != nil {
				return trace.Wrap(err)
			}
			if beam.GetMetadata().GetName() == "" {
				return trace.NotFound("beam %q has no ID", input)
			}
			idMap[input] = beam.GetMetadata().GetName()
		}

		for _, input := range cf.BeamIDs {
			_, err = beamSvc.DeleteBeam(cf.Context, &beamsv1.DeleteBeamRequest{
				BeamId: idMap[input],
			})
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Fprintf(cf.Stdout(), "Deleted beam %q\n", input)
		}
		return nil
	}))
}

func onBeamsPublish(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	var port int32 = 8080
	protocol := beamsv1.Protocol_PROTOCOL_HTTP
	if cf.BeamTCP {
		protocol = beamsv1.Protocol_PROTOCOL_TCP
	}
	beamID, err := resolveBeamID(cf.Context, tc, cf.BeamID)
	if err != nil {
		return trace.Wrap(err)
	}

	var addr string
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		resp, err := clusterClient.AuthClient.BeamsServiceClient().Publish(cf.Context, &beamsv1.PublishRequest{
			BeamId:   beamID,
			Port:     port,
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

	var dialAddr string
	if protocol == beamsv1.Protocol_PROTOCOL_HTTP {
		dialAddr = fmt.Sprintf("https://%s", addr)
	} else {
		dialAddr = fmt.Sprintf("tcp://%s:%d", addr, port)
	}

	if cf.Quiet || protocol == beamsv1.Protocol_PROTOCOL_HTTP {
		fmt.Fprintln(cf.Stdout(), dialAddr)
	} else {
		// TODO(boxofrad): Return the app name in the `Publish` response rather than
		// constructing it here.
		const usageText = "Connect to your TCP application from another beam by dialing:\n%s\n\n" +
			"Or start a local tunnel to the application with:\n" +
			"tsh proxy app beam-%s\n"
		fmt.Fprintf(cf.Stdout(), usageText, dialAddr, beamID)
	}

	return nil
}

type beamCopySpec struct {
	Source      beamCopyTarget
	Destination beamCopyTarget
}

type beamCopyTarget struct {
	Path   string
	BeamID string
	IsBeam bool
}

func onBeamsSCP(cf *CLIConf) error {
	spec, err := parseBeamCopySpec(cf.BeamCopySpec)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(runBeamCopy(cf, spec))
}

func runBeamCopy(cf *CLIConf, spec beamCopySpec) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true
	source, err := resolveBeamCopyTarget(cf, tc, spec.Source)
	if err != nil {
		return trace.Wrap(err)
	}
	destination, err := resolveBeamCopyTarget(cf, tc, spec.Destination)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := copyBeamFile(cf, tc, []string{source}, destination); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(cf.Stdout(), "Copied successfully.")
	return nil
}

func parseBeamCopySpec(rawSpec []string) (beamCopySpec, error) {
	if len(rawSpec) != 2 {
		return beamCopySpec{}, trace.BadParameter("source and destination are required")
	}

	source, err := parseBeamCopyTarget(rawSpec[0])
	if err != nil {
		return beamCopySpec{}, trace.Wrap(err)
	}
	destination, err := parseBeamCopyTarget(rawSpec[1])
	if err != nil {
		return beamCopySpec{}, trace.Wrap(err)
	}

	if !source.IsBeam && !destination.IsBeam {
		return beamCopySpec{}, trace.BadParameter("one of source or destination must be a beam path in the form BEAM:PATH")
	}

	return beamCopySpec{
		Source:      source,
		Destination: destination,
	}, nil
}

func parseBeamCopyTarget(rawTarget string) (beamCopyTarget, error) {
	if !filesftp.IsRemotePath(rawTarget) {
		return beamCopyTarget{Path: rawTarget}, nil
	}

	beamID, path, _ := strings.Cut(rawTarget, ":")
	if beamID == "" {
		return beamCopyTarget{}, trace.BadParameter("%q is missing a beam reference, use the form BEAM:PATH", rawTarget)
	}

	return beamCopyTarget{
		Path:   path,
		BeamID: beamID,
		IsBeam: true,
	}, nil
}

func resolveBeamCopyTarget(cf *CLIConf, tc *client.TeleportClient, target beamCopyTarget) (string, error) {
	if !target.IsBeam {
		return target.Path, nil
	}

	nodeID, err := getBeamNodeID(cf.Context, tc, target.BeamID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return beamRemotePath(cf, tc, nodeID, target.Path), nil
}

func resolveBeamID(ctx context.Context, tc *client.TeleportClient, beamRef string) (string, error) {
	beam, err := getBeam(ctx, tc, beamRef)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if beam.GetMetadata().GetName() == "" {
		return "", trace.NotFound("beam %q has no ID", beamRef)
	}
	return beam.GetMetadata().GetName(), nil
}

func getBeam(ctx context.Context, tc *client.TeleportClient, beamRef string) (*beamsv1.Beam, error) {
	var beam *beamsv1.Beam
	err := client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		beam, err = clusterClient.AuthClient.BeamsServiceClient().GetBeam(ctx, getBeamRequest(beamRef))
		return trace.Wrap(err)
	})
	return beam, trace.Wrap(err)
}

func getBeamRequest(beamRef string) *beamsv1.GetBeamRequest {
	if _, err := uuid.Parse(beamRef); err == nil {
		return &beamsv1.GetBeamRequest{
			Selector: &beamsv1.GetBeamRequest_Id{Id: beamRef},
		}
	}

	return &beamsv1.GetBeamRequest{
		Selector: &beamsv1.GetBeamRequest_Alias{Alias: beamRef},
	}
}

func getBeamNodeID(ctx context.Context, tc *client.TeleportClient, beamID string) (string, error) {
	beam, err := getBeam(ctx, tc, beamID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	nodeID := beam.GetStatus().GetNodeId()
	if nodeID == "" {
		return "", trace.NotFound("beam %q has no node", beamID)
	}

	return nodeID, nil
}

func copyBeamFile(cf *CLIConf, tc *client.TeleportClient, sources []string, destination string) error {
	req := client.SFTPRequest{
		Sources:     sources,
		Destination: destination,
		Recursive:   cf.RecursiveCopy,
	}
	if !cf.Quiet {
		req.ProgressWriter = cf.Stdout()
	}

	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, func() error {
		return trace.Wrap(tc.SFTP(cf.Context, req))
	}))
}

func connectToBeamSSH(cf *CLIConf, tc *client.TeleportClient, nodeID string, remoteCommand []string) error {
	tc.HostLogin = "beams"
	if cf.NodeLogin != "" {
		tc.HostLogin = cf.NodeLogin
	}
	tc.Stdin = cf.Stdin()

	target := beamNodeTarget(nodeID)
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SSH(cf.Context, remoteCommand, client.WithHostAddress(target.Addr))
	}))
}

func connectToBeamSSHWithRetry(cf *CLIConf, tc *client.TeleportClient, nodeID string, remoteCommand []string) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  100 * time.Millisecond,
		Step:   100 * time.Millisecond,
		Max:    time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var lastErr error
	for i := range 10 {
		lastErr = connectToBeamSSH(cf, tc, nodeID, remoteCommand)
		if lastErr == nil {
			return nil
		}
		logger.DebugContext(cf.Context, "Connect to beam with retry", "attempt", i+1, "error", lastErr)

		switch {
		case trace.IsNotFound(lastErr):
			// Cache may not have caught up with the node write.
		case trace.IsConnectionProblem(lastErr):
			// Beam network may not be ready yet, even if sshd is.
		default:
			return trace.Wrap(lastErr)
		}

		select {
		case <-cf.Context.Done():
			return trace.Wrap(cf.Context.Err())
		case <-retry.After():
			retry.Inc()
		}
	}

	return trace.Wrap(lastErr)
}

func renderBeamsTable(beams []*beamsv1.Beam, proxyHost string) string {
	table := asciitable.MakeTable([]string{"Alias", "ID", "Expires"})
	for _, beam := range beams {
		beamID := beam.GetMetadata().GetName()
		table.AddRow([]string{
			beam.GetStatus().GetAlias(),
			beamID,
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
	return humanize.Time(t)
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

func beamRemotePath(cf *CLIConf, tc *client.TeleportClient, nodeID, remotePath string) string {
	login := "beams"
	if cf.NodeLogin != "" {
		login = cf.NodeLogin
	}

	return fmt.Sprintf("%s@%s:%s", login, nodeID, remotePath)
}
