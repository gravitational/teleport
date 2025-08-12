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
	"log/slog"
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

	revocationsAddCmd    *kingpin.CmdClause
	revocationsRmCmd     *kingpin.CmdClause
	revocationsLsCmd     *kingpin.CmdClause
	revocationsCrlCmd    *kingpin.CmdClause
	revocationsCRLFollow bool
	revocationsCRLOut    string

	revocationType   string
	revocationSerial string
	revocationReason string
	revocationExpiry string

	overridesSignCmd   *kingpin.CmdClause
	overridesSignMode  workloadidentityv1pb.CSRCreationMode
	overridesSignForce bool

	overridesCreateCmd        *kingpin.CmdClause
	overridesCreateName       string
	overridesCreateForce      bool
	overridesCreateFullchains []string
	overridesCreateDryRun     bool

	now func() time.Time

	stdout io.Writer
	stderr io.Writer
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

	c.revocationsCrlCmd = revocationsCmd.Command(
		"crl", "Fetch the signed CRL for existing revocations.",
	)
	c.revocationsCrlCmd.Flag(
		"follow", "Follow the stream of CRL updates.",
	).BoolVar(&c.revocationsCRLFollow)
	c.revocationsCrlCmd.Flag(
		"out", "Path to write the CRL as a file to. If unspecified, STDOUT will be used.",
	).StringVar(&c.revocationsCRLOut)

	overridesCmd := cmd.Command("x509-issuer-overrides", "Manage X.509 overrides.")

	c.overridesSignCmd = overridesCmd.Command("sign-csrs", "Sign CSRs with the SPIFFE X.509 CA keys.")
	var overridesSignMode string
	c.overridesSignCmd.
		Flag(
			"creation-mode",
			"How the attributes of the issuer are encoded in the CSR: \"same\", \"empty\".",
		).
		Default("same").
		Action(parseProtobufEnum(
			&overridesSignMode,
			&c.overridesSignMode,
			map[string]workloadidentityv1pb.CSRCreationMode{
				"empty": workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_EMPTY,
				"same":  workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_SAME,
			},
		)).
		StringVar(&overridesSignMode)
	c.overridesSignMode = workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_SAME
	c.overridesSignCmd.
		Flag("force", "Attempt to sign as many CSRs as possible even in the presence of errors.").
		Short('f').
		BoolVar(&c.overridesSignForce)

	c.overridesCreateCmd = overridesCmd.Command("create", "Create an issuer override from the given certificate chains.")
	c.overridesCreateCmd.
		Flag("force", "Overwrite the existing override if it exists.").
		Short('f').
		BoolVar(&c.overridesCreateForce)
	c.overridesCreateCmd.
		Flag("dry-run", "Print the workload_identity_x509_issuer_override that would have been created, without actually creating it.").
		BoolVar(&c.overridesCreateDryRun)
	c.overridesCreateCmd.
		Flag("name", "The name of the override resource to write.").
		Default("default").
		StringVar(&c.overridesCreateName)
	c.overridesCreateCmd.
		Arg("fullchain.pem", "PEM files containing an issuer and its optional chain each.").
		Required().
		ExistingFilesVar(&c.overridesCreateFullchains)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	if c.stderr == nil {
		c.stderr = os.Stderr
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
	case c.revocationsCrlCmd.FullCommand():
		commandFunc = c.StreamCRL
	case c.overridesSignCmd.FullCommand():
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

func (c *WorkloadIdentityCommand) StreamCRL(
	ctx context.Context, client *authclient.Client,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	revocationsClient := client.WorkloadIdentityRevocationServiceClient()

	req := &workloadidentityv1pb.StreamSignedCRLRequest{}
	stream, err := revocationsClient.StreamSignedCRL(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	write := func(data []byte) error {
		_, err := c.stdout.Write(data)
		return trace.Wrap(err)
	}
	if c.revocationsCRLOut != "" {
		write = func(data []byte) error {
			err := os.WriteFile(c.revocationsCRLOut, data, 0o644)
			if err != nil {
				return trace.Wrap(err)
			}
			slog.InfoContext(ctx, "Successfully wrote updated CRL", "path", c.revocationsCRLOut)
			return nil
		}
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			if trace.IsNotImplemented(err) {
				slog.ErrorContext(ctx, "Server does not support X509 CRL functionality")
			}
			return trace.Wrap(err)
		}
		slog.InfoContext(ctx, "Received CRL from server")
		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "X509 CRL",
			Bytes: res.Crl,
		})
		if err := write(pemData); err != nil {
			return trace.Wrap(err, "writing CRL pem")
		}

		// If --follow has not been specified, exit.
		if !c.revocationsCRLFollow {
			return nil
		}
	}
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
			return trace.BadParameter("got no certificates from fullchain PEM file %q", p)
		}
		overrides = append(overrides, certs)
	}

	// Ensure that the user has not provided the Root CA - we only want them to
	// provide the intermediates that chain to the root CA. If they provide the
	// root CA, then workloads will end up needlessly distributing the root CA
	// to validators.
	for _, chain := range overrides {
		for _, cert := range chain {
			// If the issuer and subject are the same, then this is a
			// "self-signed" certificate.
			if bytes.Equal(cert.RawSubject, cert.RawIssuer) {
				slog.WarnContext(
					ctx,
					"The provided certificate chain contains a root certificate when it should only contain the issuing CA and the intermediate CAs necessary to chain the issuing CA to the root CA. Remove the root certificate from the certificate file.",
					"cert_subject", cert.Subject.String(),
				)
			}
		}
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
			return trace.BadParameter("override in fullchain PEM file %q does not match any issuer", c.overridesCreateFullchains[i])
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

	override := &workloadidentityv1pb.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: c.overridesCreateName,
		},
		Spec: &workloadidentityv1pb.X509IssuerOverrideSpec{
			Overrides: pbOverrides,
		},
	}

	if c.overridesCreateDryRun {
		fmt.Fprintln(c.stderr, "Dry run mode enabled, the following override would have been created:")
		if err := utils.WriteYAML(c.stdout, types.ProtoResource153ToLegacy(override)); err != nil {
			return trace.Wrap(err, "failed to marshal override")
		}
		return nil
	}

	if c.overridesCreateForce {
		if _, err := oclt.UpsertX509IssuerOverride(ctx, &workloadidentityv1pb.UpsertX509IssuerOverrideRequest{
			X509IssuerOverride: override,
		}); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := oclt.CreateX509IssuerOverride(ctx, &workloadidentityv1pb.CreateX509IssuerOverrideRequest{
			X509IssuerOverride: override,
		}); err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.Wrap(err, "override already exists, use the --force option to overwrite it")
			}
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(c.stdout,
		"Written "+types.KindWorkloadIdentityX509IssuerOverride+"; to check, run tctl get "+types.KindWorkloadIdentityX509IssuerOverride+"/%v\n",
		c.overridesCreateName,
	)
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
	type result struct {
		issuer *x509.Certificate
		csr    *x509.CertificateRequest
		err    error
	}
	results := make([]result, 0, len(keypairs))
	for _, kp := range keypairs {
		issuer, err := tlsca.ParseCertificatePEM(kp.Cert)
		if err != nil {
			return trace.Wrap(err)
		}
		resp, err := oclt.SignX509IssuerCSR(ctx, &workloadidentityv1pb.SignX509IssuerCSRRequest{
			Issuer:          issuer.Raw,
			CsrCreationMode: c.overridesSignMode,
		})
		if err != nil {
			if !c.overridesSignForce {
				return trace.Wrap(err)
			}
			results = append(results, result{
				issuer: issuer,
				csr:    nil,
				err:    err,
			})
			continue
		}
		csr, err := x509.ParseCertificateRequest(resp.GetCsr())
		if err != nil {
			return trace.Wrap(err)
		}
		results = append(results, result{
			issuer: issuer,
			csr:    csr,
			err:    nil,
		})
	}

	var errs []error
	for _, r := range results {
		fmt.Fprintln(c.stdout, r.issuer.Subject)
		if r.err != nil {
			errs = append(errs, r.err)
			fmt.Fprintln(c.stdout, r.err.Error())
			continue
		}
		_ = pem.Encode(c.stdout, &pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: r.csr.Raw,
		})
	}
	return trace.Wrap(trace.NewAggregate(errs...), "some or all signature requests failed")
}
