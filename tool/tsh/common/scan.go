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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proxy/transport/transportv1"
	"github.com/gravitational/teleport/api/client/webclient"
	apiconstants "github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
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
	c.Flag("ca", "CA to use for TLS").StringVar(&c.ca)
	return c
}

func (c *scanKeysCommand) run(cf *CLIConf) error {
	ctx := cf.Context
	// make the teleport client and retrieve the certificate from the proxy:
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	pingResp, err := tc.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var caData [][]byte
	if !tc.InsecureSkipVerify {
		switch {
		case c.ca != "":
			data, err := base64.StdEncoding.DecodeString(c.ca)
			if err != nil {
				return trace.Wrap(err, "failed to decode CA data")
			}
			caData = append(caData, data)
		default:
			// try to get the ca from the trust store
			if tc.LocalAgent() != nil {
				if key, err := tc.LocalAgent().GetKey(pingResp.ClusterName); err == nil {
					if data, err := key.RootClusterCAs(); err == nil {
						caData = data
						break
					}
				}
			}
			return trace.BadParameter("Cannot find CA data. Please provide it using --ca flag or use --insecure mode")
		}
	}

	if pingResp.Auth.DeviceTrust.Disabled {
		return trace.BadParameter("Device Trust is disabled")
	}

	ceremonyProcess := authn.NewCeremony()

	deviceCred, err := ceremonyProcess.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err, "device not enrolled")
	}
	fmt.Fprintf(tc.Stdout, "Device trust credentials found.\n")
	fmt.Fprintf(tc.Stdout, "Scanning %s.\n", strings.Join(c.dirs, ", "))

	keys := c.scan(ctx, deviceCred.Id)
	c.print(tc.Stdout, keys)

	fmt.Fprintf(tc.Stdout, "Creating Teleport client.\n")

	clientCfg, err := ClientConfig(ctx, tc, pingResp, caData)
	if err != nil {
		return trace.Wrap(err)
	}
	authClient, err := authclient.NewClient(clientCfg)
	if err != nil {
		return trace.Wrap(err)
	}
	stream, err := authClient.
		AccessGraphSecretsScannerClient().
		ReportSecrets(ctx)
	if err != nil {
		// FIXME: handle unimplemented
		return trace.Wrap(err)
	}

	// Run the device authentication ceremony.
	// If successful, the device will be authenticated and the device can report its secrets.
	_, err = ceremonyProcess.RunWithStream(
		streamConverter{stream},
		// important fields will be automatically filled in by the ceremonyProcess.
		&devicepb.AuthenticateDeviceInit{})
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
	fmt.Fprintf(tc.Stdout, "Reported %d SSH fingerprints to Teleport.\n", len(keys))

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
			return "public key file"
		case accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED:
			return "protected"
		case accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED:
			return "derived"
		default:
			return "unknown"
		}

	}
	fmt.Fprintf(out, "%d SSH private keys found.\n", len(keys))
	for path, key := range keys {
		fmt.Fprintf(out, "- SHA256: %q (%s) at %s\n",
			key.Spec.KeyFingerprint,
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
			KeyFingerprint: fingerprint,
			DeviceId:       deviceID,
			PublicKeyMode:  mode,
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

type streamConverter struct {
	accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient
}

func (s streamConverter) Send(request *devicepb.AuthenticateDeviceRequest) error {
	var req *accessgraphsecretsv1pb.ReportSecretsRequest
	switch inReq := request.Payload.(type) {
	case *devicepb.AuthenticateDeviceRequest_Init:
		req = &accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_Init{
				Init: &accessgraphsecretsv1pb.AuthenticateDeviceInit{
					CredentialId: inReq.Init.CredentialId,
					DeviceData:   inReq.Init.DeviceData,
				},
			},
		}
	case *devicepb.AuthenticateDeviceRequest_TpmChallengeResponse:
		req = &accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_TpmChallengeResponse{
				TpmChallengeResponse: inReq.TpmChallengeResponse,
			},
		}
	case *devicepb.AuthenticateDeviceRequest_ChallengeResponse:
		req = &accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_ChallengeResponse{
				ChallengeResponse: inReq.ChallengeResponse,
			},
		}
	default:
		return trace.BadParameter("unsupported response type: %T", request.Payload)
	}

	return s.SecretsScannerService_ReportSecretsClient.Send(req)
}

func (s streamConverter) Recv() (*devicepb.AuthenticateDeviceResponse, error) {
	in, err := s.SecretsScannerService_ReportSecretsClient.Recv()
	if err != nil {
		return nil, err
	}
	var rsp *devicepb.AuthenticateDeviceResponse
	switch inRsp := in.Payload.(type) {
	case *accessgraphsecretsv1pb.ReportSecretsResponse_Challenge:
		rsp = &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_Challenge{
				Challenge: inRsp.Challenge,
			},
		}
	case *accessgraphsecretsv1pb.ReportSecretsResponse_TpmChallenge:
		rsp = &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_TpmChallenge{
				TpmChallenge: inRsp.TpmChallenge,
			},
		}
	case *accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAuthenticated:
		rsp = &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
				UserCertificates: &devicepb.UserCertificates{},
			},
		}
	}
	return rsp, nil
}

func ClientConfig(ctx context.Context, tc *libclient.TeleportClient, cfg *webclient.PingResponse, caData [][]byte) (client.Config, error) {
	sniName := fmt.Sprintf("%x."+apiconstants.APIDomain, cfg.ClusterName)
	certPool := x509.NewCertPool()
	for _, data := range caData {
		if !certPool.AppendCertsFromPEM(data) {
			return client.Config{}, trace.BadParameter("failed to parse CA data")
		}
	}
	creds := client.LoadTLS(client.ConfigureALPN(&tls.Config{
		InsecureSkipVerify: tc.InsecureSkipVerify,
		RootCAs:            certPool,
		NextProtos: []string{
			http2.NextProtoTLS,
		},
		ServerName: sniName,
	}, cfg.ClusterName))

	if tc.TLSRoutingEnabled {
		return client.Config{
			Context:                    ctx,
			Addrs:                      []string{tc.WebProxyAddr},
			Credentials:                []client.Credentials{creds},
			ALPNSNIAuthDialClusterName: cfg.ClusterName,
			CircuitBreakerConfig:       breaker.NoopBreakerConfig(),
			ALPNConnUpgradeRequired:    tc.TLSRoutingEnabled && tc.TLSRoutingConnUpgradeRequired,
			DialOpts:                   tc.DialOpts,
			InsecureAddressDiscovery:   tc.InsecureSkipVerify,
			DialInBackground:           true,
		}, nil
	}

	proxyClient, _ := transportv1.NewClient(nil)

	return client.Config{
		Context:                  ctx,
		Credentials:              []client.Credentials{creds},
		CircuitBreakerConfig:     breaker.NoopBreakerConfig(),
		DialInBackground:         true,
		InsecureAddressDiscovery: tc.InsecureSkipVerify,
		Dialer: client.ContextDialerFunc(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
			conn, err := proxyClient.DialCluster(dialCtx, cfg.ClusterName, nil)
			return conn, trace.Wrap(err)
		}),
		DialOpts: tc.DialOpts,
	}, nil
}
