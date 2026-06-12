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
	// GCPInviteTokenName is the name of the default token to use
	// when templating the script to be executed on GCP.
	GCPInviteTokenName = "gcp-discovery-token"

	// GCPMatcherKubernetes is the GCP matcher type for GCP kubernetes.
	GCPMatcherKubernetes = "gke"
	// GCPMatcherCompute is the GCP matcher for GCP VMs.
	GCPMatcherCompute = "gce"
)

// SupportedGCPMatchers is list of GCP services currently supported by the
// Teleport discovery service.
var SupportedGCPMatchers = []string{
	GCPMatcherKubernetes,
	GCPMatcherCompute,
}

// GetTypes gets the types that the matcher can match.
func (m GCPMatcher) GetTypes() []string {
	return m.Types
}

// CopyWithTypes copies the matcher with new types.
func (m GCPMatcher) CopyWithTypes(t []string) Matcher {
	newMatcher := m
	newMatcher.Types = t
	return newMatcher
}

// GetLabels gets the matcher's labels.
func (m GCPMatcher) GetLabels() Labels {
	if len(m.Labels) != 0 {
		return m.Labels
	}
	// Check Tags as well for backwards compatibility.
	return m.Tags
}

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (m *GCPMatcher) CheckAndSetDefaults() error {
	if len(m.Types) == 0 {
		return trace.BadParameter("At least one GCP discovery service type must be specified, the supported resource types are: %v",
			SupportedGCPMatchers)
	}

	for _, matcherType := range m.Types {
		if !slices.Contains(SupportedGCPMatchers, matcherType) {
			return trace.BadParameter("GCP discovery service type does not support %q resource type; supported resource types are: %v",
				matcherType, SupportedGCPMatchers)
		}
	}

	if slices.Contains(m.Types, GCPMatcherCompute) {
		if m.Params == nil {
			m.Params = &InstallerParams{}
		}

		switch m.Params.JoinMethod {
		case JoinMethodGCP, "":
			m.Params.JoinMethod = JoinMethodGCP
		default:
			return trace.BadParameter("only GCP joining is supported for GCP auto-discovery")
		}

		if m.Params.JoinToken == "" {
			m.Params.JoinToken = GCPInviteTokenName
		}

		if m.Params.ScriptName == "" {
			m.Params.ScriptName = DefaultInstallerScriptName
		}
	}

	if slices.Contains(m.Locations, Wildcard) || len(m.Locations) == 0 {
		m.Locations = []string{Wildcard}
	}

	if slices.Contains(m.ProjectIDs, Wildcard) && len(m.ProjectIDs) > 1 {
		return trace.BadParameter("GCP discovery service either supports wildcard project_ids or multiple values, but not both.")
	}
	if len(m.ProjectIDs) == 0 {
		return trace.BadParameter("GCP discovery service project_ids does cannot be empty; please specify at least one value in project_ids.")
	}

	if len(m.Labels) > 0 && len(m.Tags) > 0 {
		return trace.BadParameter("labels and tags should not both be set.")
	}

	if len(m.Tags) > 0 {
		m.Labels = m.Tags
	}

	if len(m.Labels) == 0 {
		m.Labels = map[string]apiutils.Strings{
			Wildcard: {Wildcard},
		}
	}

	return nil
}
