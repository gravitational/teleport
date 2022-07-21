// Copyright 2021 Gravitational, Inc
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
	"path"
)

// This function calls the build-apt-repos tool which handles the APT portion of RFD 0058.
func promoteYumPipeline() pipeline {
	return getYumPipelineBuilder().buildPromoteOsPackagePipeline()
}

func migrateYumPipeline(triggerBranch string, migrationVersions []string) pipeline {
	return getYumPipelineBuilder().buildMigrateOsPackagePipeline(triggerBranch, migrationVersions)
}

func getYumPipelineBuilder() *OsPackageToolPipelineBuilder {
	optpb := NewOsPackageToolPipelineBuilder(
		"drone-s3-yumrepo-pvc",
		"ubuntu:22.04",
		"rpm",
		"yum",
		NewRepoBucketSecretNames(
			"YUM_REPO_NEW_AWS_S3_BUCKET",
			"YUM_REPO_NEW_AWS_ACCESS_KEY_ID",
			"YUM_REPO_NEW_AWS_SECRET_ACCESS_KEY",
		),
	)

	optpb.environmentVars["CACHE_DIR"] = value{
		raw: path.Join(optpb.pvcMountPoint, "createrepo_cache"),
	}
	optpb.environmentVars["BUCKET_CACHE_PATH"] = value{
		raw: path.Join(optpb.pvcMountPoint, "bucket"),
	}
	optpb.environmentVars["DEBIAN_FRONTEND"] = value{
		raw: "noninteractive",
	}

	optpb.setupCommands = append(
		[]string{
			"apt update",
			"apt install -y createrepo-c",
			"mkdir -pv \"$CACHE_DIR\"",
		},
		getInstallGoSteps()...,
	)

	optpb.extraArgs = []string{
		"-cache-dir \"$CACHE_DIR\"",
	}

	return optpb
}

func getInstallGoSteps() []string {
	goArchiveName := "go1.18.4.linux-amd64.tar.gz"
	downloadURL := fmt.Sprintf("https://go.dev/dl/%s", goArchiveName)
	downloadPath := path.Join("/", "tmp", goArchiveName)
	installPath := path.Join("/", "usr", "local")
	binPath := path.Join(installPath, "go", "bin")

	// See https://go.dev/doc/install for details
	return []string{
		fmt.Sprintf("curl --silent --show-error --location --output %q %q", downloadPath, downloadURL),
		fmt.Sprintf("tar -C %q -xzf %q", installPath, downloadPath),
		fmt.Sprintf("export PATH=$PATH:%s", binPath),
	}
}
