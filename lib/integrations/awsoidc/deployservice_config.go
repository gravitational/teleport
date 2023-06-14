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

package awsoidc

import (
	"encoding/base64"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
)

// generateTeleportConfigString creates a teleport.yaml configuration
func generateTeleportConfigString(req DeployServiceRequest) (string, error) {
	teleportConfig, err := config.MakeSampleFileConfig(config.SampleFlags{
		Version:      defaults.TeleportConfigVersionV3,
		ProxyAddress: req.ProxyServerHostPort,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Disable default services
	teleportConfig.Auth.EnabledFlag = "no"
	teleportConfig.Proxy.EnabledFlag = "no"
	teleportConfig.SSH.EnabledFlag = "no"

	// Ensure the NodeName is not set to the current host (Teleport Proxy).
	// Setting it to an empty string, ensures the NodeName is picked up from the host's hostname.
	teleportConfig.NodeName = ""

	// Use IAM Token join method to enroll into the Cluster.
	// iam-token must have the following TokenRule:
	/*
		types.TokenRule{
			AWSAccount: "<account-id>",
			AWSARN:     "arn:aws:sts::<account-id>:assumed-role/<taskRoleARN>/*",
		}
	*/
	teleportConfig.JoinParams = config.JoinParams{
		TokenName: string(types.JoinMethodIAM) + "-token",
		Method:    types.JoinMethodIAM,
	}

	switch req.DeploymentMode {
	case DatabaseServiceDeploymentMode:
		teleportConfig.Databases.Service.EnabledFlag = "yes"
		teleportConfig.Databases.ResourceMatchers = []config.ResourceMatcher{{
			Labels: req.DatabaseResourceMatcherLabels,
		}}

	default:
		return "", trace.BadParameter("invalid deployment mode, supported modes: %v", DeploymentModes)
	}

	teleportConfigYAMLBytes, err := yaml.Marshal(teleportConfig)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// This Config is meant to be passed as argument to `teleport start`
	// Eg, `teleport start --config-string <X>`
	teleportConfigString := base64.StdEncoding.EncodeToString(teleportConfigYAMLBytes)

	return teleportConfigString, nil
}
