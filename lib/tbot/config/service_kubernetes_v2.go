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

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

var (
	_ ServiceConfig = &KubernetesV2Output{}
	_ Initable      = &KubernetesV2Output{}
)

const KubernetesV2OutputType = "kubernetes/v2"

// KubernetesOutput produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type KubernetesV2Output struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`

	// DisableExecPlugin disables the default behavior of using `tbot` as a
	// `kubectl` credentials exec plugin. This is useful in environments where
	// `tbot` may not exist on the system that will consume the outputted
	// kubeconfig. It does mean that kubectl will not be able to automatically
	// refresh the credentials within an individual invocation.
	DisableExecPlugin bool `yaml:"disable_exec_plugin,omitempty"`

	// Selectors is a list of selectors for path-based routing. Multiple
	// selectors can be used to generate an output containing all matches.
	Selectors []*KubernetesSelector `yaml:"selectors,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

func (o *KubernetesV2Output) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}

	if len(o.Selectors) == 0 {
		return trace.BadParameter("at least one selector must be provided")
	}

	for _, s := range o.Selectors {
		if err := s.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(o.Destination.CheckAndSetDefaults())
}

func (o *KubernetesV2Output) GetDestination() bot.Destination {
	return o.Destination
}

func (o *KubernetesV2Output) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *KubernetesV2Output) Describe() []FileDescription {
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
	}
}

func (o *KubernetesV2Output) MarshalYAML() (any, error) {
	type raw KubernetesV2Output
	return withTypeHeader((*raw)(o), KubernetesV2OutputType)
}

func (o *KubernetesV2Output) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw KubernetesV2Output
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *KubernetesV2Output) Type() string {
	return KubernetesV2OutputType
}

// KubernetesSelector allows querying for a Kubernetes cluster to include either
// by its name or labels.
type KubernetesSelector struct {
	Name string `yaml:"name,omitempty"`

	Labels map[string]string `yaml:"labels,omitempty"`
}

func (s *KubernetesSelector) CheckAndSetDefaults() error {
	if s.Name == "" && len(s.Labels) == 0 {
		return trace.BadParameter("selectors: one of 'name' and 'labels' must be specified")
	}

	if s.Name != "" && len(s.Labels) > 0 {
		return trace.BadParameter("selectors: only one of 'name' and 'labels' may be specified")
	}

	if s.Labels == nil {
		s.Labels = map[string]string{}
	}

	return nil
}

func (s *KubernetesSelector) UnmarshalYAML(value *yaml.Node) error {
	// A custom unmarshaler so Labels is consistently initialized to not-nil.
	// Primarily needed for tests.
	type temp KubernetesSelector
	out := temp{
		Labels: make(map[string]string),
	}

	if err := value.Decode(&out); err != nil {
		return err
	}

	*s = KubernetesSelector(out)
	return nil
}

func (o *KubernetesV2Output) GetCredentialLifetime() CredentialLifetime {
	return o.CredentialLifetime
}
