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
	// AzureInviteTokenName is the name of the default token to use
	// when templating the script to be executed on Azure.
	AzureInviteTokenName = "azure-discovery-token"

	// AzureMatcherVM is the Azure matcher type for Azure VMs.
	AzureMatcherVM = "vm"
	// AzureMatcherKubernetes is the Azure matcher type for Azure Kubernetes.
	AzureMatcherKubernetes = "aks"
	// AzureMatcherMySQL is the Azure matcher type for Azure MySQL databases.
	AzureMatcherMySQL = "mysql"
	// AzureMatcherPostgres is the Azure matcher type for Azure Postgres databases.
	AzureMatcherPostgres = "postgres"
	// AzureMatcherRedis is the Azure matcher type for Azure Cache for Redis databases.
	AzureMatcherRedis = "redis"
	// AzureMatcherSQLServer is the Azure matcher type for SQL Server databases.
	AzureMatcherSQLServer = "sqlserver"
)

// SupportedAzureMatchers is list of Azure services currently supported by the
// Teleport discovery service.
// IMPORTANT: when adding new Database matchers, make sure reference configs
// for both Discovery and Database Service are updated in docs.
var SupportedAzureMatchers = []string{
	AzureMatcherVM,
	AzureMatcherKubernetes,
	AzureMatcherMySQL,
	AzureMatcherPostgres,
	AzureMatcherRedis,
	AzureMatcherSQLServer,
}

// GetTypes gets the types that the matcher can match.
func (m AzureMatcher) GetTypes() []string {
	return m.Types
}

// CopyWithTypes copies the matcher with new types.
func (m AzureMatcher) CopyWithTypes(t []string) Matcher {
	newMatcher := m
	newMatcher.Types = t
	return newMatcher
}

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (m *AzureMatcher) CheckAndSetDefaults() error {
	if len(m.Types) == 0 {
		return trace.BadParameter("At least one Azure discovery service type must be specified, the supported resource types are: %v",
			SupportedAzureMatchers)
	}

	for _, matcherType := range m.Types {
		if !slices.Contains(SupportedAzureMatchers, matcherType) {
			return trace.BadParameter("Azure discovery service type does not support %q resource type; supported resource types are: %v",
				matcherType, SupportedAzureMatchers)
		}
	}

	if slices.Contains(m.Types, AzureMatcherVM) {
		if m.Params == nil {
			m.Params = &InstallerParams{}
		}
		if m.Params.Azure == nil {
			m.Params.Azure = &AzureInstallerParams{}
		}

		if m.Params.Suffix != "" {
			if !isAlphanumericIncluding(m.Params.Suffix, '-') {
				return trace.BadParameter("install.suffix can only contain alphanumeric characters and hyphens")
			}
		}

		if m.Params.UpdateGroup != "" {
			if !isAlphanumericIncluding(m.Params.UpdateGroup, '-') {
				return trace.BadParameter("install.update_group can only contain alphanumeric characters and hyphens")
			}
		}

		switch m.Params.JoinMethod {
		case JoinMethodAzure, "":
			m.Params.JoinMethod = JoinMethodAzure
		default:
			return trace.BadParameter("only Azure joining is supported for Azure auto-discovery")
		}

		if m.Params.JoinToken == "" {
			m.Params.JoinToken = AzureInviteTokenName
		}

		if m.Params.ScriptName == "" {
			m.Params.ScriptName = DefaultInstallerScriptName
		}
	}

	if slices.Contains(m.Regions, Wildcard) || len(m.Regions) == 0 {
		m.Regions = []string{Wildcard}
	}

	if slices.Contains(m.Subscriptions, Wildcard) || len(m.Subscriptions) == 0 {
		m.Subscriptions = []string{Wildcard}
	}

	if slices.Contains(m.ResourceGroups, Wildcard) || len(m.ResourceGroups) == 0 {
		m.ResourceGroups = []string{Wildcard}
	}

	if len(m.ResourceTags) == 0 {
		m.ResourceTags = map[string]apiutils.Strings{
			Wildcard: {Wildcard},
		}
	}
	return nil
}
