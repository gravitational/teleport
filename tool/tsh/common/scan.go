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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/lib/accessgraph/secretsservice"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

type scanCommands struct {
	keys *scanKeysCommand
}

func newScanCommand(app *kingpin.Application) scanCommands {
	kube := app.Command("scan", "Scan secrets in workforce fleet and report to Teleport")
	cmds := scanCommands{
		keys: newScanKeysCommand(kube),
	}
	return cmds
}

type scanKeysCommand struct {
	*kingpin.CmdClause
	dirs []string
	ca   string
	out  io.Writer
}

func newScanKeysCommand(parent *kingpin.CmdClause) *scanKeysCommand {
	c := &scanKeysCommand{CmdClause: parent.Command("keys", "Scan SSH Private Keys in workforce fleet and report to Teleport")}
	c.Flag("dirs", "Directory to scan for SSH Private Keys").Default("/").StringsVar(&c.dirs)
	return c
}

func (c *scanKeysCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	client, err := secretsservice.NewSecretsScannerServiceClient(
		ctx,
		secretsservice.ClientConfig{
			ProxyServer: cf.Proxy,
			Insecure:    cf.InsecureSkipVerify,
			Log:         slog.Default(),
		})

	deviceCred, err := native.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err, "device not enrolled")
	}

	fmt.Fprintf(os.Stdout, "Device trust credentials found.\n")
	fmt.Fprintf(os.Stdout, "Scanning %s.\n", strings.Join(c.dirs, ", "))

	keys := c.scan(ctx, deviceCred.Id)
	c.print(os.Stdout, keys)

	fmt.Fprintf(os.Stdout, "Creating Teleport client.\n")

	stream, err := client.ReportSecrets(ctx)
	if err != nil {
		// FIXME: handle unimplemented
		return trace.Wrap(err)
	}

	cremonyProccess, err := assert.NewCeremony()
	if err != nil {
		return trace.Wrap(err, "failed to create ceremony")
	}

	// Run the device authentication ceremony.
	// If successful, the device will be authenticated and the device can report its secrets.
	err = cremonyProccess.Run(
		ctx,
		reportToAssertstreamAdapter{stream},
	)
	if err != nil {
		return trace.Wrap(err, "failed to run device authentication ceremony")
	}

	keysSlice := make([]*accessgraphsecretsv1pb.PrivateKey, 0, len(keys))
	for _, key := range keys {
		keysSlice = append(keysSlice, key)
	}

	if err := stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_PrivateKeys{
			PrivateKeys: &accessgraphsecretsv1pb.ReportPrivateKeys{
				Keys: keysSlice,
			},
		},
	}); err != nil {
		return trace.Wrap(err, "failed to send private keys")
	}

	if err := stream.CloseSend(); err != nil {
		return trace.Wrap(err, "failed to close send")
	}

	if _, err := stream.Recv(); err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err, "error closing the stream")
	}

	fmt.Fprintf(os.Stdout, "Reported %d SSH fingerprints to Teleport.\n", len(keys))

	return nil
}

func (c *scanKeysCommand) scan(ctx context.Context, deviceID string) map[string]*accessgraphsecretsv1pb.PrivateKey {
	allKeys := make(map[string]*accessgraphsecretsv1pb.PrivateKey)
	for _, dir := range c.dirs {
		keys := walkAndCheckFiles(ctx, dir, deviceID)
		maps.Copy(allKeys, keys)
	}

	return allKeys
}

func (c *scanKeysCommand) print(out io.Writer, keys map[string]*accessgraphsecretsv1pb.PrivateKey) {
	if len(keys) == 0 {
		fmt.Fprintf(out, "No SSH private keys found.\n")
		return
	}
	publicKeyMode := func(mode accessgraphsecretsv1pb.PublicKeyMode) string {
		switch mode {
		case accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE:
			return "used public key file"
		case accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED:
			return "protected private key"
		case accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED:
			return "derived from private key"
		default:
			return "unknown"
		}

	}
	fmt.Fprintf(out, "%d SSH private keys found.\n", len(keys))
	for path, key := range keys {
		fmt.Fprintf(out, "- SHA256: %q (%s) at %s\n",
			key.Spec.PublicKeyFingerprint,
			publicKeyMode(key.Spec.PublicKeyMode),
			path,
		)
	}
}

var (
	supportedPrivateKeysHeaders = [][]byte{
		[]byte("RSA PRIVATE KEY"),
		[]byte("PRIVATE KEY"),
		[]byte("EC PRIVATE KEY"),
		[]byte("DSA PRIVATE KEY"),
		[]byte("OPENSSH PRIVATE KEY"),
	}
)

// isSSHPrivateKey checks if a file is an OpenSSH private key
func isSSHPrivateKey(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// read the first 60 bytes of the file
	var buf [150]byte
	n, err := file.Read(buf[:])
	if errors.Is(err, io.EOF) || n < len(buf) {
		return false, nil
	} else if err != nil {
		return false, trace.Wrap(err, "failed to read file")
	}

	for _, header := range supportedPrivateKeysHeaders {
		if bytes.Contains(buf[:], header) {
			return true, nil
		}
	}
	return false, nil
}

// walkAndCheckFiles walks through all files in a directory and its subdirectories
// and checks if they are OpenSSH private keys
func walkAndCheckFiles(ctx context.Context, root, deviceID string) map[string]*accessgraphsecretsv1pb.PrivateKey {
	logger := slog.Default()
	keys := make(map[string]*accessgraphsecretsv1pb.PrivateKey)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch isKey, err := isSSHPrivateKey(path); {
		case err != nil:
			logger.DebugContext(ctx, "error reading file", "path", path, "error", err)
		case isKey:
			key, err := extractSSHKey(ctx, path, deviceID)
			if err != nil {
				logger.DebugContext(ctx, "error extracting private key", "path", path, "error", err)
			} else {
				keys[path] = key
			}
		}
		return nil
	})

	if err != nil {
		logger.WarnContext(ctx, "error walking directory", "root", root, "error", err)
	}
	return keys
}

func extractSSHKey(ctx context.Context, path, deviceID string) (*accessgraphsecretsv1pb.PrivateKey, error) {
	logger := slog.Default().With("private_key_file", path, "device_id", deviceID)
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var publicKey ssh.PublicKey
	var mode accessgraphsecretsv1pb.PublicKeyMode
	var pme *ssh.PassphraseMissingError
	switch pk, err := ssh.ParsePrivateKey(fileData); {
	case errors.As(err, &pme):
		mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED
		if pk, err = ssh.ParsePrivateKeyWithPassphrase(fileData, nil); errors.As(err, &pme) && pme.PublicKey != nil {
			publicKey = pme.PublicKey
			mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED
			break
		}

		pubPath := path + ".pub"
		logger = logger.With("public_key_file", pubPath)
		logger.DebugContext(ctx, "PrivateKey is password protected. Fallback to public key file.")

		pubData, err := os.ReadFile(pubPath)
		if err == nil {
			logger.DebugContext(ctx, "Trying to parse public key as authorized key data.")
			if pub, _, _, _, err := ssh.ParseAuthorizedKey(pubData); err == nil {
				publicKey = pub
				mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
				break
			} else {
				logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)
			}

			logger.DebugContext(ctx, "Trying to parse public key directly.")
			if pub, err := ssh.ParsePublicKey(pubData); err == nil {
				publicKey = pub
				mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
				break
			} else {
				logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)
			}
		} else {
			logger.DebugContext(ctx, "Unable to read public key file.", "err", err)
		}
	case err != nil:
		return nil, trace.Wrap(err)
	default:
		publicKey = pk.PublicKey()
	}
	var fingerprint string
	if publicKey != nil {
		fingerprint = ssh.FingerprintSHA256(publicKey)
	}

	key, err := accessgraph.NewPrivateKeyWithName(
		privateKeyNameGen(path, deviceID, fingerprint),
		&accessgraphsecretsv1pb.PrivateKeySpec{
			PublicKeyFingerprint: fingerprint,
			DeviceId:             deviceID,
			PublicKeyMode:        mode,
		},
	)
	return key, trace.Wrap(err)
}

func privateKeyNameGen(path, deviceID, fingerprint string) string {
	sha := sha256.New()
	sha.Write([]byte(path))
	sha.Write([]byte(deviceID))
	sha.Write([]byte(fingerprint))
	return hex.EncodeToString(sha.Sum(nil))
}

// streamAdapter is a wrapper for the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] that implements the
// [assert.AssertDeviceClientStream] interface.
//
// This adapter allows the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] to be used with the [assert.AssertDeviceClientStream]
// interface, which is essential for the [assert.Ceremony] in executing the device authentication process. It handles the extraction and insertion
// of device assertion messages from and into the [accessgraphsecretsv1pb.ReportSecretsRequest] and [accessgraphsecretsv1pb.ReportSecretsResponse] messages.
type reportToAssertstreamAdapter struct {
	accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient
}

func (s reportToAssertstreamAdapter) Send(request *devicepb.AssertDeviceRequest) error {
	return trace.Wrap(
		s.SecretsScannerService_ReportSecretsClient.Send(
			&accessgraphsecretsv1pb.ReportSecretsRequest{
				Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
					DeviceAssertion: request,
				},
			},
		),
	)
}

func (s reportToAssertstreamAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	in, err := s.SecretsScannerService_ReportSecretsClient.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if in.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("unsupported response type: expected DeviceAssertion, got %T", in.Payload)
	}

	return in.GetDeviceAssertion(), nil
}
