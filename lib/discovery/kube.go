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

package discovery

import (
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

// KubernetesMatcher matches Kubernetes resources.
type KubernetesMatcher struct {
	// Types are Kubernetes services types to match. Currently only 'app' is supported.
	Types []string `yaml:"types,omitempty"`
	// Namespaces are Kubernetes namespaces in which to discover services
	Namespaces []string `yaml:"namespaces,omitempty"`
	// Labels are Kubernetes services labels to match.
	Labels map[string]apiutils.Strings `yaml:"labels,omitempty"`
}

// CheckAndSetDefaultsForKubeMatchers sets the default values for Kubernetes matchers
// and validates the provided types.
func CheckAndSetDefaultsForKubeMatchers(matchers []KubernetesMatcher) error {
	for i := range matchers {
		matcher := &matchers[i]

		for _, t := range matcher.Types {
			if !slices.Contains(services.SupportedKubernetesMatchers, t) {
				return trace.BadParameter("Kubernetes discovery does not support %q resource type; supported resource types are: %v",
					t, services.SupportedKubernetesMatchers)
			}
		}

		if len(matcher.Types) == 0 {
			matcher.Types = []string{services.KubernetesMatchersApp}
		}

		if len(matcher.Namespaces) == 0 {
			matcher.Namespaces = []string{types.Wildcard}
		}

		if len(matcher.Labels) == 0 {
			matcher.Labels = map[string]apiutils.Strings{types.Wildcard: {types.Wildcard}}
		}
	}

	return nil
}
