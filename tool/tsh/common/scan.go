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
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	dtnative "github.com/gravitational/teleport/lib/devicetrust/native"
	secretsscannerclient "github.com/gravitational/teleport/lib/secretsscanner/client"
	secretsreporter "github.com/gravitational/teleport/lib/secretsscanner/reporter"
	secretsscanner "github.com/gravitational/teleport/lib/secretsscanner/scanner"
)

type scanCommand struct {
	keys *scanKeysCommand
}

func newScanCommand(app *kingpin.Application) scanCommand {
	scan := app.Command("scan", "Scan the local machine for Secrets and report findings to Teleport.")
	cmd := scanCommand{
		keys: newScanKeysCommand(scan),
	}
	return cmd
}

type scanKeysCommand struct {
	*kingpin.CmdClause
	dirs      []string
	skipPaths []string
}

func newScanKeysCommand(parent *kingpin.CmdClause) *scanKeysCommand {
	c := &scanKeysCommand{CmdClause: parent.Command("keys", "Scan the local machine for SSH private keys and report findings to Teleport.")}
	c.Flag("dirs", "Directories to scan.").Default(defaultDirValues()).StringsVar(&c.dirs)
	c.Flag("skip-paths", "Paths to directories or files to skip. Supports for matching patterns.").StringsVar(&c.skipPaths)
	return c
}

func defaultDirValues() string {
	switch runtime.GOOS {
	case constants.LinuxOS:
		return "/home/"
	case constants.DarwinOS:
		return "/Users/"
	case constants.WindowsOS:
		return "C:\\Users\\"
	default:
		return "/"
	}
}

func (c *scanKeysCommand) run(cf *CLIConf) error {
	if len(c.dirs) == 0 {
		return trace.BadParameter("no directories to scan")
	}

	if cf.Proxy == "" {
		return trace.BadParameter("proxy address is required")
	}

	ctx := cf.Context

	deviceCred, err := dtnative.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err, "device not enrolled")
	}

	dirs := splitCommaSeparatedSlice(c.dirs)
	fmt.Printf("Device trust credentials found.\nScanning %s.\n", strings.Join(dirs, ", "))

	scanner, err := secretsscanner.New(secretsscanner.Config{
		Dirs:      dirs,
		SkipPaths: splitCommaSeparatedSlice(c.skipPaths),
		Log:       slog.Default(),
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
	if err != nil {
		return trace.Wrap(err, "failed to create client")
	}

	reporter, err := secretsreporter.New(
		secretsreporter.Config{
			Client: client,
			Log:    slog.Default(),
			AssertCeremonyBuilder: func() (*assert.Ceremony, error) {
				return assert.NewCeremony()
			},
		},
	)
	if err != nil {
		return trace.Wrap(err, "failed to create reporter")
	}

	if err := reporter.ReportPrivateKeys(ctx, collectPrivateKeys(privateKeys)); trace.IsNotImplemented(err) {
		return handleUnimplementedError(ctx, err, *cf)
	} else if err != nil {
		return trace.Wrap(err, "failed to report private keys")
	}

	fmt.Printf("Reported %d SSH fingerprints to Teleport.\n", len(privateKeys))

	return nil
}

func printPrivateKeys(privateKeys []secretsscanner.SSHPrivateKey) {
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

func collectPrivateKeys(privateKeys []secretsscanner.SSHPrivateKey) []*accessgraphsecretsv1pb.PrivateKey {
	keys := make([]*accessgraphsecretsv1pb.PrivateKey, 0, len(privateKeys))
	for _, pk := range privateKeys {
		keys = append(keys, pk.Key)
	}
	return keys
}

func splitCommaSeparatedSlice(s []string) []string {
	var result []string
	for _, entry := range s {
		for split := range strings.SplitSeq(entry, ",") {
			result = append(result, strings.TrimSpace(split))
		}
	}
	return result
}
