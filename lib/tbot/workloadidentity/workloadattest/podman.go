/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package workloadattest

import (
	"net/url"
	"path/filepath"

	"github.com/gravitational/trace"
)

// PodmanAttestorConfig holds the configuration for the Podman workload attestor.
type PodmanAttestorConfig struct {
	// Enabled determines whether Podman workload attestation will be performed.
	Enabled bool `yaml:"enabled"`

	// Addr is the address at which the [Podman API service] can be reached.
	//
	// It must be in the form `unix://path/to/socket`, as making the service
	// reachable via the network (i.e. binding to a TCP port) is *strongly*
	// discouraged for security reasons.
	//
	// Addr is a required option. We do not provide a default value because
	// there isn't one that would work "out of the box" in the majority of cases.
	//
	// # Rootful Podman
	//
	// If running Podman as root, enable and start the API service by running:
	//
	// 	$ sudo systemctl enable --now podman.socket
	//
	// At this point, the service can be reached at `unix:///run/podman/podman.sock`
	// but only by root. If you do not want to run tbot as root, you can override
	// the default systemd unit to make the socket owned by a group your tbot user
	// belongs to.
	//
	// 	$ sudo groupadd podman-root
	// 	$ sudo usermod -a -G podman-root <tbot user>
	// 	$ sudo systemctl edit podman.socket
	//
	// In the override file add:
	//
	// 	[Socket]
	// 	SocketGroup=podman-root
	//
	// Remember to reload the systemd daemon and restart the socket unit:
	//
	// 	$ sudo systemctl daemon-reload
	// 	$ sudo systemctl restart podman.socket
	//
	// # Rootless Podman
	//
	// If running Podman as a non-root user, enable and start the API service by
	// running:
	//
	// 	$ sudo loginctl enable-linger <podman user>
	// 	$ systemctl --user enable --now podman.socket
	//
	// The service can now be reached at `unix://$XDG_RUNTIME_DIR/podman/podman.sock`
	// (i.e. `/run/user/$(id -u)/podman/podman.socket`) but only by your chosen
	// Podman user or root. If you do not want to run tbot as the same user or
	// as root, you can override the default systemd unit to move the socket out
	// of `$XDG_RUNTIME_DIR` and add your tbot user to the podman user's group.
	//
	// 	$ sudo usermod -a -G <podman user group> <tbot user>
	// 	$ systemctl --user edit podman.socket
	//
	// In the override file add:
	//
	// 	[Socket]
	// 	ListenStream=/path/to/socket # Example: /srv/podman.<podman user>/podman.sock
	//
	// Remember to reload the systemd daemon and restart the socket unit:
	//
	// 	$ systemctl --user daemon-reload
	// 	$ systemctl --user restart podman.socket
	//
	// # Running tbot in a container
	//
	// In order for tbot running inside a container to attest workloads in other
	// containers, it must have access to the host's PID namespace. Also, if you
	// are using group permissions to grant access to the Podman socket (see the
	// above examples) you must set the `run.oci.keep_original_groups` annotation.
	//
	// 	$ podman run \
	// 		--pid=host \
	// 		--annotation run.oci.keep_original_groups=1 \
	// 		--volume /path/to/socket:/path/to/socket \
	// 		--volume /path/to/config:/path/to/config \
	// 		<tbot image> \
	// 		start -c /path/to/config
	//
	// [Podman API service]: https://docs.podman.io/en/latest/markdown/podman-system-service.1.html
	Addr string `yaml:"addr,omitempty"`
}

func (c PodmanAttestorConfig) CheckAndSetDefaults() error {
	if !c.Enabled {
		return nil
	}

	if c.Addr == "" {
		return trace.BadParameter("addr is required")
	}

	u, err := url.Parse(c.Addr)
	if err != nil {
		return trace.Wrap(err, "invalid addr")
	}
	if u.Scheme != "unix" || (u.Path == "" && u.Host == "") {
		return trace.BadParameter("addr must be in the form `unix://path/to/socket`")
	}

	// Forgetting the leading slash is a common typo with `unix://` addresses.
	if u.Host != "" {
		suggestedAddr := &url.URL{
			Scheme: u.Scheme,
			Path:   filepath.Join("/", u.Host, u.Path),
		}
		return trace.BadParameter(
			"addr host segment must be empty, did you forget a leading slash in the socket path? (i.e. `%s`)",
			suggestedAddr.String(),
		)
	}

	return nil
}
