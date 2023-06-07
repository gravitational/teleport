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

	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const IdentityOutputType = "identity"

// IdentityOutput produces credentials which can be used with `tsh`, `tctl`,
// `openssh` and most SSH compatible tooling. It can also be used with the
// Teleport API and things which use the API client (e.g the terraform provider)
//
// It cannot be used to connect to Applications, Databases or Kubernetes
// Clusters.
type IdentityOutput struct {
	Common OutputCommon `yaml:",inline"`
	// Cluster allows certificates to be generated for a leaf cluster of the
	// cluster that the bot is connected to. These certificates can be used
	// to directly connect to a Teleport proxy of that leaf cluster, or used
	// with the root cluster's proxy which will forward the request to the
	// leaf cluster.
	// For now, only SSH is supported.
	Cluster string `yaml:"cluster,omitempty"`

	destPath string
}

func (o *IdentityOutput) templates() []template {
	return []template{
		&templateTLSCAs{},
		&templateSSHClient{
			getSSHVersion: openssh.GetSystemSSHVersion,
			destPath:      o.destPath,
		},
		&templateIdentity{},
	}
}

func (o *IdentityOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.GetDestination()); err != nil {
			return trace.Wrap(err, "rendering %s", t.name())
		}
	}

	return nil
}

func (o *IdentityOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.GetDestination().Init(subDirs))
}

func (o *IdentityOutput) GetDestination() bot.Destination {
	return o.Common.Destination.Get()
}

func (o *IdentityOutput) GetRoles() []string {
	return o.Common.Roles
}

func (o *IdentityOutput) CheckAndSetDefaults() error {
	if err := o.Common.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	dest, ok := o.Common.Destination.Get().(*DestinationDirectory)
	if ok {
		o.destPath = dest.Path
	} else {
		// If destDir is unset, we're not using a filesystem Destination and
		// ssh_config will not be sensible. Log a note and bail early without
		// writing ssh_config. (Future users of k8s secrets will need to bring
		// their own config, we can't predict where paths will be in practice.)
		log.Infof("Note: no ssh_config will be written for non-filesystem "+
			"Destination %s.", o)
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

func (o IdentityOutput) MarshalYAML() (interface{}, error) {
	type raw IdentityOutput
	return marshalHeadered(raw(o), IdentityOutputType)
}

func (o *IdentityOutput) String() string {
	return fmt.Sprintf("%s (%s)", IdentityOutputType, o.GetDestination())
}
