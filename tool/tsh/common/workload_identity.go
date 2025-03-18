/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
)

type workloadIdentityCommands struct {
	issueX509 *issueX509Command
}

func newWorkloadIdentityCommands(
	app *kingpin.Application,
) workloadIdentityCommands {
	cmd := app.Command("workload-identity", "Issue Workload Identity credentials")
	cmds := workloadIdentityCommands{
		issueX509: newIssueX509Command(cmd),
	}
	return cmds
}

type issueX509Command struct {
	*kingpin.CmdClause
	nameSelector    string
	labelSelector   string
	ttl             time.Duration
	outputDirectory string
}

func newIssueX509Command(parent *kingpin.CmdClause) *issueX509Command {
	cmd := &issueX509Command{
		CmdClause: parent.Command("issue-x509", "Use Teleport Workload Identity to issue an X509 credential write it to a local directory."),
	}

	cmd.Flag(
		"name-selector",
		"The name of the workload identity to issue",
	).StringVar(&cmd.nameSelector)
	cmd.Flag(
		"label-selector",
		"A label-based selector for which workload identities to issue. Multiple labels can be provided using ','.",
	).StringVar(&cmd.labelSelector)
	cmd.Flag("credential-ttl", "Sets the time to live for the credential.").
		Default("1h").
		DurationVar(&cmd.ttl)
	cmd.Flag("output", "Path to the directory to write the SVID into.").
		Required().
		StringVar(&cmd.outputDirectory)

	return cmd
}

func (c *issueX509Command) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	selector := config.WorkloadIdentitySelector{}
	switch {
	case c.nameSelector != "" && c.labelSelector != "":
		return trace.BadParameter("cannot specify both name and label selectors")
	case c.nameSelector != "":
		selector.Name = c.nameSelector
	case c.labelSelector != "":
		labels, err := client.ParseLabelSpec(c.labelSelector)
		if err != nil {
			return trace.Wrap(err)
		}
		selector.Labels = map[string][]string{}
		for k, v := range labels {
			selector.Labels[k] = []string{v}
		}
	default:
		return trace.BadParameter("name-selector or label-selector must be specified")
	}

	return client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		credentials, privateKey, err := workloadidentity.IssueX509WorkloadIdentity(
			ctx,
			slog.Default(),
			clusterClient.AuthClient,
			selector,
			c.ttl,
			nil,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		var x509Credential *workloadidentityv1pb.Credential
		switch len(credentials) {
		case 0:
			return trace.BadParameter("no X509 SVIDs returned")
		case 1:
			x509Credential = credentials[0]
		default:
			// We could eventually implement some kind of hint selection mechanism
			// to pick the "right" one.
			received := make([]string, 0, len(credentials))
			for _, cred := range credentials {
				received = append(received,
					fmt.Sprintf(
						"%s:%s",
						cred.WorkloadIdentityName,
						cred.SpiffeId,
					),
				)
			}
			return trace.BadParameter(
				"multiple X509 SVIDs received: %v", received,
			)
		}

		// Write private key
		privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return trace.Wrap(err)
		}
		keyPath := filepath.Join(c.outputDirectory, svidKeyPEMPath)
		err = os.WriteFile(
			keyPath,
			pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privBytes,
			}),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		// Write SVID
		svidPath := filepath.Join(c.outputDirectory, svidPEMPath)
		err = os.WriteFile(
			svidPath,
			pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: x509Credential.GetX509Svid().GetCert(),
			}),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		// Write trust bundle
		caRes, err := clusterClient.AuthClient.GetCertAuthorities(
			ctx, types.SPIFFECA, false,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		trustBundleBytes := &bytes.Buffer{}
		for _, ca := range caRes {
			for _, cert := range services.GetTLSCerts(ca) {
				// Values are already PEM encoded, so we just append to the buffer
				if _, err := trustBundleBytes.Write(cert); err != nil {
					return trace.Wrap(err, "writing trust bundle to buffer")
				}
			}
		}
		trustBundlePath := filepath.Join(c.outputDirectory, svidTrustBundlePEMPath)
		err = os.WriteFile(
			trustBundlePath,
			trustBundleBytes.Bytes(),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Fprintf(
			cf.Stdout(),
			"SVID %q issued. Files written to: \n - %s\n - %s\n - %s\n",
			x509Credential.SpiffeId,
			keyPath,
			svidPath,
			trustBundlePath,
		)

		return nil
	})
}

// svidCommands manages the SVID commands.
// Deprecated and being replaced by workloadIdentityCommands
type svidCommands struct {
	issue *svidIssueCommand
}

func newSVIDCommands(app *kingpin.Application) svidCommands {
	cmd := app.Command("svid", "Manage Teleport Workload Identity SVIDs.")
	cmds := svidCommands{
		issue: newSVIDIssueCommand(cmd),
	}
	return cmds
}

const (
	// Based on the default paths listed in
	// https://github.com/spiffe/spiffe-helper/blob/v0.7.0/README.md
	svidPEMPath            = "svid.pem"
	svidKeyPEMPath         = "svid_key.pem"
	svidTrustBundlePEMPath = "svid_bundle.pem"

	svidTypeX509 = "x509"
)

type svidIssueCommand struct {
	*kingpin.CmdClause
	svidSPIFFEIDPath string
	svidType         string
	svidDNSSANs      []string
	svidIPSANs       []string
	svidTTL          time.Duration
	outputDirectory  string
}

func newSVIDIssueCommand(parent *kingpin.CmdClause) *svidIssueCommand {
	cmd := &svidIssueCommand{
		CmdClause: parent.Command("issue", "Issue a SPIFFE SVID using Teleport Workload Identity and write it to a local directory."),
	}
	cmd.Arg("path", "Path to use for the SVID SPIFFE ID. Must have a preceding '/'.").
		Required().
		StringVar(&cmd.svidSPIFFEIDPath)
	cmd.Flag("type", "Type of the SVID to issue (x509). Defaults to x509.").
		Default(svidTypeX509).
		EnumVar(&cmd.svidType, svidTypeX509)
	cmd.Flag("output", "Path to the directory to write the SVID into.").
		Required().
		StringVar(&cmd.outputDirectory)
	cmd.Flag("dns-san", "DNS SANs to include in the SVID. By default, none are included.").
		StringsVar(&cmd.svidDNSSANs)
	cmd.Flag("ip-san", "IP SANs to include in the SVID. By default, none are included.").
		StringsVar(&cmd.svidIPSANs)
	cmd.Flag("svid-ttl", "Sets the time to live for the SVID.").
		Default("1h").
		DurationVar(&cmd.svidTTL)
	return cmd
}

func (c *svidIssueCommand) run(cf *CLIConf) error {
	ctx := cf.Context
	// Validate flags
	if c.svidType != svidTypeX509 {
		return trace.BadParameter("unsupported SVID type: %v", c.svidType)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	return client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		// Generate keypair to use in SVID
		privateKey, err := native.GenerateRSAPrivateKey()
		if err != nil {
			return trace.Wrap(err)
		}
		pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
		if err != nil {
			return trace.Wrap(err)
		}

		res, err := clusterClient.AuthClient.WorkloadIdentityServiceClient().
			SignX509SVIDs(ctx,
				&machineidv1pb.SignX509SVIDsRequest{
					Svids: []*machineidv1pb.SVIDRequest{
						{
							SpiffeIdPath: c.svidSPIFFEIDPath,
							PublicKey:    pubBytes,
							DnsSans:      c.svidDNSSANs,
							IpSans:       c.svidIPSANs,
							Ttl:          durationpb.New(c.svidTTL),
						},
					},
				},
			)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(res.Svids) != 1 {
			return trace.BadParameter("expected 1 SVID, got %v", len(res.Svids))
		}

		// Write private key
		privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return trace.Wrap(err)
		}
		keyPath := path.Join(c.outputDirectory, svidKeyPEMPath)
		err = os.WriteFile(
			keyPath,
			pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privBytes,
			}),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		// Write SVID
		svidPath := path.Join(c.outputDirectory, svidPEMPath)
		err = os.WriteFile(
			svidPath,
			pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: res.Svids[0].Certificate,
			}),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		// Write trust bundle
		caRes, err := clusterClient.AuthClient.GetCertAuthorities(
			ctx, types.SPIFFECA, false,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		trustBundleBytes := &bytes.Buffer{}
		for _, ca := range caRes {
			for _, cert := range services.GetTLSCerts(ca) {
				// Values are already PEM encoded, so we just append to the buffer
				if _, err := trustBundleBytes.Write(cert); err != nil {
					return trace.Wrap(err, "writing trust bundle to buffer")
				}
			}
		}
		trustBundlePath := path.Join(c.outputDirectory, svidTrustBundlePEMPath)
		err = os.WriteFile(
			trustBundlePath,
			trustBundleBytes.Bytes(),
			teleport.FileMaskOwnerOnly,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Fprintf(
			cf.Stdout(),
			"SVID %q issued. Files written to: \n - %s\n - %s\n - %s\n",
			res.Svids[0].SpiffeId,
			keyPath,
			svidPath,
			trustBundlePath,
		)

		return nil
	})
}
