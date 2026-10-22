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
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/spinner"
	"github.com/gravitational/teleport/tool/common"
)

type beamsAddCommand struct {
	*kingpin.CmdClause
	console             bool
	format              string
	region              string
	isTerminalOverwrite func(io.Writer) bool
	// userCredentialClientOverride replaces the root cluster connection in
	// tests.
	userCredentialClientOverride usercredpb.UserExternalCredentialServiceClient
	shareConnectedAccounts       bool
}

func newBeamsAddCommand(parent *kingpin.CmdClause) *beamsAddCommand {
	cmd := &beamsAddCommand{
		CmdClause: parent.Command("add", "Start a new beam, and optionally connect to it via SSH."),
	}
	cmd.Flag("console", "Connect to the beam via SSH after creation.").Default("true").BoolVar(&cmd.console)
	cmd.Flag("region", "Region where the beam should live. If not specified, the closest region is selected.").StringVar(&cmd.region)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Flag("share-connected-accounts", "Share your connected accounts (such as GitHub) with the beam. Defaults to true.").Default("true").BoolVar(&cmd.shareConnectedAccounts)
	cmd.Alias(beamsAddHelp)
	return cmd
}

func (c *beamsAddCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyRegion, err := c.discoverProxyRegion(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	var beam *beamsv1.Beam
	err = client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		// Show spinner after successful connection to avoid spinning during re-login.
		if c.shouldShowSpinner(cf.Stdout(), c.format) {
			creatingBeamSpinner := spinner.New(cf.Stdout(), "Creating beam...")
			defer creatingBeamSpinner.Stop()
		}

		rootClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		// Create the beam.
		rsp, err := rootClient.
			BeamServiceClient().
			CreateBeam(ctx, beamsv1.CreateBeamRequest_builder{
				Egress:         beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED,
				ProxyRegion:    proxyRegion,
				OverrideRegion: c.region,
			}.Build())
		if err != nil {
			return trace.Wrap(err)
		}

		beam = rsp.GetBeam()
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// formatBeams uses the proxy address to derive the Publish URL but we given
	// the beam won't be published yet, there's no need to actually fetch it.
	const proxyAddr = ""

	switch strings.ToLower(c.format) {
	case teleport.JSON:
		c.maybeSyncCredentialsAndPrintError(ctx, tc, beam, cf.Stderr())
		return trace.Wrap(common.PrintJSONIndent(cf.Stdout(), formatBeam(beam, proxyAddr)))
	case teleport.YAML:
		c.maybeSyncCredentialsAndPrintError(ctx, tc, beam, cf.Stderr())
		return trace.Wrap(common.PrintYAML(cf.Stdout(), formatBeam(beam, proxyAddr)))
	default:
		if err := c.printCreatedMessage(cf.Stdout(), beam, c.region); err != nil {
			return trace.Wrap(err)
		}

		// Connect to the beam via SSH.
		if c.console {
			// Sync in the background, and report after the SSH session to not
			// print into it.
			syncErr := make(chan error, 1)
			go func() {
				syncErr <- c.maybeSyncCredentials(ctx, tc, beam)
			}()
			sshErr := sshBeam(cf, tc, beam, nil)
			if err := <-syncErr; err != nil {
				fmt.Fprintf(cf.Stderr(), "Warning: failed to share your connected accounts (such as GitHub) with the beam: %v\n", err)
			}
			if sshErr != nil {
				return trace.Wrap(sshErr)
			}
			return trace.Wrap(c.printReconnectMessage(cf.Stdout(), beam.GetStatus().GetAlias()))
		}
		c.maybeSyncCredentialsAndPrintError(ctx, tc, beam, cf.Stderr())
	}

	return nil
}

func (c *beamsAddCommand) maybeSyncCredentialsAndPrintError(ctx context.Context, tc *client.TeleportClient, beam *beamsv1.Beam, w io.Writer) {
	if err := c.maybeSyncCredentials(ctx, tc, beam); err != nil {
		fmt.Fprintf(w, "Warning: failed to share your connected accounts (such as GitHub) with the beam: %v\n", err)
	}
}

func (c *beamsAddCommand) maybeSyncCredentials(ctx context.Context, tc *client.TeleportClient, beam *beamsv1.Beam) error {
	if !c.shareConnectedAccounts {
		logger.DebugContext(ctx, "Skipping credentials sync as sharing connected accounts is disabled")
		return nil
	}

	// Beams with a delegation session (Delegation V1) do not create
	// credentials records.
	//
	// TODO(greedy52) remove once beams no longer get a delegation session on
	// all supported clusters.
	if beam.GetStatus().GetDelegationSessionId() != "" {
		logger.DebugContext(ctx, "Skipping credentials sync for beam with a delegation session", "beam", beam.GetStatus().GetAlias())
		return nil
	}

	// Sessions without an encryption key have no credentials.
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	} else if keyRing.EncryptionPrivateKey == nil {
		logger.DebugContext(ctx, "No encryption key registered for user session")
		return nil
	}

	botUser, err := services.BotResourceName(apiscopes.QualifiedName{
		Name: beam.GetStatus().GetBotName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clt, closeClient, err := c.connectUserCredentialClient(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeClient()

	resp, err := clt.GetUserExternalCredentials(ctx, &usercredpb.GetUserExternalCredentialsRequest{})
	if trace.IsNotImplemented(err) {
		logger.DebugContext(ctx, "Cluster does not support user credentials", "error", err)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	} else if len(resp.GetCredentials().GetSpec().GetCredentials()) == 0 {
		logger.DebugContext(ctx, "No user credentials found for user session")
		return nil
	}

	// The beam's tbot registers its credentials record when it joins, which is
	// before the beam is ready. The bot of a beam is expected to have a single
	// session.
	listResp, err := clt.ListUserExternalCredentials(ctx, usercredpb.ListUserExternalCredentialsRequest_builder{
		Filter: usercredpb.EncryptionKeyID_builder{
			User:     tc.Username,
			Delegate: botUser,
		}.Build(),
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	} else if len(listResp.GetCredentials()) == 0 {
		return trace.NotFound("the beam has not registered its session")
	}

	for _, target := range listResp.GetCredentials() {
		parsed, err := x509.ParsePKIXPublicKey(target.GetSpec().GetEncryptionPublicKey())
		if err != nil {
			return trace.Wrap(err)
		}
		targetKey, ok := parsed.(*ecdsa.PublicKey)
		if !ok {
			return trace.BadParameter("unsupported encryption public key type %T", parsed)
		}
		credentials, err := client.ReencryptUserExternalCredentials(ctx, resp.GetCredentials(), keyRing.EncryptionPrivateKey, targetKey)
		if err != nil {
			return trace.Wrap(err)
		}
		synced := proto.CloneOf(target)
		synced.GetSpec().SetCredentials(credentials)
		if _, err := clt.SyncUserExternalCredentials(ctx, usercredpb.SyncUserExternalCredentialsRequest_builder{
			Credentials: synced,
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// connectUserCredentialClient returns the user external credential client of
// the root cluster, and a function to close the connection.
func (c *beamsAddCommand) connectUserCredentialClient(ctx context.Context, tc *client.TeleportClient) (usercredpb.UserExternalCredentialServiceClient, func(), error) {
	if c.userCredentialClientOverride != nil {
		return c.userCredentialClientOverride, func() {}, nil
	}
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		clusterClient.Close()
		return nil, nil, trace.Wrap(err)
	}
	return rootClient.UserExternalCredentialClient(), func() {
		rootClient.Close()
		clusterClient.Close()
	}, nil
}

func (c *beamsAddCommand) discoverProxyRegion(ctx context.Context, tc *client.TeleportClient) (string, error) {
	resp, err := webclient.Find(&webclient.Config{
		Context:      ctx,
		ProxyAddr:    tc.WebProxyAddr,
		Insecure:     tc.InsecureSkipVerify,
		ExtraHeaders: tc.ExtraProxyHeaders,
	})
	if err != nil {
		return "", trace.Wrap(err, "discovering proxy region")
	}
	return resp.Proxy.GroupID, nil
}

func (c *beamsAddCommand) printCreatedMessage(w io.Writer, beam *beamsv1.Beam, overrideRegion string) error {
	region := beam.GetStatus().GetRegion()
	alias := beam.GetStatus().GetAlias()

	switch {
	case overrideRegion != "" && region != "" && overrideRegion != region:
		_, err := fmt.Fprintf(w, "Region %s not supported. Beam %q created in %s.\n", overrideRegion, alias, region)
		return trace.Wrap(err)
	case region != "":
		_, err := fmt.Fprintf(w, "Beam %q created in %s.\n", alias, region)
		return trace.Wrap(err)
	default:
		_, err := fmt.Fprintf(w, "Beam %q created.\n", alias)
		return trace.Wrap(err)
	}
}

func (c *beamsAddCommand) printReconnectMessage(w io.Writer, alias string) error {
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	_, err := fmt.Fprintln(w, gray.Render(fmt.Sprintf("\nTo reconnect to this beam, run:\n    tsh beams ssh %s", alias)))
	return trace.Wrap(err)
}

func (c *beamsAddCommand) shouldShowSpinner(w io.Writer, format string) bool {
	switch strings.ToLower(format) {
	case teleport.JSON, teleport.YAML:
		return false
	default:
		if c.isTerminalOverwrite != nil {
			return c.isTerminalOverwrite(w)
		}
		return utils.IsTerminal(w)
	}
}
