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

// AzureMatcher matches Azure resources.
type AzureMatcher struct {
	// Subscriptions are Azure subscriptions to query for resources.
	Subscriptions []string `yaml:"subscriptions,omitempty"`
	// ResourceGroups are Azure resource groups to query for resources.
	ResourceGroups []string `yaml:"resource_groups,omitempty"`
	// Types are Azure types to match: "mysql", "postgres", "aks", "vm"
	Types []string `yaml:"types,omitempty"`
	// Regions are Azure locations to match for databases.
	Regions []string `yaml:"regions,omitempty"`
	// ResourceTags are Azure tags on resources to match.
	ResourceTags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered Azure nodes.
	InstallParams *InstallParams `yaml:"install,omitempty"`
}

// CheckAndSetDefaultsForAzureMatchers sets the default values for discovery Azure matchers
// and validates the provided types.
func CheckAndSetDefaultsForAzureMatchers(matcherInput []AzureMatcher) error {
	for i := range matcherInput {
		matcher := &matcherInput[i]

		if len(matcher.Types) == 0 {
			return trace.BadParameter("At least one Azure discovery service type must be specified, the supported resource types are: %v",
				services.SupportedAzureMatchers)
		}

		for _, matcherType := range matcher.Types {
			if !slices.Contains(services.SupportedAzureMatchers, matcherType) {
				return trace.BadParameter("Azure discovery service type does not support %q resource type; supported resource types are: %v",
					matcherType, services.SupportedAzureMatchers)
			}
		}

		if slices.Contains(matcher.Types, services.AzureMatcherVM) {
			if err := checkAndSetDefaultsForAzureInstaller(matcher); err != nil {
				return trace.Wrap(err)
			}
		}

		if slices.Contains(matcher.Regions, types.Wildcard) || len(matcher.Regions) == 0 {
			matcher.Regions = []string{types.Wildcard}
		}

		if slices.Contains(matcher.Subscriptions, types.Wildcard) || len(matcher.Subscriptions) == 0 {
			matcher.Subscriptions = []string{types.Wildcard}
		}

		if slices.Contains(matcher.ResourceGroups, types.Wildcard) || len(matcher.ResourceGroups) == 0 {
			matcher.ResourceGroups = []string{types.Wildcard}
		}

		if len(matcher.ResourceTags) == 0 {
			matcher.ResourceTags = map[string]apiutils.Strings{
				types.Wildcard: {types.Wildcard},
			}
		}

	}
	return nil
}

func checkAndSetDefaultsForAzureInstaller(matcher *AzureMatcher) error {
	if matcher.InstallParams == nil {
		matcher.InstallParams = &InstallParams{
			JoinParams: types.JoinParams{
				TokenName: defaults.AzureInviteTokenName,
				Method:    types.JoinMethodAzure,
			},
			ScriptName: installers.InstallerScriptName,
		}
		return nil
	}

	switch matcher.InstallParams.JoinParams.Method {
	case types.JoinMethodAzure, "":
		matcher.InstallParams.JoinParams.Method = types.JoinMethodAzure
	default:
		return trace.BadParameter("only Azure joining is supported for Azure auto-discovery")
	}

	if token := matcher.InstallParams.JoinParams.TokenName; token == "" {
		matcher.InstallParams.JoinParams.TokenName = defaults.AzureInviteTokenName
	}

	if installer := matcher.InstallParams.ScriptName; installer == "" {
		matcher.InstallParams.ScriptName = installers.InstallerScriptName
	}
	return nil
}
