// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"path/filepath"
)

// awsRoleSettings contains the information necessary to assume an AWS Role
//
// This is intended to be imbedded, please use the kubernetes/mac/windows versions
// with their corresponding pipelines.
type awsRoleSettings struct {
	awsAccessKeyID     value
	awsSecretAccessKey value
	role               value
}

// kuberentesRoleSettings contains the info necessary to assume an AWS role and save the credentials to a volume that later steps can use
type kubernetesRoleSettings struct {
	awsRoleSettings
	configVolume volumeRef
	name         string
	profile      string
	append       bool
}

// kuberentesS3Settings contains all info needed to download from S3 in a kubernetes pipeline
type kubernetesS3Settings struct {
	region       string
	source       string
	target       string
	configVolume volumeRef
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

// kubernetesUploadToS3Step generates an S3 upload step
func kubernetesUploadToS3Step(s kubernetesS3Settings) step {
	return step{
		Name:  "Upload to S3",
		Image: "amazon/aws-cli",
		Pull:  "if-not-exists",
		Environment: map[string]value{
			"AWS_S3_BUCKET": {fromSecret: "AWS_S3_BUCKET"},
			"AWS_REGION":    {raw: s.region},
		},
		Volumes: []volumeRef{s.configVolume},
		Commands: []string{
			`cd ` + s.source,
			`aws s3 sync . s3://$AWS_S3_BUCKET/` + s.target,
		},
	}
}
