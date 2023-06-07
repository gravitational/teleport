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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const KubernetesOutputType = "kubernetes"

// KubernetesOutput produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type KubernetesOutput struct {
	Common OutputCommon `yaml:",inline"`
	// ClusterName is the name of the Kubernetes cluster in Teleport.
	ClusterName string `yaml:"cluster_name"`
}

func (o *KubernetesOutput) templates() []template {
	return []template{
		&templateTLSCAs{},
		&templateIdentity{},
		&templateKubernetes{
			clusterName: o.ClusterName,
		},
	}
}

func (o *KubernetesOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.GetDestination()); err != nil {
			return trace.Wrap(err, "rendering %s", t.name())
		}
	}

	return nil
}

func (o *KubernetesOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Common.Destination.Get().Init(subDirs))
}

func (o *KubernetesOutput) CheckAndSetDefaults() error {
	if o.ClusterName == "" {
		return trace.BadParameter("cluster_name must not be empty")
	}

	return o.Common.CheckAndSetDefaults()
}

func (o *KubernetesOutput) GetDestination() bot.Destination {
	return o.Common.Destination.Get()
}

func (o *KubernetesOutput) GetRoles() []string {
	return o.Common.Roles
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
	return marshalHeadered(raw(o), KubernetesOutputType)
}

func (o *KubernetesOutput) String() string {
	return fmt.Sprintf("%s (%s)", KubernetesOutputType, o.Common.Destination)
}
