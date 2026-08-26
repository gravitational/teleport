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

package ssh

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

const (
	// ConfigName is the name of the ssh_config file on disk
	ConfigName = "ssh_config"

	// KnownHostsName is the name of the known_hosts file on disk
	KnownHostsName = "known_hosts"
)

type certAuthorityGetter interface {
	GetCertAuthority(
		ctx context.Context,
		id types.CertAuthID,
		includeSigningKeys bool,
	) (types.CertAuthority, error)
}

// GenerateKnownHosts generates a known_hosts file for the provided cluster
// names and proxy hosts.
//
// It produces:
//   - A main known_hosts file that includes all the clusters, with each
//     cluster's CA limited to the wildcard of the cluster's domain name.
//   - A known_hosts file per cluster that will match any hostname.
func GenerateKnownHosts(
	ctx context.Context,
	bot certAuthorityGetter,
	clusterNames []string,
	proxyHosts string,
) (string, map[string]string, error) {
	certAuthorities := make([]types.CertAuthority, 0, len(clusterNames))
	for _, cn := range clusterNames {
		ca, err := bot.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: cn,
		}, false)
		if err != nil {
			return "", nil, trace.Wrap(err)
		}
		certAuthorities = append(certAuthorities, ca)
	}

	perCluster := make(map[string]string)
	var sb strings.Builder
	for _, auth := range authclient.AuthoritiesToTrustedCerts(certAuthorities) {
		pubKeys, err := auth.SSHCertPublicKeys()
		if err != nil {
			return "", nil, trace.Wrap(err)
		}

		var perClusterSB strings.Builder
		fmt.Fprintf(
			&perClusterSB,
			"# Cluster specific known_hosts generated for cluster '%s'\n",
			auth.ClusterName,
		)
		for _, pubKey := range pubKeys {
			bytes := ssh.MarshalAuthorizedKey(pubKey)
			fmt.Fprintf(&sb,
				"@cert-authority %s,%s,*.%s %s type=host\n",
				proxyHosts,
				auth.ClusterName,
				auth.ClusterName,
				strings.TrimSpace(string(bytes)),
			)
			fmt.Fprintf(&perClusterSB,
				"@cert-authority * %s type=host\n",
				strings.TrimSpace(string(bytes)),
			)
		}
		perCluster[auth.ClusterName] = perClusterSB.String()
	}

	return sb.String(), perCluster, nil
}
