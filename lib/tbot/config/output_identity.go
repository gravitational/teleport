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
	"fmt"
	"os"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const IdentityOutputType = "identity"

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

	destPath string
}

func (o *IdentityOutput) templates() []template {
	templates := []template{
		&templateTLSCAs{},
		&templateIdentity{},
	}
	if o.SSHConfigMode == SSHConfigModeOn {
		templates = append(templates, &templateSSHClient{
			getSSHVersion:        openssh.GetSystemSSHVersion,
			executablePathGetter: os.Executable,
			destPath:             o.destPath,
		})
	}
	return templates
}

func (o *IdentityOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	ctx, span := tracer.Start(
		ctx,
		"IdentityOutput/Render",
	)
	defer span.End()

	dest := o.GetDestination()
	if err := identity.SaveIdentity(ctx, ident, dest, identity.DestinationKinds()...); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, dest); err != nil {
			return trace.Wrap(err, "rendering template %s", t.name())
		}
	}

	return nil
}

func (o *IdentityOutput) Init(ctx context.Context) error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Destination.Init(ctx, subDirs))
}

func (o *IdentityOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *IdentityOutput) GetRoles() []string {
	return o.Roles
}

func (o *IdentityOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	dest, ok := o.Destination.(*DestinationDirectory)
	if ok {
		o.destPath = dest.Path
	} else {
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
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
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

func (o *IdentityOutput) String() string {
	return fmt.Sprintf("%s (%s)", IdentityOutputType, o.GetDestination())
}
