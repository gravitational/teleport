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
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accessgraph"
	secretsscannerclient "github.com/gravitational/teleport/lib/secretsscanner/client"
	"github.com/gravitational/teleport/lib/secretsscanner/scan"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	dtnative "github.com/gravitational/teleport/lib/devicetrust/native"
)

type scanCommand struct {
	keys *scanKeysCommand
}

func newScanCommand(app *kingpin.Application) scanCommand {
	scan := app.Command("scan", "Scan secrets in the local machine and reports findings to Teleport.")
	cmd := scanCommand{
		keys: newScanKeysCommand(scan),
	}
	return cmd
}

type scanKeysCommand struct {
	*kingpin.CmdClause
	dirs []string
	ca   string
	out  io.Writer
}

func newScanKeysCommand(parent *kingpin.CmdClause) *scanKeysCommand {
	c := &scanKeysCommand{CmdClause: parent.Command("keys", "Scan SSH Private Keys in the local machine and reports findings to Teleport.")}
	c.Flag("dirs", "Directory to scan for SSH Private Keys").Default("/").StringsVar(&c.dirs)
	return c
}

func (c *scanKeysCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	deviceCred, err := dtnative.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err, "device not enrolled")
	}

	fmt.Printf("Device trust credentials found.\nScanning %s.\n", strings.Join(c.dirs, ", "))

	scanner, err := scan.NewScanner(scan.ScannerConfig{
		Dirs: c.dirs,
		Log:  slog.Default(),
	})
	if err != nil {
		return trace.Wrap(err, "failed to create scanner")
	}

	privateKeys := scanner.ScanPrivateKeys(
		ctx,
		deviceCred.Id,
	)

	printPrivateKeys(privateKeys)

	client, err := secretsscannerclient.NewSecretsScannerServiceClient(
		ctx,
		secretsscannerclient.ClientConfig{
			ProxyServer: cf.Proxy,
			Insecure:    cf.InsecureSkipVerify,
			Log:         slog.Default(),
		})

	if err := authenticateAndReportPrivateKeys(ctx, client, privateKeys); err != nil {
		return trace.Wrap(err, "failed to report private keys")
	}

	fmt.Printf("Reported %d SSH fingerprints to Teleport.\n", len(privateKeys))

	return nil
}

func printPrivateKeys(privateKeys []scan.SSHPrivateKey) {
	if len(privateKeys) == 0 {
		fmt.Println("No SSH private keys found.")
		return
	}

	fmt.Println("SSH private keys found:")
	for _, pk := range privateKeys {
		path, key := pk.Path, pk.Key
		fmt.Printf("- SHA256 fingerprint: %q (mode: %s) at %s\n",
			key.Spec.PublicKeyFingerprint,
			accessgraph.DescribePublicKeyMode(key.Spec.PublicKeyMode),
			path,
		)
	}
}

// authenticateAndReportPrivateKeys reports the private keys to the Teleport server.
// It creates a client to the secrets scanner service, runs the device authentication ceremony, and reports the private keys.
// It returns an error if the client cannot be created or if the private keys cannot be reported.
func authenticateAndReportPrivateKeys(ctx context.Context, client secretsscannerclient.Client, privateKeys []scan.SSHPrivateKey) error {
	const notImplementedMsg = "Teleport version does not support secrets scanning. Please upgrade Teleport to the latest version."
	stream, err := client.ReportSecrets(ctx)
	if trace.IsNotImplemented(err) {
		return trace.NotImplemented(notImplementedMsg)
	} else if err != nil {
		return trace.Wrap(err, "failed to create client")
	}

	if err := runAssertionCeremony(ctx, stream); trace.IsNotImplemented(err) {
		return trace.NotImplemented(notImplementedMsg)
	} else if err != nil {
		return trace.Wrap(err, "failed to run assertion ceremony")
	}

	getProtoMsgFromSSHPrivateKeys := func(pks []scan.SSHPrivateKey) []*accessgraphsecretsv1pb.PrivateKey {
		keys := make([]*accessgraphsecretsv1pb.PrivateKey, 0, len(pks))
		for _, pk := range pks {
			keys = append(keys, pk.Key)
		}
		return keys
	}

	const batchSize = 400
	for i := 0; len(privateKeys) > i; i += batchSize {
		end := i + batchSize
		if end > len(privateKeys) {
			end = len(privateKeys)
		}

		keysSlice := getProtoMsgFromSSHPrivateKeys(privateKeys[i:end])
		if err := stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_PrivateKeys{
				PrivateKeys: &accessgraphsecretsv1pb.ReportPrivateKeys{
					Keys: keysSlice,
				},
			},
		}); err != nil {
			return trace.Wrap(err, "failed to send private keys")
		}
	}

	// Inform the server that there are no more private keys to report.
	if err := stream.CloseSend(); err != nil {
		return trace.Wrap(err, "failed to close send")
	}

	// Wait for the server to acknowledge the report.
	if _, err := stream.Recv(); err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err, "error closing the stream")
	}
	return nil
}

// runAssertionCeremony runs the device assertion ceremony.
func runAssertionCeremony(ctx context.Context, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) error {
	// Create a new device authentication ceremony.
	assertCeremony, err := assert.NewCeremony()
	if err != nil {
		return trace.Wrap(err, "failed to create assertCeremony")
	}

	// Run the device authentication ceremony.
	// If successful, the device will be authenticated and the device can report its secrets.
	err = assertCeremony.Run(
		ctx,
		reportToAssertStreamAdapter{stream},
	)
	return trace.Wrap(err, "failed to run device authentication ceremony")
}

// reportToAssertStreamAdapter is a wrapper for the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] that implements the
// [assert.AssertDeviceClientStream] interface.
//
// This adapter allows the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] to be used with the [assert.AssertDeviceClientStream]
// interface, which is essential for the [assert.Ceremony] in executing the device authentication process. It handles the extraction and insertion
// of device assertion messages from and into the [accessgraphsecretsv1pb.ReportSecretsRequest] and [accessgraphsecretsv1pb.ReportSecretsResponse] messages.
type reportToAssertStreamAdapter struct {
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient
}

func (s reportToAssertStreamAdapter) Send(request *devicepb.AssertDeviceRequest) error {
	return trace.Wrap(
		s.stream.Send(
			&accessgraphsecretsv1pb.ReportSecretsRequest{
				Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
					DeviceAssertion: request,
				},
			},
		),
	)
}

func (s reportToAssertStreamAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	in, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if in.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("unsupported response type: expected DeviceAssertion, got %T", in.Payload)
	}

	return in.GetDeviceAssertion(), nil
}
