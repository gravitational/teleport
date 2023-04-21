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

// This file contains variables and functions to make testing of the container image build process
// more simple and easier.

// To run one of these pipelines locally:
// # Drone requires certain variables to be set
// export DRONE_REMOTE_URL="https://github.com/gravitational/teleport"
// export DRONE_SOURCE_BRANCH="$(git branch --show-current)"
// # `drone exec` does not support `exec` or `kubernetes` pipelines
// sed -i '' 's/type\: kubernetes/type\: docker/' .drone.yml && sed -i '' 's/type\: exec/type\: docker/' .drone.yml
// # Drone has a bug where "workspace" is appended to "/drone/src". This fixes that by updating references
// sed -i '' 's~/go/~/drone/src/go/~g' .drone.yml
// # Pull the current branch instead of v11
// sed -i '' "s~git checkout -qf \"\$(cat '/go/vars/full-version/v11')\"~git checkout -qf \"${DRONE_SOURCE_BRANCH}\"~" .drone.yml
// # `drone exec` does not properly map the workspace path. This creates a volume to be shared between steps
// #  at the correct path
// DOCKER_VOLUME_NAME="go"
// docker volume create "$DOCKER_VOLUME_NAME"
// drone exec --trusted --pipeline teleport-container-images-current-version-cron --clone=false --volume "${DOCKER_VOLUME_NAME}:/go"
// # Cleanup
// docker volume rm "$DOCKER_VOLUME_NAME"

// If you are working on a PR/testing changes to this file you should configure the following for Drone testing:
// 1. Publish the branch you're working on
// 2. Set `prBranch` to the name of the branch in (1)
// 3. Set `configureForPRTestingOnly` to true
// 4. Create a public and private ECR repos for "teleport", "teleport-ent", "teleport-operator", "teleport-lab"
// 5. Set `testingECRRegistryOrg` to the org name(s) used in (4)
// 6. Set the `ECRTestingDomain` to the domain used for the private ECR repos
// 7. Create two separate IAM users, each with full access to either the public ECR repo OR the private ECR repo
// 8. Set the Drone secrets for the secret names listed in "GetContainerRepos" to the credentials in (7, 8), prefixed by the value of `testingSecretPrefix`
//
// On each commit, after running `make dronegen``, run the following commands and resign the file:
// # Pull the current branch instead of v11 so the appropriate dockerfile gets loaded
// sed -i '' "s~git checkout -qf \"\$(cat '/go/vars/full-version/v11')\"~git checkout -qf \"${DRONE_SOURCE_BRANCH}\"~" .drone.yml
//
// When finishing up your PR check the following:
// * The testing secrets added to Drone have been removed
// * `configureForPRTestingOnly` has been set to false, and `make dronegen` has been reran afterwords

const (
	configureForPRTestingOnly bool   = false
	testingSecretPrefix       string = "TEST_"
	testingECRRegistryOrg     string = "u8j2q1d9"
	testingECRRegion          string = "us-east-2"
	prBranch                  string = "" // "fred/multiarch-teleport-actual-container-images"
	testingECRDomain          string = "278576220453.dkr.ecr.us-east-2.amazonaws.com"
)

const (
	ProductionRegistryOrg string = "gravitational"
	PublicEcrRegion       string = "us-east-1"
	StagingEcrRegion      string = "us-west-2"
)

func NewTestTrigger(triggerBranch, testMajorVersion string) *TriggerInfo {
	// baseTrigger := NewTagTrigger(testMajorVersion)
	// baseTrigger := NewPromoteTrigger(testMajorVersion)
	baseTrigger := NewCronTrigger([]string{testMajorVersion})
	baseTrigger.Name = "Test trigger on push"
	baseTrigger.Trigger = trigger{
		Repo:   triggerRef{Include: []string{"gravitational/teleport"}},
		Event:  triggerRef{Include: []string{"push"}},
		Branch: triggerRef{Include: []string{triggerBranch}},
	}

	return baseTrigger
}
