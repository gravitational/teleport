// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// WorkloadIdentityCommand is a group of commands pertaining to Teleport
// Workload Identity.
type WorkloadIdentityCommand struct {
	format               string
	workloadIdentityName string

	listCmd *kingpin.CmdClause
	rmCmd   *kingpin.CmdClause

	revocationsAddCmd *kingpin.CmdClause
	revocationsRmCmd  *kingpin.CmdClause
	revocationsLsCmd  *kingpin.CmdClause

	revocationType   string
	revocationSerial string
	revocationReason string
	revocationExpiry string

	overridesSignCSRsCmd *kingpin.CmdClause
	overridesCreateCmd   *kingpin.CmdClause

	overridesCreateFullchains []string

	now func() time.Time

	stdout io.Writer
}

// Initialize sets up the "tctl workload-identity" command.
func (c *WorkloadIdentityCommand) Initialize(
	app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config,
) {
	cmd := app.Command(
		"workload-identity",
		"Manage Teleport Workload Identity.",
	)

	c.listCmd = cmd.Command(
		"ls",
		"List workload identity configurations.",
	)
	c.listCmd.
		Flag(
			"format",
			"Output format, 'text' or 'json'",
		).
		Hidden().
		Default(teleport.Text).
		EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.rmCmd = cmd.Command(
		"rm",
		"Delete a workload identity configuration.",
	)
	c.rmCmd.
		Arg("name", "Name of the workload identity configuration to delete.").
		Required().
		StringVar(&c.workloadIdentityName)

	revocationsCmd := cmd.Command("revocations", "Manage workload identity revocations.")
	c.revocationsAddCmd = revocationsCmd.Command("add", "Create a new revocation.")
	c.revocationsAddCmd.Flag("serial", "Serial number of the certificate to revoke.").
		Required().
		StringVar(&c.revocationSerial)
	c.revocationsAddCmd.Flag("type", "Type of credential to revoke (x509)").
		Required().
		EnumVar(&c.revocationType, "x509")
	c.revocationsAddCmd.Flag("reason", "Reason for revocation.").
		Required().
		StringVar(&c.revocationReason)
	c.revocationsAddCmd.
		Flag(
			"expires-at",
			"Time that the revocation should expire, usually this should match the expiry time of the credential. This should be specified using RFC3339 e.g '2024-02-05T15:04:00Z'. If unspecified, the time 1 week from now is used.").
		StringVar(&c.revocationExpiry)

	c.revocationsRmCmd = revocationsCmd.Command("rm", "Delete a revocation.")
	c.revocationsRmCmd.Flag("serial", "Serial number of the certificate to remove the revocation for.").Required().StringVar(&c.revocationSerial)
	c.revocationsRmCmd.Flag("type", "Type of credential to remove the revocation for (x509).").Required().EnumVar(&c.revocationType, "x509")

	c.revocationsLsCmd = revocationsCmd.Command("ls", "List revocations.")
	c.revocationsLsCmd.
		Flag(
			"format",
			"Output format, 'text' or 'json'",
		).
		Hidden().
		Default(teleport.Text).
		EnumVar(&c.format, teleport.Text, teleport.JSON)

	overridesCmd := cmd.Command("unstable-x509-overrides", "Manage X.509 overrides.")

	c.overridesSignCSRsCmd = overridesCmd.Command("sign-csrs", "Sign CSRs with the SPIFFE X.509 CA keys.")

	c.overridesCreateCmd = overridesCmd.Command("create-default-override", "Create a default issuer override from certificate chains.")
	c.overridesCreateCmd.Arg("fullchain.pem", "Issuer and optional chain.").Required().ExistingFilesVar(&c.overridesCreateFullchains)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	if c.now == nil {
		c.now = time.Now
	}
}

// TryRun attempts to run subcommands.
func (c *WorkloadIdentityCommand) TryRun(
	ctx context.Context, cmd string, clientFunc commonclient.InitFunc,
) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.listCmd.FullCommand():
		commandFunc = c.ListWorkloadIdentities
	case c.rmCmd.FullCommand():
		commandFunc = c.DeleteWorkloadIdentity
	case c.revocationsAddCmd.FullCommand():
		commandFunc = c.AddRevocation
	case c.revocationsLsCmd.FullCommand():
		commandFunc = c.ListRevocations
	case c.revocationsRmCmd.FullCommand():
		commandFunc = c.DeleteRevocation
	case c.overridesSignCSRsCmd.FullCommand():
		commandFunc = c.runOverridesSignCSRs
	case c.overridesCreateCmd.FullCommand():
		commandFunc = c.runOverridesCreate
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

func (c *WorkloadIdentityCommand) DeleteWorkloadIdentity(
	ctx context.Context,
	client *authclient.Client,
) error {
	workloadIdentityClient := client.WorkloadIdentityResourceServiceClient()
	_, err := workloadIdentityClient.DeleteWorkloadIdentity(
		ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
			Name: c.workloadIdentityName,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(
		c.stdout,
		"Workload Identity %q deleted successfully.\n",
		c.workloadIdentityName,
	)

	return nil
}

// ListWorkloadIdentities writes a listing of the WorkloadIdentity resources
func (c *WorkloadIdentityCommand) ListWorkloadIdentities(
	ctx context.Context, client *authclient.Client,
) error {
	workloadIdentityClient := client.WorkloadIdentityResourceServiceClient()
	var workloadIdentities []*workloadidentityv1pb.WorkloadIdentity
	req := &workloadidentityv1pb.ListWorkloadIdentitiesRequest{}
	for {
		resp, err := workloadIdentityClient.ListWorkloadIdentities(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		workloadIdentities = append(
			workloadIdentities, resp.WorkloadIdentities...,
		)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if c.format == teleport.Text {
		if len(workloadIdentities) == 0 {
			fmt.Fprintln(c.stdout, "No workload identities configured")
			return nil
		}
		t := asciitable.MakeTable([]string{"Name", "SPIFFE ID"})
		for _, u := range workloadIdentities {
			t.AddRow([]string{
				u.GetMetadata().GetName(), u.GetSpec().GetSpiffe().GetId(),
			})
		}
		fmt.Fprintln(c.stdout, t.AsBuffer().String())
	} else {
		err := utils.WriteJSONArray(c.stdout, workloadIdentities)
		if err != nil {
			return trace.Wrap(err, "failed to marshal workload identities")
		}
	}
	return nil
}

func normalizeCertificateSerial(str string) (string, error) {
	stripped := strings.ReplaceAll(str, ":", "")
	value := new(big.Int)
	_, ok := value.SetString(stripped, 16)
	if !ok {
		return "", trace.BadParameter("invalid serial number")
	}
	return value.Text(16), nil
}

// AddRevocation creates a new revocation. Currently, only the X509 type is
// supported.
func (c *WorkloadIdentityCommand) AddRevocation(
	ctx context.Context,
	client *authclient.Client,
) error {
	if c.revocationType != "x509" {
		return trace.BadParameter("only x509 revocations are supported")
	}
	normalizedSerial, err := normalizeCertificateSerial(c.revocationSerial)
	if err != nil {
		return trace.Wrap(err, "normalizing serial")
	}

	// Default to a weeks TTL as this aligns with the longest possible TTL of
	// an issued credential.
	expiry := c.now().Add(time.Hour * 24 * 7)
	if c.revocationExpiry != "" {
		expiry, err = time.Parse(time.RFC3339, c.revocationExpiry)
		if err != nil {
			return trace.Wrap(err, "parsing expires-at time")
		}
	}

	revocationClient := client.WorkloadIdentityRevocationServiceClient()
	_, err = revocationClient.CreateWorkloadIdentityX509Revocation(ctx, &workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
		WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    normalizedSerial,
				Expires: timestamppb.New(expiry),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    c.revocationReason,
				RevokedAt: timestamppb.New(c.now()),
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "creating revocation")
	}

	fmt.Fprintf(
		c.stdout,
		"Revocation for the X509 certificate with serial %s created\n",
		normalizedSerial,
	)

	return nil
}

// DeleteRevocation deletes a revocation. Currently, only the X509 type is
// supported.
func (c *WorkloadIdentityCommand) DeleteRevocation(
	ctx context.Context,
	client *authclient.Client,
) error {
	if c.revocationType != "x509" {
		return trace.BadParameter("only x509 revocations are supported")
	}
	normalizedSerial, err := normalizeCertificateSerial(c.revocationSerial)
	if err != nil {
		return trace.Wrap(err, "normalizing serial")
	}

	revocationClient := client.WorkloadIdentityRevocationServiceClient()
	_, err = revocationClient.DeleteWorkloadIdentityX509Revocation(ctx, &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
		Name: normalizedSerial,
	})
	if err != nil {
		return trace.Wrap(err, "deleting revocation")
	}

	fmt.Fprintf(
		c.stdout,
		"Revocation for the X509 certificate with serial %s deleted\n",
		normalizedSerial,
	)

	return nil
}

// ListRevocations lists the existing X509 revocations, and can display them as
// a table or as YAML.
func (c *WorkloadIdentityCommand) ListRevocations(
	ctx context.Context, client *authclient.Client,
) error {
	revocationsClient := client.WorkloadIdentityRevocationServiceClient()
	var revocations []*workloadidentityv1pb.WorkloadIdentityX509Revocation
	req := &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{}
	for {
		resp, err := revocationsClient.ListWorkloadIdentityX509Revocations(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		revocations = append(
			revocations, resp.WorkloadIdentityX509Revocations...,
		)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if c.format == teleport.Text {
		if len(revocations) == 0 {
			fmt.Fprintln(c.stdout, "No revocations configured")
			return nil
		}
		t := asciitable.MakeTable([]string{"Type", "Serial", "Revoked At", "Expires At", "Reason"})
		for _, u := range revocations {
			expiryTime := u.GetMetadata().GetExpires().AsTime()
			t.AddRow([]string{
				"x509",
				u.GetMetadata().GetName(),
				u.GetSpec().GetRevokedAt().AsTime().Format(time.RFC3339),
				fmt.Sprintf(
					"%s (%s)",
					expiryTime.Format(time.RFC3339),
					expiryTime.Sub(c.now()).Truncate(time.Second).String(),
				),
				u.GetSpec().GetReason(),
			})
		}
		fmt.Fprintln(c.stdout, t.AsBuffer().String())
	} else {
		converted := []types.Resource{}
		for _, resource := range revocations {
			converted = append(converted, types.ProtoResource153ToLegacy(resource))
		}
		err := utils.WriteJSONArray(c.stdout, converted)
		if err != nil {
			return trace.Wrap(err, "failed to marshal revocations")
		}
	}
	return nil
}

func (c *WorkloadIdentityCommand) runOverridesCreate(ctx context.Context, client *authclient.Client) error {
	oclt := client.WorkloadIdentityX509OverridesClient()

	overrides := make([][]*x509.Certificate, 0, len(c.overridesCreateFullchains))
	for _, p := range c.overridesCreateFullchains {
		f, err := os.ReadFile(p)
		if err != nil {
			return trace.Wrap(err)
		}
		certs, err := tlsca.ParseCertificatePEMs(f)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(certs) < 1 {
			return trace.BadParameter("got no certificates from fullchain.pem file %q", p)
		}
		overrides = append(overrides, certs)
	}

	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	const loadSigningKeysFalse = false
	ca, err := client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: clusterName,
	}, loadSigningKeysFalse)
	if err != nil {
		return trace.Wrap(err)
	}

	keypairs := ca.GetTrustedTLSKeyPairs()
	if len(overrides) != len(keypairs) {
		return trace.BadParameter("expected %v override(s), got %v", len(keypairs), len(overrides))
	}

	caCerts := make([]*x509.Certificate, 0, len(keypairs))
	for _, keypair := range keypairs {
		caCert, err := tlsca.ParseCertificatePEM(keypair.Cert)
		if err != nil {
			return trace.Wrap(err)
		}
		caCerts = append(caCerts, caCert)
	}

	for i, override := range overrides {
		if !slices.ContainsFunc(caCerts, func(caCert *x509.Certificate) bool {
			return bytes.Equal(override[0].RawSubjectPublicKeyInfo, caCert.RawSubjectPublicKeyInfo)
		}) {
			return trace.BadParameter("override in fullchain.pem %q does not match any issuer", c.overridesCreateFullchains[i])
		}
	}

	pbOverrides := make([]*workloadidentityv1pb.X509IssuerOverrideSpec_Override, 0, len(overrides))
	for _, override := range overrides {
		chainDer := make([][]byte, 0, len(override))
		for _, cert := range override {
			chainDer = append(chainDer, cert.Raw)
		}
		pbOverrides = append(pbOverrides, &workloadidentityv1pb.X509IssuerOverrideSpec_Override{
			Issuer: chainDer[0],
			Chain:  chainDer,
		})
	}

	if _, err := oclt.CreateX509IssuerOverride(ctx, &workloadidentityv1pb.CreateX509IssuerOverrideRequest{
		X509IssuerOverride: &workloadidentityv1pb.X509IssuerOverride{
			Kind:    types.KindWorkloadIdentityX509IssuerOverride,
			SubKind: "",
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "default",
			},
			Spec: &workloadidentityv1pb.X509IssuerOverrideSpec{
				Overrides: pbOverrides,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(c.stdout, "Created default workload_identity_x509_issuer_override; to check, run: tctl get workload_identity_x509_issuer_override/default")
	return nil
}

func (c *WorkloadIdentityCommand) runOverridesSignCSRs(ctx context.Context, client *authclient.Client) error {
	oclt := client.WorkloadIdentityX509OverridesClient()

	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	const loadSigningKeysFalse = false
	ca, err := client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: clusterName,
	}, loadSigningKeysFalse)
	if err != nil {
		return trace.Wrap(err)
	}

	keypairs := ca.GetTrustedTLSKeyPairs()
	csrs := make([]*x509.CertificateRequest, 0, len(keypairs))
	for _, kp := range keypairs {
		block, _ := pem.Decode(kp.Cert)
		if block == nil {
			return trace.BadParameter("failed to decode PEM block in SPIFFE CA")
		}
		resp, err := oclt.SignX509IssuerCSR(ctx, &workloadidentityv1pb.SignX509IssuerCSRRequest{Issuer: block.Bytes})
		if err != nil {
			return trace.Wrap(err)
		}
		csr, err := x509.ParseCertificateRequest(resp.GetCsr())
		if err != nil {
			return trace.Wrap(err)
		}
		csrs = append(csrs, csr)
	}
	for _, csr := range csrs {
		fmt.Println(csr.Subject)
		_ = pem.Encode(c.stdout, &pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: csr.Raw,
		})
	}
	return nil
}
