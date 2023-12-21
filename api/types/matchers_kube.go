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

package types

import (
	"slices"

	"github.com/gravitational/trace"

	apiutils "github.com/gravitational/teleport/api/utils"
)

const (
	// KubernetesMatchersApp is app matcher type for Kubernetes services
	KubernetesMatchersApp = "app"
)

// SupportedKubernetesMatchers is a list of Kubernetes matchers supported by
// Teleport discovery service
var SupportedKubernetesMatchers = []string{
	KubernetesMatchersApp,
}

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (m *KubernetesMatcher) CheckAndSetDefaults() error {
	for _, t := range m.Types {
		if !slices.Contains(SupportedKubernetesMatchers, t) {
			return trace.BadParameter("Kubernetes discovery does not support %q resource type; supported resource types are: %v",
				t, SupportedKubernetesMatchers)
		}
	}

	if len(m.Types) == 0 {
		m.Types = []string{KubernetesMatchersApp}
	}

	if len(m.Namespaces) == 0 {
		m.Namespaces = []string{Wildcard}
	}

	if len(m.Labels) == 0 {
		m.Labels = map[string]apiutils.Strings{Wildcard: {Wildcard}}
	}

	return nil
}
