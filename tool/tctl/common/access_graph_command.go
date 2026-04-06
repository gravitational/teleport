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
	"crypto"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/utils/keys"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

const defaultAccessGraphListQuery = "SELECT * FROM nodes"

// AccessGraphCommand implements experimental Access Graph commands.
type AccessGraphCommand struct {
	access   *kingpin.CmdClause
	accessLS *kingpin.CmdClause

	query string
}

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	_ = config
	c.access = app.Command("access", "Experimental Access Graph commands.")
	c.accessLS = c.access.Command("ls", "List Access Graph resources using the query API.")
	c.accessLS.Flag("query", "SQL query to send to the Access Graph query API.").
		Default(defaultAccessGraphListQuery).
		StringVar(&c.query)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AccessGraphCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.accessLS.FullCommand():
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(c.list(ctx, client))
}

func (c *AccessGraphCommand) list(ctx context.Context, client *authclient.Client) error {
	proxyAddr, accessGraphClient, err := c.newAccessGraphHTTPClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	endpoint := url.URL{
		Scheme: "https",
		Host:   proxyAddr,
		Path:   "/v1/enterprise/accessgraph/query",
	}
	query := endpoint.Query()
	query.Set("query", c.query)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := accessGraphClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return trace.BadParameter("access graph query failed with status %v: %s", resp.Status, string(body))
	}

	var result accessgraphv1alpha.QueryResponse
	if err := protojson.Unmarshal(body, &result); err != nil {
		return trace.Wrap(err, "failed to decode access graph response")
	}

	return trace.Wrap(renderAccessGraphQueryResponse(os.Stdout, &result))
}

func (c *AccessGraphCommand) newAccessGraphHTTPClient(ctx context.Context, client *authclient.Client) (string, *http.Client, error) {
	pingResp, err := client.Ping(ctx)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	if pingResp.GetProxyPublicAddr() == "" {
		return "", nil, trace.NotFound("proxy public address is not configured")
	}
	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	existingTLSConfig := client.HTTPClient.TLSConfig()
	if existingTLSConfig == nil {
		return "", nil, trace.BadParameter("missing auth client TLS config")
	}

	signer, err := existingTLSSigner(existingTLSConfig)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	privateKeyPEM, certs, err := generateAccessGraphUserCerts(ctx, client, signer, currentUser.GetName())
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	fmt.Printf("Original TLS client cert for user %q:\n", currentUser.GetName())
	existingClientCert, err := existingTLSConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	fmt.Println(string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: existingClientCert.Certificate[0],
	})))
	fmt.Printf("New Access Graph client cert for user %q:\n", currentUser.GetName())
	fmt.Println(string(certs.TLS))

	clientCert, err := tls.X509KeyPair(certs.TLS, privateKeyPEM)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	proxyHost, _, err := webclient.ParseHostPort(pingResp.GetProxyPublicAddr())
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	baseTLSConfig, err := newAccessGraphTLSConfig(proxyHost, clientCert)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	return pingResp.GetProxyPublicAddr(), &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: baseTLSConfig,
			Proxy:           http.ProxyFromEnvironment,
		},
		Timeout: 30 * time.Second,
	}, nil
}

// generateAccessGraphUserCerts re-signs the existing TLS key material with an
// Access Graph-specific usage so the request can be authenticated by the proxy.
func generateAccessGraphUserCerts(ctx context.Context, client *authclient.Client, signer crypto.Signer, username string) ([]byte, *proto.Certs, error) {
	tlsPublicKey, err := keys.MarshalPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey: tlsPublicKey,
		Username:     username,
		Expires:      time.Now().Add(time.Hour),
		Usage:        proto.UserCertsRequest_AccessGraphAPI,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return privateKeyPEM, certs, nil
}

// newAccessGraphTLSConfig builds a web-proxy TLS config and layers the
// Access Graph client certificate on top of it.
func newAccessGraphTLSConfig(serverName string, clientCert tls.Certificate) (*tls.Config, error) {
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// existingTLSSigner extracts the signer backing the standard tctl TLS identity
// so Access Graph can reuse the same key material with a different cert usage.
func existingTLSSigner(tlsConfig *tls.Config) (crypto.Signer, error) {
	if len(tlsConfig.Certificates) > 0 {
		return signerFromTLSCertificate(&tlsConfig.Certificates[0])
	}
	if tlsConfig.GetClientCertificate == nil {
		return nil, trace.BadParameter("missing TLS client certificate")
	}
	fmt.Println("We made it here, since we do not have a certificate in the TLS config, but we do have a GetClientCertificate callback. Calling it to get the cert.")
	cert, err := tlsConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signerFromTLSCertificate(cert)
}

// signerFromTLSCertificate normalizes the TLS certificate's private key into a
// crypto.Signer so it can be re-signed by GenerateUserCerts.
func signerFromTLSCertificate(cert *tls.Certificate) (crypto.Signer, error) {
	if cert == nil {
		return nil, trace.BadParameter("missing TLS client certificate")
	}
	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("unsupported TLS private key type %T", cert.PrivateKey)
	}
	return signer, nil
}

// renderAccessGraphQueryResponse prints a small tabular view of returned nodes
// and falls back to JSON for edge data until a dedicated renderer exists.
func renderAccessGraphQueryResponse(w io.Writer, resp *accessgraphv1alpha.QueryResponse) error {
	if len(resp.GetNodes()) == 0 && len(resp.GetEdges()) == 0 {
		_, err := fmt.Fprintln(w, "No Access Graph resources returned.")
		return trace.Wrap(err)
	}

	if len(resp.GetNodes()) > 0 {
		t := asciitable.MakeTable([]string{"Kind", "Subkind", "Name", "ID", "Hostname"})
		for _, node := range resp.GetNodes() {
			t.AddRow([]string{
				node.GetKind(),
				node.GetSubKind(),
				node.GetName(),
				node.GetId(),
				node.GetHostname(),
			})
		}
		if _, err := t.AsBuffer().WriteTo(w); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(resp.GetEdges()) > 0 {
		out, err := json.MarshalIndent(resp.GetEdges(), "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintf(w, "\n\nEdges:\n%s\n", out); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
