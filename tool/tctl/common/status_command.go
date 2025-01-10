/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"cmp"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// StatusCommand implements `tctl token` group of commands.
type StatusCommand struct {
	config *servicecfg.Config

	// CLI clauses (subcommands)
	status *kingpin.CmdClause
}

// Initialize allows StatusCommand to plug itself into the CLI parser.
func (c *StatusCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	c.status = app.Command("status", "Report cluster status.")
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *StatusCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.status.FullCommand():
		commandFunc = c.Status
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

// Status is called to execute "status" CLI command.
func (c *StatusCommand) Status(ctx context.Context, client *authclient.Client) error {
	pingResp, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	status, err := newStatusModel(ctx, client, pingResp)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := status.renderText(os.Stdout, c.config.Debug); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type statusModel struct {
	cluster     *clusterStatusModel
	authorities []*authorityStatusModel
}

func newStatusModel(ctx context.Context, client *authclient.Client, pingResp proto.PingResponse) (*statusModel, error) {
	var authorities []types.CertAuthority
	for _, caType := range types.CertAuthTypes {
		cas, err := client.GetCertAuthorities(ctx, caType, false)
		if err != nil {
			slog.WarnContext(ctx, "Failed to fetch CA", "type", caType, "error", err)
			continue
		}
		authorities = append(authorities, cas...)
	}
	cluster, err := newClusterStatusModel(pingResp, authorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authorityModels := make([]*authorityStatusModel, 0, len(authorities))
	for _, authority := range authorities {
		authorityModels = append(authorityModels, newAuthorityStatusModel(authority))
	}
	return &statusModel{
		cluster:     cluster,
		authorities: authorityModels,
	}, nil
}

func (m *statusModel) renderText(w io.Writer, debug bool) error {
	summaryTable := asciitable.MakeHeadlessTable(2)
	summaryTable.AddRow([]string{"Cluster:", m.cluster.name})
	summaryTable.AddRow([]string{"Version:", m.cluster.version})
	for i, caPin := range m.cluster.caPins {
		if i == 0 {
			summaryTable.AddRow([]string{"CA pins:", caPin})
		} else {
			summaryTable.AddRow([]string{"", caPin})
		}
	}
	if err := summaryTable.WriteTo(w); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(w, "")

	keysTable := asciitable.MakeTable([]string{"authority", "rotation", "protocol", "status", "algorithm", "storage"})
	for _, authority := range m.authorities {
		if !debug && authority.clusterName != m.cluster.name {
			// Only print remote authorities in debug mode.
			continue
		}
		rows := make([][]string, 0, len(authority.activeKeys)+len(authority.additionalTrustedKeys))
		for _, key := range authority.activeKeys {
			rows = append(rows, []string{"", "", key.protocol, "active", key.algo, key.storage})
		}
		for _, key := range authority.additionalTrustedKeys {
			rows = append(rows, []string{"", "", key.protocol, "trusted", key.algo, key.storage})
		}
		sortRows(rows)
		if len(rows) == 0 {
			rows = [][]string{[]string{"", "", "", "no keys found for authority", "", ""}}
		}
		// Each CA gets a row in the table for each key. Only the first row
		// contains the CA type and the CA rotation status, to reduce clutter
		// because it's the same for all keys.
		rows[0][0] = string(authority.authorityType)
		rows[0][1] = authority.rotationStatus.String()
		for _, row := range rows {
			keysTable.AddRow(row)
		}
	}
	return trace.Wrap(keysTable.WriteTo(w))
}

// sortRows sorts the rows by each column left to right.
func sortRows(rows [][]string) {
	slices.SortFunc(rows, func(a, b []string) int {
		for i := range len(a) {
			if a[i] != b[i] {
				return cmp.Compare(a[i], b[i])
			}
		}
		return 0
	})
}

type clusterStatusModel struct {
	name    string
	version string
	caPins  []string
}

func newClusterStatusModel(pingResp proto.PingResponse, authorities []types.CertAuthority) (*clusterStatusModel, error) {
	var pins []string
	for _, authority := range authorities {
		if authority.GetType() != types.HostCA || authority.GetClusterName() != pingResp.ClusterName {
			continue
		}
		for _, keyPair := range authority.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err, "parsing host CA TLS certificate")
			}
			pin := utils.CalculateSPKI(cert)
			pins = append(pins, pin)
		}
	}
	return &clusterStatusModel{
		name:    pingResp.ClusterName,
		version: pingResp.ServerVersion,
		caPins:  pins,
	}, nil
}

type authorityStatusModel struct {
	clusterName           string
	authorityType         types.CertAuthType
	rotationStatus        types.Rotation
	activeKeys            []*authorityKeyModel
	additionalTrustedKeys []*authorityKeyModel
}

func newAuthorityStatusModel(authority types.CertAuthority) *authorityStatusModel {
	return &authorityStatusModel{
		clusterName:           authority.GetClusterName(),
		authorityType:         authority.GetType(),
		rotationStatus:        authority.GetRotation(),
		activeKeys:            newAuthorityKeyModels(authority.GetActiveKeys()),
		additionalTrustedKeys: newAuthorityKeyModels(authority.GetAdditionalTrustedKeys()),
	}
}

type authorityKeyModel struct {
	protocol string
	algo     string
	storage  string
}

func newAuthorityKeyModels(keySet types.CAKeySet) []*authorityKeyModel {
	var out []*authorityKeyModel
	for _, sshKey := range keySet.SSH {
		out = append(out, newSSHKeyModel(sshKey))
	}
	for _, tlsKey := range keySet.TLS {
		out = append(out, newTLSKeyModel(tlsKey))
	}
	for _, jwtKey := range keySet.JWT {
		out = append(out, newJWTKeyModel(jwtKey))
	}
	return out
}

func newSSHKeyModel(sshKey *types.SSHKeyPair) *authorityKeyModel {
	algo := "unknown"
	if pub, _, _, _, err := ssh.ParseAuthorizedKey(sshKey.PublicKey); err == nil {
		algo = publicKeyAlgorithmName(pub)
	} else {
		slog.ErrorContext(context.Background(), "parsing SSH CA public key", "error", err)
	}
	return &authorityKeyModel{
		protocol: "SSH",
		algo:     algo,
		storage:  storageName(sshKey.PrivateKeyType),
	}
}

func newTLSKeyModel(tlsKey *types.TLSKeyPair) *authorityKeyModel {
	algo := "unknown"
	if cert, err := tlsca.ParseCertificatePEM(tlsKey.Cert); err == nil {
		algo = publicKeyAlgorithmName(cert.PublicKey)
	} else {
		slog.ErrorContext(context.Background(), "parsing TLS CA public key", "error", err)
	}
	return &authorityKeyModel{
		protocol: "TLS",
		algo:     algo,
		storage:  storageName(tlsKey.KeyType),
	}
}

func newJWTKeyModel(jwtKey *types.JWTKeyPair) *authorityKeyModel {
	algo := "unknown"
	if pubKey, err := keys.ParsePublicKey(jwtKey.PublicKey); err == nil {
		algo = publicKeyAlgorithmName(pubKey)
	} else {
		slog.ErrorContext(context.Background(), "parsing JWT CA public key", "error", err)
	}
	return &authorityKeyModel{
		protocol: "JWT",
		algo:     algo,
		storage:  storageName(jwtKey.PrivateKeyType),
	}
}

func publicKeyAlgorithmName(pubKey crypto.PublicKey) string {
	switch k := pubKey.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", k.Size()*8)
	case *ecdsa.PublicKey:
		return fmt.Sprintf("ECDSA %s", k.Params().Name)
	case ed25519.PublicKey:
		return "Ed25519"
	case ssh.CryptoPublicKey:
		return publicKeyAlgorithmName(k.CryptoPublicKey())
	default:
		return "unknown"
	}
}

func storageName(privateKeyType types.PrivateKeyType) string {
	switch privateKeyType {
	case types.PrivateKeyType_RAW:
		return "software"
	case types.PrivateKeyType_PKCS11:
		return "PKCS#11 HSM"
	case types.PrivateKeyType_GCP_KMS:
		return "GCP KMS"
	case types.PrivateKeyType_AWS_KMS:
		return "AWS KMS"
	default:
		return privateKeyType.String()
	}
}
