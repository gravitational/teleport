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

package main

import (
	"fmt"
	"path/filepath"
)

// awsRoleSettings contains the information necessary to assume an AWS Role
//
// This is intended to be embedded, please use the kubernetes/mac versions
// with their corresponding pipelines.
type awsRoleSettings struct {
	awsAccessKeyID     value
	awsSecretAccessKey value
	role               value
}

// kubernetesRoleSettings contains the info necessary to assume an AWS role and save the credentials to a volume that later steps can use
type kubernetesRoleSettings struct {
	awsRoleSettings
	configVolume volumeRef
	name         string
	profile      string
	append       bool
}

// assumeRoleCommands is a helper to build the role assumption commands on a *nix platform
func assumeRoleCommands(profile, configPath string, appendFile bool) []string {
	if profile == "" { // set a default profile if none is specified
		profile = "default"
	}

	var redirect string
	if appendFile {
		redirect = ">>"
	} else {
		redirect = ">"
	}

	assumeRoleCmd := fmt.Sprintf(`printf "[%s]\naws_access_key_id = %%s\naws_secret_access_key = %%s\naws_session_token = %%s\n" \
  $(aws sts assume-role \
    --role-arn "$AWS_ROLE" \
    --role-session-name $(echo "drone-${DRONE_REPO}-${DRONE_BUILD_NUMBER}" | sed "s|/|-|g") \
    --query "Credentials.[AccessKeyId,SecretAccessKey,SessionToken]" \
    --output text) \
  %s %s`, profile, redirect, configPath)

	return []string{
		`aws sts get-caller-identity`, // check the original identity
		assumeRoleCmd,
		`unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY`,    // remove original identity from environment
		`aws sts get-caller-identity --profile ` + profile, // check the new assumed identity
	}
}

// kubernetesAssumeAwsRoleStep builds a step to assume an AWS role and save it to a volume that later steps can use
func kubernetesAssumeAwsRoleStep(s kubernetesRoleSettings) step {
	if s.name == "" {
		s.name = "Assume AWS Role"
	}
	configPath := filepath.Join(s.configVolume.Path, "credentials")
	return step{
		Name:  s.name,
		Image: "amazon/aws-cli",
		Pull:  "if-not-exists",
		Environment: map[string]value{
			"AWS_ACCESS_KEY_ID":     s.awsAccessKeyID,
			"AWS_SECRET_ACCESS_KEY": s.awsSecretAccessKey,
			"AWS_ROLE":              s.role,
		},
		Volumes:  []volumeRef{s.configVolume},
		Commands: assumeRoleCommands(s.profile, configPath, s.append),
	}
}
