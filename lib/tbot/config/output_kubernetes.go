/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
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
}

func (o *KubernetesOutput) templates() []template {
	return []template{
		&templateTLSCAs{},
		&templateIdentity{},
		&templateKubernetes{
			clusterName:          o.KubernetesCluster,
			executablePathGetter: os.Executable,
		},
	}
}

func (o *KubernetesOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	dest := o.GetDestination()
	if err := identity.SaveIdentity(ident, dest, identity.DestinationKinds()...); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, dest); err != nil {
			return trace.Wrap(err, "rendering template %s", t.name())
		}
	}

	return nil
}

func (o *KubernetesOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Destination.Init(subDirs))
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

func (o *KubernetesOutput) GetRoles() []string {
	return o.Roles
}

func (o *KubernetesOutput) Describe() []FileDescription {
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
	}

	return fds
}

func (o KubernetesOutput) MarshalYAML() (interface{}, error) {
	type raw KubernetesOutput
	return withTypeHeader(raw(o), KubernetesOutputType)
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

func (o *KubernetesOutput) String() string {
	return fmt.Sprintf("%s (%s)", KubernetesOutputType, o.GetDestination())
}
