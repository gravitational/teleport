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
	"github.com/gravitational/teleport/api/types/installers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// GCPMatcher matches GCP resources.
type GCPMatcher struct {
	// Types are GKE resource types to match: "gke", "gce".
	Types []string `yaml:"types,omitempty"`
	// Locations are GKE locations to search resources for.
	Locations []string `yaml:"locations,omitempty"`
	// Tags are GCP labels to match.
	Tags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// ProjectIDs are the GCP project ID where the resources are deployed.
	ProjectIDs []string `yaml:"project_ids,omitempty"`
	// ServiceAccounts are the emails of service accounts attached to VMs.
	ServiceAccounts []string `yaml:"service_accounts,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered GCP VMs.
	InstallParams *InstallParams `yaml:"install,omitempty"`
}

// CheckAndSetDefaultsForGCPMatchers sets the default values for GCP matchers
// and validates the provided types.
func CheckAndSetDefaultsForGCPMatchers(matcherInput []GCPMatcher) error {
	for i := range matcherInput {
		matcher := &matcherInput[i]

		if len(matcher.Types) == 0 {
			return trace.BadParameter("At least one GCP discovery service type must be specified, the supported resource types are: %v",
				services.SupportedGCPMatchers)
		}

		for _, matcherType := range matcher.Types {
			if !slices.Contains(services.SupportedGCPMatchers, matcherType) {
				return trace.BadParameter("GCP discovery service type does not support %q resource type; supported resource types are: %v",
					matcherType, services.SupportedGCPMatchers)
			}
		}

		if slices.Contains(matcher.Types, services.GCPMatcherCompute) {
			if err := checkAndSetDefaultsForGCPInstaller(matcher); err != nil {
				return trace.Wrap(err)
			}
		}

		if slices.Contains(matcher.Locations, types.Wildcard) || len(matcher.Locations) == 0 {
			matcher.Locations = []string{types.Wildcard}
		}

		if slices.Contains(matcher.ProjectIDs, types.Wildcard) {
			return trace.BadParameter("GCP discovery service project_ids does not support wildcards; please specify at least one value in project_ids.")
		}
		if len(matcher.ProjectIDs) == 0 {
			return trace.BadParameter("GCP discovery service project_ids does cannot be empty; please specify at least one value in project_ids.")
		}

		if len(matcher.Tags) == 0 {
			matcher.Tags = map[string]apiutils.Strings{
				types.Wildcard: {types.Wildcard},
			}
		}

	}
	return nil
}

func checkAndSetDefaultsForGCPInstaller(matcher *GCPMatcher) error {
	if matcher.InstallParams == nil {
		matcher.InstallParams = &InstallParams{
			JoinParams: types.JoinParams{
				TokenName: defaults.GCPInviteTokenName,
				Method:    types.JoinMethodGCP,
			},
			ScriptName: installers.InstallerScriptName,
		}
		return nil
	}

	switch matcher.InstallParams.JoinParams.Method {
	case types.JoinMethodGCP, "":
		matcher.InstallParams.JoinParams.Method = types.JoinMethodGCP
	default:
		return trace.BadParameter("only GCP joining is supported for GCP auto-discovery")
	}

	if token := matcher.InstallParams.JoinParams.TokenName; token == "" {
		matcher.InstallParams.JoinParams.TokenName = defaults.GCPInviteTokenName
	}

	if installer := matcher.InstallParams.ScriptName; installer == "" {
		matcher.InstallParams.ScriptName = installers.InstallerScriptName
	}
	return nil
}
