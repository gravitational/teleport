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

package cli

// SSHProxyCommand includes fields for `tbot ssh-proxy-command`
type SSHProxyCommand struct {
	*genericExecutorHandler[SSHProxyCommand]

	// DestinationDir stores the generated end-user certificates.
	DestinationDir string

	// Cluster is the name of the Teleport cluster on which resources should
	// be accessed.
	Cluster string

	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer string

	// User is the os login to use for ssh connections.
	User string

	// Host is the target ssh machine to connect to.
	Host string

	// Post is the post of the ssh machine to connect on.
	Port string

	// EnableResumption turns on automatic session resumption to prevent connections from
	// being dropped if Proxy connectivity is lost.
	EnableResumption bool

	// TLSRoutingEnabled indicates whether the cluster has TLS routing enabled.
	TLSRoutingEnabled bool

	// ConnectionUpgradeRequired indicates that an ALPN connection upgrade is required
	// for connections to the cluster.
	ConnectionUpgradeRequired bool

	// TSHConfigPath is the path to a tsh config file.
	TSHConfigPath string
}

// NewSSHProxyCommand initializes the `tbot ssh-proxy-command` subcommand and
// its fields.
func NewSSHProxyCommand(app KingpinClause, action func(*SSHProxyCommand) error) *SSHProxyCommand {
	cmd := app.Command("ssh-proxy-command", "An OpenSSH/PuTTY proxy command").Hidden()

	c := &SSHProxyCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&c.DestinationDir)
	cmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&c.Cluster)
	cmd.Flag("user", "The remote user name for the connection").Required().StringVar(&c.User)
	cmd.Flag("host", "The remote host to connect to").Required().StringVar(&c.Host)
	cmd.Flag("port", "The remote port to connect on.").StringVar(&c.Port)
	cmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").Required().StringVar(&c.ProxyServer)
	cmd.Flag("tls-routing", "Whether the Teleport cluster has tls routing enabled.").Required().BoolVar(&c.TLSRoutingEnabled)
	cmd.Flag("connection-upgrade", "Whether the Teleport cluster requires an ALPN connection upgrade.").Required().BoolVar(&c.ConnectionUpgradeRequired)
	cmd.Flag("proxy-templates", "The path to a file containing proxy templates to be evaluated.").StringVar(&c.TSHConfigPath)
	cmd.Flag("resume", "Enable SSH connection resumption").BoolVar(&c.EnableResumption)

	return c
}
