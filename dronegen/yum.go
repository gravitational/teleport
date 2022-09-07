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

	optpb.requiredPackages = []string{
		"createrepo-c",
	}

	optpb.setupCommands = []string{
		"mkdir -pv \"$CACHE_DIR\"",
	}

	optpb.extraArgs = []string{
		"-cache-dir \"$CACHE_DIR\"",
	}

	return optpb
}
