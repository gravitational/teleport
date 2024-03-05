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
	"os"
	"path"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityCommands struct {
	issue *workloadIdentityIssueCommand
}

func newWorkloadIdentityCommands(app *kingpin.Application) workloadIdentityCommands {
	cmd := app.Command("workloadid", "Manage Teleport Workload Identity for SPIFFE.")
	cmds := workloadIdentityCommands{
		issue: newWorkloadIdentityIssueCommand(cmd),
	}
	return cmds
}

const (
	// Based on the default paths listed in
	// https://github.com/spiffe/spiffe-helper/blob/main/README.md
	svidPEMPath            = "svid.pem"
	svidKeyPEMPath         = "svid_key.pem"
	svidTrustBundlePEMPath = "svid_bundle.pem"

	svidTypeX509 = "x509"
)

type workloadIdentityIssueCommand struct {
	*kingpin.CmdClause
	svidPath        string
	svidType        string
	svidDNSSANs     []string
	svidIPSANs      []string
	svidTTL         time.Duration
	outputDirectory string
}

func newWorkloadIdentityIssueCommand(parent *kingpin.CmdClause) *workloadIdentityIssueCommand {
	cmd := &workloadIdentityIssueCommand{
		CmdClause: parent.Command("issue", "Issue a SPIFFE SVID"),
	}
	cmd.Arg("path", "Path to include the the SPIFFE ID. Must start with a /").
		Required().
		StringVar(&cmd.svidPath)
	cmd.Flag("type", "Type of the SVID to issue (x509)").
		Default(svidTypeX509).
		EnumVar(&cmd.svidType, svidTypeX509)
	cmd.Flag("output", "Path to directory to write SVID into").
		Required().
		StringVar(&cmd.outputDirectory)
	cmd.Flag("dns-san", "DNS SANs to include in the SVID").
		StringsVar(&cmd.svidDNSSANs)
	cmd.Flag("ip-san", "IP SANs to include in the SVID").
		StringsVar(&cmd.svidIPSANs)
	cmd.Flag("ttl", "Time to live for the SVID").
		Default("1h").
		DurationVar(&cmd.svidTTL)
	return cmd
}

func (c *workloadIdentityIssueCommand) run(cf *CLIConf) error {
	ctx := cf.Context
	// Validate flags
	if c.svidType != svidTypeX509 {
		return trace.BadParameter("unsupported SVID type: %v", c.svidType)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rootAuthClient.Close()

	privateKey, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return trace.Wrap(err)
	}

	res, err := rootAuthClient.WorkloadIdentityServiceClient().SignX509SVIDs(ctx,
		&machineidv1pb.SignX509SVIDsRequest{
			Svids: []*machineidv1pb.SVIDRequest{
				{
					SpiffeIdPath: c.svidPath,
					PublicKey:    pubBytes,
					DnsSans:      c.svidDNSSANs,
					IpSans:       c.svidIPSANs,
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
	err = os.WriteFile(
		path.Join(c.outputDirectory, svidKeyPEMPath),
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
	err = os.WriteFile(
		path.Join(c.outputDirectory, svidPEMPath),
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
	caRes, err := rootAuthClient.GetCertAuthorities(
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
	err = os.WriteFile(
		path.Join(c.outputDirectory, svidTrustBundlePEMPath),
		trustBundleBytes.Bytes(),
		teleport.FileMaskOwnerOnly,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil

}
