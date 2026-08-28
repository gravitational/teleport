/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package deployserviceconfig

import (
	"encoding/base64"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// DefaultTeleportIAMTokenName is the default Teleport IAM Token to use when it's not specified.
	DefaultTeleportIAMTokenName = "discover-aws-oidc-iam-token"
)

// GenerateTeleportConfigString creates a teleport.yaml configuration that the agent
// deployed in a ECS Cluster (using Fargate) will use.
//
// Returns config as base64-encoded string suitable for passing to teleport process
// via --config-string flag.
func GenerateTeleportConfigString(proxyHostPort, iamTokenName string, resourceMatcherLabels types.Labels) (string, error) {
	teleportConfig, err := config.MakeSampleFileConfig(config.SampleFlags{
		Version:      defaults.TeleportConfigVersionV3,
		ProxyAddress: proxyHostPort,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	teleportConfig.Logger.Severity = teleport.DebugLevel

	// Disable default services
	teleportConfig.Auth.EnabledFlag = "no"
	teleportConfig.Proxy.EnabledFlag = "no"
	teleportConfig.SSH.EnabledFlag = "no"

	// Ensure the NodeName is not set to the current host (Teleport Proxy).
	// Setting it to an empty string, ensures the NodeName is picked up from the host's hostname.
	teleportConfig.NodeName = ""

	// Use IAM Token join method to enroll into the Cluster.
	// req.TeleportIAMTokenName must have the following TokenRule:
	/*
		types.TokenRule{
			AWSAccount: "<account-id>",
			AWSARN:     "arn:aws:sts::<account-id>:assumed-role/<taskRoleARN>/*",
		}
	*/
	teleportConfig.JoinParams = config.JoinParams{
		TokenName: iamTokenName,
		Method:    types.JoinMethodIAM,
	}

	teleportConfig.Databases.Service.EnabledFlag = "yes"
	teleportConfig.Databases.ResourceMatchers = []config.ResourceMatcher{{
		Labels: resourceMatcherLabels,
	}}

	teleportConfigYAMLBytes, err := yaml.Marshal(teleportConfig)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// This Config is meant to be passed as argument to `teleport start`
	// Eg, `teleport start --config-string <X>`
	teleportConfigString := base64.StdEncoding.EncodeToString(teleportConfigYAMLBytes)

	return teleportConfigString, nil
}

// ParseResourceLabelMatchers receives a teleport config string and returns the Resource Matcher Label.
// The expected input is a base64 encoded yaml string containing a teleport configuration,
// the same format that GenerateTeleportConfigString returns.
func ParseResourceLabelMatchers(teleportConfigStringBase64 string) (types.Labels, error) {
	teleportConfigString, err := base64.StdEncoding.DecodeString(teleportConfigStringBase64)
	if err != nil {
		return nil, trace.BadParameter("invalid base64 value, error=%v", err)
	}

	var teleportConfig config.FileConfig
	if err := yaml.Unmarshal(teleportConfigString, &teleportConfig); err != nil {
		return nil, trace.BadParameter("invalid teleport config, error=%v", err)
	}

	if len(teleportConfig.Databases.ResourceMatchers) == 0 {
		return nil, trace.BadParameter("valid yaml configuration but db_service.resources has 0 items")
	}

	resourceMatchers := teleportConfig.Databases.ResourceMatchers[0]

	return resourceMatchers.Labels, nil
}
