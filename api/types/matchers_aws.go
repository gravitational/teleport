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
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
)

const (
	// IAMInviteTokenName is the name of the default Teleport IAM
	// token to use when templating the script to be executed.
	IAMInviteTokenName = "aws-discovery-iam-token"

	// SSHDConfigPath is the path to the sshd config file to modify
	// when using the agentless installer
	SSHDConfigPath = "/etc/ssh/sshd_config"

	// AWSInstallerDocument is the name of the default AWS document
	// that will be called when executing the SSM command.
	AWSInstallerDocument = "TeleportDiscoveryInstaller"

	// AWSAgentlessInstallerDocument is the name of the default AWS document
	// that will be called when executing the SSM command .
	AWSAgentlessInstallerDocument = "TeleportAgentlessDiscoveryInstaller"

	// AWSMatcherEC2 is the AWS matcher type for EC2 instances.
	AWSMatcherEC2 = "ec2"
	// AWSMatcherEKS is the AWS matcher type for AWS Kubernetes.
	AWSMatcherEKS = "eks"
	// AWSMatcherRDS is the AWS matcher type for RDS databases.
	AWSMatcherRDS = "rds"
	// AWSMatcherRDSProxy is the AWS matcher type for RDS Proxy databases.
	AWSMatcherRDSProxy = "rdsproxy"
	// AWSMatcherRedshift is the AWS matcher type for Redshift databases.
	AWSMatcherRedshift = "redshift"
	// AWSMatcherRedshiftServerless is the AWS matcher type for Redshift Serverless databases.
	AWSMatcherRedshiftServerless = "redshift-serverless"
	// AWSMatcherElastiCache is the AWS matcher type for ElastiCache databases.
	AWSMatcherElastiCache = "elasticache"
	// AWSMatcherMemoryDB is the AWS matcher type for MemoryDB databases.
	AWSMatcherMemoryDB = "memorydb"
	// AWSMatcherOpenSearch is the AWS matcher type for OpenSearch databases.
	AWSMatcherOpenSearch = "opensearch"
)

// SupportedAWSMatchers is list of AWS services currently supported by the
// Teleport discovery service.
var SupportedAWSMatchers = append([]string{
	AWSMatcherEC2,
	AWSMatcherEKS,
}, SupportedAWSDatabaseMatchers...)

// SupportedAWSDatabaseMatchers is a list of the AWS databases currently
// supported by the Teleport discovery service.
// IMPORTANT: when adding new Database matchers, make sure reference configs
// for both Discovery and Database Service are updated in docs.
var SupportedAWSDatabaseMatchers = []string{
	AWSMatcherRDS,
	AWSMatcherRDSProxy,
	AWSMatcherRedshift,
	AWSMatcherRedshiftServerless,
	AWSMatcherElastiCache,
	AWSMatcherMemoryDB,
	AWSMatcherOpenSearch,
}

// RequireAWSIAMRolesAsUsersMatchers is a list of the AWS databases that
// require AWS IAM roles as database users.
// IMPORTANT: if you add database matchers for AWS keyspaces, OpenSearch, or
// DynamoDB discovery, add them here and in RequireAWSIAMRolesAsUsers in
// api/types.
var RequireAWSIAMRolesAsUsersMatchers = []string{
	AWSMatcherRedshiftServerless,
	AWSMatcherOpenSearch,
}

// GetTypes gets the types that the matcher can match.
func (m AWSMatcher) GetTypes() []string {
	return m.Types
}

// CopyWithTypes copies the matcher with new types.
func (m AWSMatcher) CopyWithTypes(t []string) Matcher {
	newMatcher := m
	newMatcher.Types = t
	return newMatcher
}

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (m *AWSMatcher) CheckAndSetDefaults() error {
	for _, matcherType := range m.Types {
		if !slices.Contains(SupportedAWSMatchers, matcherType) {
			return trace.BadParameter("discovery service type does not support %q, supported resource types are: %v",
				matcherType, SupportedAWSMatchers)
		}
	}

	if len(m.Types) == 0 {
		return trace.BadParameter("discovery service requires at least one type")
	}

	if len(m.Regions) == 0 {
		return trace.BadParameter("discovery service requires at least one region")
	}

	for _, region := range m.Regions {
		if err := awsapiutils.IsValidRegion(region); err != nil {
			return trace.BadParameter("discovery service does not support region %q", region)
		}
	}

	if m.AssumeRole != nil {
		if m.AssumeRole.RoleARN != "" {
			if err := awsapiutils.CheckRoleARN(m.AssumeRole.RoleARN); err != nil {
				return trace.BadParameter("invalid assume role: %v", err)
			}
		} else if m.AssumeRole.ExternalID != "" {
			for _, t := range m.Types {
				if !slices.Contains(RequireAWSIAMRolesAsUsersMatchers, t) {
					return trace.BadParameter("discovery service AWS matcher assume_role_arn is empty, but has external_id %q",
						m.AssumeRole.ExternalID)
				}
			}
		}
	}

	if m.Tags == nil || len(m.Tags) == 0 {
		m.Tags = map[string]apiutils.Strings{Wildcard: {Wildcard}}
	}

	if m.Params == nil {
		m.Params = &InstallerParams{
			InstallTeleport: true,
		}
	}

	switch m.Params.EnrollMode {
	case InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_UNSPECIFIED:
		m.Params.EnrollMode = InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT
		if m.Integration != "" {
			m.Params.EnrollMode = InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE
		}

	case InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE:
		if m.Integration == "" {
			return trace.BadParameter("integration is required for eice enroll mode")
		}

	case InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT:

	default:
		return trace.BadParameter("invalid enroll mode %s", m.Params.EnrollMode.String())
	}

	switch m.Params.JoinMethod {
	case JoinMethodIAM, "":
		m.Params.JoinMethod = JoinMethodIAM
	default:
		return trace.BadParameter("only IAM joining is supported for EC2 auto-discovery")
	}

	if m.Params.JoinToken == "" {
		m.Params.JoinToken = IAMInviteTokenName
	}

	if m.Params.SSHDConfig == "" {
		m.Params.SSHDConfig = SSHDConfigPath
	}

	if m.Params.ScriptName == "" {
		m.Params.ScriptName = DefaultInstallerScriptNameAgentless
		if m.Params.InstallTeleport {
			m.Params.ScriptName = DefaultInstallerScriptName
		}
	}

	if m.SSM == nil {
		m.SSM = &AWSSSM{}
	}

	if m.SSM.DocumentName == "" {
		m.SSM.DocumentName = AWSAgentlessInstallerDocument
		if m.Params.InstallTeleport {
			m.SSM.DocumentName = AWSInstallerDocument
		}
	}
	return nil
}
