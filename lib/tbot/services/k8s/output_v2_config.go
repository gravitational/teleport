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
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const OutputV2ServiceType = "kubernetes/v2"

// OutputV2Config produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type OutputV2Config struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`

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
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`

	// ContextNameTemplate determines the format of context names in the
	// generated kubeconfig. It is a "text/template" string that supports the
	// following variables:
	//
	//   - {{.ClusterName}} - Name of the Teleport cluster
	//   - {{.KubeName}} - Name of the Kubernetes cluster resource
	//
	// By default, the following template will be used: "{{.ClusterName}}-{{.KubeName}}".
	ContextNameTemplate string `yaml:"context_name_template,omitempty"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *OutputV2Config) GetName() string {
	return o.Name
}

func (o *OutputV2Config) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}

	if len(o.Selectors) == 0 {
		return trace.BadParameter("at least one selector must be provided")
	}

	for _, s := range o.Selectors {
		if err := s.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if o.ContextNameTemplate == "" {
		o.ContextNameTemplate = defaultContextNameTemplate
	} else {
		if _, err := kubeconfig.ContextNameFromTemplate(o.ContextNameTemplate, "", ""); err != nil {
			return trace.BadParameter("context_name_template is invalid: %v", err)
		}
	}

	return trace.Wrap(o.Destination.CheckAndSetDefaults())
}

func (o *OutputV2Config) GetDestination() destination.Destination {
	return o.Destination
}

func (o *OutputV2Config) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *OutputV2Config) Describe() []bot.FileDescription {
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
	}
}

func (o *OutputV2Config) MarshalYAML() (any, error) {
	type raw OutputV2Config
	return encoding.WithTypeHeader((*raw)(o), OutputV2ServiceType)
}

func (o *OutputV2Config) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *OutputV2Config) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw OutputV2Config
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *OutputV2Config) Type() string {
	return OutputV2ServiceType
}

// KubernetesSelector allows querying for a Kubernetes cluster to include either
// by its name or labels.
type KubernetesSelector struct {
	Name string `yaml:"name,omitempty"`

	Labels map[string]string `yaml:"labels,omitempty"`

	// DefaultNamespace specifies the default namespace that should be set in
	// the resulting kubeconfig context for clusters yielded by this selector.
	DefaultNamespace string `yaml:"default_namespace,omitempty"`
}

// String returns a human-readable representation of the selector for logs.
func (s *KubernetesSelector) String() string {
	switch {
	case s.Name != "":
		return fmt.Sprintf("name=%s", s.Name)
	case len(s.Labels) != 0:
		labels := make([]string, 0, len(s.Labels))
		for k, v := range s.Labels {
			labels = append(labels, k+"="+v)
		}
		slices.Sort(labels)
		return fmt.Sprintf("labels={%s}", strings.Join(labels, ", "))
	default:
		return "<empty selector>"
	}
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

func (o *OutputV2Config) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
