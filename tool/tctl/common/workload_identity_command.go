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
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
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
	revocationsCrlCmd *kingpin.CmdClause

	revocationType    string
	revocationSerial  string
	revocationReason  string
	revocationExpiry  string
	revocationsFollow bool

	stdout io.Writer
}

// Initialize sets up the "tctl workload-identity" command.
func (c *WorkloadIdentityCommand) Initialize(
	app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config,
) {
	// TODO(noah): Remove the hidden flag once base functionality is released.
	cmd := app.Command(
		"workload-identity",
		"Manage Teleport Workload Identity.",
	).Hidden()

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
			"expiry",
			"Time that the revocation should expire, usually this should match the expiry time of the credential. This should be specified using RFC3339 e.g 2024-02-05T15:04:00Z. If unspecified, the time 1 week from now is used.").
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

	c.revocationsCrlCmd = revocationsCmd.Command("crl", "Fetch the signed CRL for existing revocations.")
	c.revocationsCrlCmd.Flag("follow", "Follow the stream of CRL updates.").BoolVar(&c.revocationsFollow)

	if c.stdout == nil {
		c.stdout = os.Stdout
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
	case c.revocationsCrlCmd.FullCommand():
		commandFunc = c.StreamRevocationsCrl
	case c.revocationsRmCmd.FullCommand():
		commandFunc = c.DeleteRevocation
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
	expiry := time.Now().Add(time.Hour * 24 * 7)
	if c.revocationExpiry != "" {
		expiry, err = time.Parse(time.RFC3339, c.revocationExpiry)
		if err != nil {
			return trace.Wrap(err, "parsing expiry time")
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
				RevokedAt: timestamppb.Now(),
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
					expiryTime.Sub(time.Now()).Truncate(time.Second).String(),
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

func (c *WorkloadIdentityCommand) StreamRevocationsCrl(
	ctx context.Context, client *authclient.Client,
) error {

	revocationsClient := client.WorkloadIdentityRevocationServiceClient()

	req := &workloadidentityv1pb.StreamSignedCRLRequest{}
	stream, err := revocationsClient.StreamSignedCRL(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "X509 CRL",
			Bytes: res.Crl,
		})
		fmt.Println(string(pemData))

		if !c.revocationsFollow {
			return nil
		}
	}
}
