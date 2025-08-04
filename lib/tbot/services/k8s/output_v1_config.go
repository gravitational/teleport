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

package k8s

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const OutputV1ServiceType = "kubernetes"

// OutputV1Config produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type OutputV1Config struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// KubernetesCluster is the name of the Kubernetes cluster in Teleport.
	// This is named a little more verbosely to avoid conflicting with the
	// name of the Teleport cluster to use.
	KubernetesCluster string `yaml:"kubernetes_cluster"`

	// DisableExecPlugin disables the default behavior of using `tbot` as a
	// `kubectl` credentials exec plugin. This is useful in environments where
	// `tbot` may not exist on the system that will consume the outputted
	// kubeconfig. It does mean that kubectl will not be able to automatically
	// refresh the credentials within an individual invocation.
	DisableExecPlugin bool `yaml:"disable_exec_plugin"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *OutputV1Config) GetName() string {
	return o.Name
}

func (o *OutputV1Config) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	if o.KubernetesCluster == "" {
		return trace.BadParameter("kubernetes_cluster must not be empty")
	}
	return nil
}

func (o *OutputV1Config) GetDestination() destination.Destination {
	return o.Destination
}

func (o *OutputV1Config) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *OutputV1Config) Describe() []bot.FileDescription {
	return []bot.FileDescription{
		{
			Name: "kubeconfig.yaml",
		},
		{
			Name: internal.IdentityFilePath,
		},
		{
			Name: internal.HostCAPath,
		},
		{
			Name: internal.UserCAPath,
		},
		{
			Name: internal.DatabaseCAPath,
		},
	}
}

func (o *OutputV1Config) MarshalYAML() (any, error) {
	type raw OutputV1Config
	return encoding.WithTypeHeader((*raw)(o), OutputV1ServiceType)
}

func (o *OutputV1Config) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *OutputV1Config) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw OutputV1Config
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *OutputV1Config) Type() string {
	return OutputV1ServiceType
}

func (o *OutputV1Config) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
