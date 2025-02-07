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
)

var (
	_ ServiceConfig = &KubernetesOutput{}
	_ Initable      = &KubernetesOutput{}
)

const KubernetesOutputType = "kubernetes"

// KubernetesOutput produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type KubernetesOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
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

	// CertificateLifetime contains configuration for how long certificates will
	// last and the frequency at which they'll be renewed.
	CertificateLifetime CertificateLifetime `yaml:",inline"`
}

func (o *KubernetesOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if o.KubernetesCluster == "" {
		return trace.BadParameter("kubernetes_cluster must not be empty")
	}
	return nil
}

func (o *KubernetesOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *KubernetesOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *KubernetesOutput) Describe() []FileDescription {
	// Based on tbot.KubernetesOutputService.Render
	return []FileDescription{
		{
			Name: "kubeconfig.yaml",
		},
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
}

func (o *KubernetesOutput) MarshalYAML() (interface{}, error) {
	type raw KubernetesOutput
	return withTypeHeader((*raw)(o), KubernetesOutputType)
}

func (o *KubernetesOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw KubernetesOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *KubernetesOutput) Type() string {
	return KubernetesOutputType
}

func (o *KubernetesOutput) GetCertificateLifetime() CertificateLifetime {
	return o.CertificateLifetime
}
