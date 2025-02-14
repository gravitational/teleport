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

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/ssh"
)

const IdentityOutputType = "identity"

const (
	// HostCAPath is the default filename for the host CA certificate
	HostCAPath = "teleport-host-ca.crt"

	// UserCAPath is the default filename for the user CA certificate
	UserCAPath = "teleport-user-ca.crt"

	// DatabaseCAPath is the default filename for the database CA
	// certificate
	DatabaseCAPath = "teleport-database-ca.crt"

	IdentityFilePath = "identity"
)

// SSHConfigMode controls whether to write an ssh_config file to the
// destination directory.
type SSHConfigMode string

const (
	// SSHConfigModeNone will default to SSHConfigModeOn.
	SSHConfigModeNone SSHConfigMode = ""
	// SSHConfigModeOff will not write an ssh_config file. This is useful where
	// you do not want to use SSH functionality and would like to avoid
	// pinging the proxy.
	SSHConfigModeOff SSHConfigMode = "off"
	// SSHConfigModeOn will write an ssh_config file to the destination
	// directory.
	// Causes the generation of:
	// - ssh_config
	// - known_hosts
	SSHConfigModeOn SSHConfigMode = "on"
)

var (
	_ ServiceConfig = &IdentityOutput{}
	_ Initable      = &IdentityOutput{}
)

type AccessRequest struct {
	Roles []string `yaml:"roles"`
	Reviewers []string
	Reason string
}

// IdentityOutput produces credentials which can be used with `tsh`, `tctl`,
// `openssh` and most SSH compatible tooling. It can also be used with the
// Teleport API and things which use the API client (e.g the terraform provider)
//
// It cannot be used to connect to Applications, Databases or Kubernetes
// Clusters.
type IdentityOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// Cluster allows certificates to be generated for a leaf cluster of the
	// cluster that the bot is connected to. These certificates can be used
	// to directly connect to a Teleport proxy of that leaf cluster, or used
	// with the root cluster's proxy which will forward the request to the
	// leaf cluster.
	// For now, only SSH is supported.
	Cluster string `yaml:"cluster,omitempty"`

	// SSHConfigMode controls whether to write an ssh_config file to the
	// destination directory. Defaults to SSHConfigModeOn.
	SSHConfigMode SSHConfigMode `yaml:"ssh_config,omitempty"`

	// AllowReissue controls whether the generated credentials can be used to
	// reissue further credentials (e.g to produce a certificate for application
	// access). It is recommended to leave this disabled to prevent the scope of
	// issued credentials from being increased, however, it can be useful in
	// scenarios where credentials are desired to be reissued in a dynamic way.
	//
	// Defaults to false.
	AllowReissue bool `yaml:"allow_reissue,omitempty"`

	// AccessRequest alternatively uses just-in-time access requests to fetch
	// alternative credentials for this output. Mutually exclusive with Roles.
	AccessRequest *AccessRequest `yaml:"access_request,omitempty"`
}

func (o *IdentityOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *IdentityOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *IdentityOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}

	if _, ok := o.Destination.(*DestinationDirectory); !ok {
		// If destDir is unset, we're not using a filesystem destination and
		// ssh_config will not be sensible. Log a note and bail early without
		// writing ssh_config. (Future users of k8s secrets will need to bring
		// their own config, we can't predict where paths will be in practice.)
		log.InfoContext(
			context.TODO(),
			"Note: no ssh_config will be written for non-filesystem destination",
			"destination", o.Destination,
		)
	}

	switch o.SSHConfigMode {
	case SSHConfigModeNone:
		log.DebugContext(context.Background(), "Defaulting to SSHConfigModeOn")
		o.SSHConfigMode = SSHConfigModeOn
	case SSHConfigModeOff, SSHConfigModeOn:
	default:
		return trace.BadParameter("ssh_config: unrecognized value %q", o.SSHConfigMode)
	}

	return nil
}

func (o *IdentityOutput) Describe() []FileDescription {
	var fds = []FileDescription{
		{
			Name: IdentityFilePath,
		},
		{
			Name: HostCAPath,
		},
		{
			Name: UserCAPath,
		},
		{
			Name: DatabaseCAPath,
		},
	}
	if o.SSHConfigMode == SSHConfigModeOn {
		fds = append(fds, FileDescription{
			Name: ssh.KnownHostsName,
		})
		if _, ok := o.Destination.(*DestinationDirectory); ok {
			fds = append(fds, FileDescription{
				Name: ssh.ConfigName,
			})
		}
	}

	return fds
}

func (o *IdentityOutput) MarshalYAML() (interface{}, error) {
	type raw IdentityOutput
	return withTypeHeader((*raw)(o), IdentityOutputType)
}

func (o *IdentityOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw IdentityOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *IdentityOutput) Type() string {
	return IdentityOutputType
}
