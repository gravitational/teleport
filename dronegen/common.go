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
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

const (
	// StagingRegistry is the staging registry images are pushed to before being promoted to the production registry.
	StagingRegistry = "146628656107.dkr.ecr.us-west-2.amazonaws.com"

	// ProductionRegistry is the production image registry that hosts are customer facing container images.
	ProductionRegistry = "public.ecr.aws"

	// ProductionRegistryQuay is the production image registry that hosts images on quay.io. Will be deprecated in the future.
	// See RFD 73 - https://github.com/gravitational/teleport/blob/c18c09f5d562dd46a509154eab4295ad39decc3c/rfd/0073-public-image-registry.md
	ProductionRegistryQuay = "quay.io"

	// Go version used by internal tools
	GoVersion = "1.18"

	// The name of this service must match k8s.io/apimachinery/pkg/util/validation `IsDNS1123Subdomain`
	// so that it is resolvable
	// See https://github.com/drone-runners/drone-runner-kube/blob/master/engine/compiler/compiler.go#L398
	// for details
	LocalRegistryHostname string = "drone-docker-registry"
	LocalRegistrySocket   string = LocalRegistryHostname + ":5000"
)

var (
	triggerPush = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}
	triggerTag = trigger{
		Event: triggerRef{Include: []string{"tag"}},
		Ref:   triggerRef{Include: []string{"refs/tags/v*"}},
		Repo:  triggerRef{Include: []string{"gravitational/*"}},
	}
	triggerPromote = trigger{
		Event:  triggerRef{Include: []string{"promote"}},
		Target: triggerRef{Include: []string{"production"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}

	volumeDocker = volume{
		Name: "dockersock",
		Temp: &volumeTemp{},
	}
	volumeRefDocker = volumeRef{
		Name: "dockersock",
		Path: "/var/run",
	}
	volumeTmpfs = volume{
		Name: "tmpfs",
		Temp: &volumeTemp{Medium: "memory"},
	}
	volumeRefTmpfs = volumeRef{
		Name: "tmpfs",
		Path: "/tmpfs",
	}
	volumeAwsConfig = volume{
		Name: "awsconfig",
		Temp: &volumeTemp{},
	}
	volumeRefAwsConfig = volumeRef{
		Name: "awsconfig",
		Path: "/root/.aws",
	}

	// volumeDockerConfig is a temporary volume for storing docker
	// credentials for use with the Docker-in-Docker service we use
	// to isolate the host machines docker daemon from the one used
	// during the build. Mount this any time you use `volumeDocker`
	//
	// Drone claims to destroy the the temp volumes after a workflow
	// has run, so it should be safe to write credentials etc.
	volumeDockerConfig = volume{
		Name: "dockerconfig",
		Temp: &volumeTemp{},
	}

	// volumeRefDockerConfig is how you reference the docker config
	// volume in a workflow step
	volumeRefDockerConfig = volumeRef{
		Name: "dockerconfig",
		Path: "/root/.docker",
	}
)

var buildboxVersion value

var goRuntime value

func init() {
	v, err := exec.Command("make", "-s", "-C", "build.assets", "print-go-version").Output()
	if err != nil {
		log.Fatalf("could not get Go version: %v", err)
	}
	goRuntime = value{raw: string(bytes.TrimSpace(v))}

	v, err = exec.Command("make", "-s", "-C", "build.assets", "print-buildbox-version").Output()
	if err != nil {
		log.Fatalf("could not get buildbox version: %v", err)
	}
	buildboxVersion = value{raw: string(bytes.TrimSpace(v))}
}

func pushTriggerForBranch(branches ...string) trigger {
	t := trigger{
		Event: triggerRef{Include: []string{"push"}},
		Repo:  triggerRef{Include: []string{"gravitational/teleport"}},
	}
	t.Branch.Include = append(t.Branch.Include, branches...)
	return t
}

func cronTrigger(cronJobNames []string) trigger {
	return trigger{
		Cron: triggerRef{Include: cronJobNames},
		Repo: triggerRef{Include: []string{"gravitational/teleport"}},
	}
}

func cloneRepoCommands(cloneDirectory, commit string) []string {
	return []string{
		fmt.Sprintf("mkdir -pv %q", cloneDirectory),
		fmt.Sprintf("cd %q", cloneDirectory),
		"git init",
		"git remote add origin ${DRONE_REMOTE_URL}",
		"git fetch origin --tags",
		fmt.Sprintf("git checkout -qf %q", commit),
	}
}

type buildType struct {
	os              string
	arch            string
	fips            bool
	centos7         bool
	windowsUnsigned bool
}

// Description provides a human-facing description of the artifact, e.g.:
//
//	Windows 64-bit (tsh client only)
//	Linux ARMv7 (32-bit)
//	MacOS Intel .pkg installer
func (b *buildType) Description(packageType string, extraQualifications ...string) string {
	var result string

	var os string
	var arch string
	var darwinArch string
	var bitness int
	var qualifications []string

	switch b.os {
	case "linux":
		os = "Linux"
	case "darwin":
		os = "MacOS"
	case "windows":
		os = "Windows"
	default:
		panic(fmt.Sprintf("unhandled OS: %s", b.os))
	}

	switch b.arch {
	case "arm64":
		arch = "ARM64/ARMv8"
		darwinArch = "Apple Silicon"
		bitness = 64
	case "amd64":
		darwinArch = "Intel"
		bitness = 64

	case "arm":
		arch = "ARMv7"
		fallthrough
	case "386":
		bitness = 32

	default:
		panic(fmt.Sprintf("unhandled arch: %s", b.arch))
	}

	if b.centos7 {
		qualifications = append(qualifications, "RHEL/CentOS 7.x compatible")
	}
	if b.fips {
		qualifications = append(qualifications, "FedRAMP/FIPS")
	}

	qualifications = append(qualifications, extraQualifications...)

	result = os

	if b.os == "darwin" {
		result += fmt.Sprintf(" %s", darwinArch)
	} else {
		// arch is implicit for Windows/Linux i386/amd64
		if arch == "" {
			result += fmt.Sprintf(" %d-bit", bitness)
		} else {
			result += fmt.Sprintf(" %s (%d-bit)", arch, bitness)
		}
	}

	if packageType != "" {
		result += fmt.Sprintf(" %s", packageType)
	}

	if len(qualifications) > 0 {
		result += fmt.Sprintf(" (%s)", strings.Join(qualifications, ", "))
	}
	return result
}

func (b *buildType) hasTeleportConnect() bool {
	return (b.os == "darwin" && b.arch == "amd64") ||
		(b.os == "linux" && b.arch == "amd64" && !b.centos7 && !b.fips)
}

// dockerService generates a docker:dind service
// It includes the Docker socket volume by default, plus any extra volumes passed in
func dockerService(v ...volumeRef) service {
	return service{
		Name:       "Start Docker",
		Image:      "docker:23.0-dind",
		Privileged: true,
		Volumes:    append(v, volumeRefDocker),
	}
}

// Starts a container registry service at `LocalRegistrySocket`
// This can be pushed/pulled to via `docker push/pull <LocalRegistrySocket>:5000/image:tag`
func dockerRegistryService() service {
	return service{
		Name:  LocalRegistryHostname,
		Image: "registry:2",
	}
}

// releaseMakefileTarget gets the correct Makefile target for a given arch/fips/centos combo
func releaseMakefileTarget(b buildType) string {
	makefileTarget := fmt.Sprintf("release-%s", b.arch)
	// All x86_64 binaries are built on CentOS 7 now for better glibc compatibility.
	if b.centos7 || b.arch == "amd64" {
		makefileTarget += "-centos7"
	}
	if b.fips {
		makefileTarget += "-fips"
	}

	// Override Windows targets.
	if b.os == "windows" {
		if b.windowsUnsigned {
			makefileTarget = "release-windows-unsigned"
		} else {
			makefileTarget = "release-windows"
		}
	}

	return makefileTarget
}

// waitForDockerStep returns a step which checks that the Docker socket is active before trying
// to run container operations
func waitForDockerStep() step {
	return step{
		Name:  "Wait for docker",
		Image: "docker",
		Pull:  "if-not-exists",
		Commands: []string{
			`timeout 30s /bin/sh -c 'while [ ! -S /var/run/docker.sock ]; do sleep 1; done'`,
			`printenv DOCKERHUB_PASSWORD | docker login -u="$DOCKERHUB_USERNAME" --password-stdin`,
		},
		Volumes: []volumeRef{volumeRefDocker, volumeRefDockerConfig},
		Environment: map[string]value{
			"DOCKERHUB_USERNAME": {fromSecret: "DOCKERHUB_USERNAME"},
			"DOCKERHUB_PASSWORD": {fromSecret: "DOCKERHUB_READONLY_TOKEN"},
		},
	}
}

// waitForDockerStep returns a step which checks that the Docker registry is ready
func waitForDockerRegistryStep() step {
	return step{
		Name:  "Wait for docker registry",
		Image: "alpine",
		Pull:  "if-not-exists",
		Commands: []string{
			"apk add curl",
			fmt.Sprintf(`timeout 30s /bin/sh -c 'while [ "$(curl -s -o /dev/null -w %%{http_code} http://%s/)" != "200" ]; do sleep 1; done'`, LocalRegistrySocket),
		},
	}
}

func verifyTaggedStep() step {
	return step{
		Name:  "Verify build is tagged",
		Image: "alpine:latest",
		Pull:  "if-not-exists",
		Commands: []string{
			"[ -n ${DRONE_TAG} ] || (echo 'DRONE_TAG is not set. Is the commit tagged?' && exit 1)",
		},
	}
}

// Note that tags are also valid here as a tag refers to a specific commit
func cloneRepoStep(clonePath, commit string) step {
	return step{
		Name:     "Check out code",
		Image:    "alpine/git:latest",
		Pull:     "if-not-exists",
		Commands: cloneRepoCommands(clonePath, commit),
	}
}

func sliceSelect[T, V any](slice []T, selector func(T) V) []V {
	selectedValues := make([]V, len(slice))
	for i, entry := range slice {
		selectedValues[i] = selector(entry)
	}

	return selectedValues
}

func getStepNames(steps []step) []string {
	return sliceSelect(steps, func(s step) string { return s.Name })
}
