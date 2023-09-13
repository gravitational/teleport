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
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// AWSMatcher matches AWS EC2 instances and AWS Databases
type AWSMatcher struct {
	// Types are AWS database types to match, "ec2", "rds", "redshift", "elasticache",
	// or "memorydb".
	Types []string `yaml:"types,omitempty"`
	// Regions are AWS regions to query for databases.
	Regions []string `yaml:"regions,omitempty"`
	// AssumeRoleARN is the AWS role to assume for database discovery.
	AssumeRoleARN string `yaml:"assume_role_arn,omitempty"`
	// ExternalID is the AWS external ID to use when assuming a role for
	// database discovery in an external AWS account.
	ExternalID string `yaml:"external_id,omitempty"`
	// Tags are AWS tags to match.
	Tags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered EC2 nodes
	InstallParams *InstallParams `yaml:"install,omitempty"`
	// SSM provides options to use when sending a document command to
	// an EC2 node
	SSM AWSSSM `yaml:"ssm,omitempty"`
}

// AWSSSM provides options to use when executing SSM documents
type AWSSSM struct {
	// DocumentName is the name of the document to use when executing an
	// SSM command
	DocumentName string `yaml:"document_name,omitempty"`
}

// CheckAndSetDefaultsForAWSMatchers sets the default values for discovery AWS matchers
// and validates the provided types.
func CheckAndSetDefaultsForAWSMatchers(matcherInput []AWSMatcher) error {
	for i := range matcherInput {
		matcher := &matcherInput[i]
		for _, matcherType := range matcher.Types {
			if !slices.Contains(services.SupportedAWSMatchers, matcherType) {
				return trace.BadParameter("discovery service type does not support %q, supported resource types are: %v",
					matcherType, services.SupportedAWSMatchers)
			}
		}

		for _, region := range matcher.Regions {
			if err := awsapiutils.IsValidRegion(region); err != nil {
				return trace.BadParameter("discovery service does not support region %q; supported regions are: %v",
					region, awsutils.GetKnownRegions())
			}
		}

		if matcher.AssumeRoleARN != "" {
			_, err := awsutils.ParseRoleARN(matcher.AssumeRoleARN)
			if err != nil {
				return trace.Wrap(err, "discovery service AWS matcher assume_role_arn is invalid")
			}
		} else if matcher.ExternalID != "" {
			for _, t := range matcher.Types {
				if !slices.Contains(services.RequireAWSIAMRolesAsUsersMatchers, t) {
					return trace.BadParameter("discovery service AWS matcher assume_role_arn is empty, but has external_id %q",
						matcher.ExternalID)
				}
			}
		}

		if matcher.Tags == nil || len(matcher.Tags) == 0 {
			matcher.Tags = map[string]apiutils.Strings{types.Wildcard: {types.Wildcard}}
		}

		var installParams types.InstallerParams
		var err error

		if matcher.InstallParams == nil {
			matcher.InstallParams = &InstallParams{
				JoinParams: types.JoinParams{
					TokenName: defaults.IAMInviteTokenName,
					Method:    types.JoinMethodIAM,
				},
				ScriptName:      installers.InstallerScriptName,
				InstallTeleport: "",
				SSHDConfig:      defaults.SSHDConfigPath,
			}
			installParams, err = matcher.InstallParams.Parse()
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			if method := matcher.InstallParams.JoinParams.Method; method == "" {
				matcher.InstallParams.JoinParams.Method = types.JoinMethodIAM
			} else if method != types.JoinMethodIAM {
				return trace.BadParameter("only IAM joining is supported for EC2 auto-discovery")
			}

			if matcher.InstallParams.JoinParams.TokenName == "" {
				matcher.InstallParams.JoinParams.TokenName = defaults.IAMInviteTokenName
			}

			if matcher.InstallParams.SSHDConfig == "" {
				matcher.InstallParams.SSHDConfig = defaults.SSHDConfigPath
			}

			installParams, err = matcher.InstallParams.Parse()
			if err != nil {
				return trace.Wrap(err)
			}

			if installer := matcher.InstallParams.ScriptName; installer == "" {
				if installParams.InstallTeleport {
					matcher.InstallParams.ScriptName = installers.InstallerScriptName
				} else {
					matcher.InstallParams.ScriptName = installers.InstallerScriptNameAgentless
				}
			}
		}

		if matcher.SSM.DocumentName == "" {
			if installParams.InstallTeleport {
				matcher.SSM.DocumentName = defaults.AWSInstallerDocument
			} else {
				matcher.SSM.DocumentName = defaults.AWSAgentlessInstallerDocument
			}
		}
	}
	return nil
}
