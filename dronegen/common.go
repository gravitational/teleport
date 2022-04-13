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

	volumeDocker = volume{
		Name: "dockersock",
		Temp: &volumeTemp{},
	}
	volumeTmpfs = volume{
		Name: "tmpfs",
		Temp: &volumeTemp{Medium: "memory"},
	}
	volumeRefTmpfs = volumeRef{
		Name: "tmpfs",
		Path: "/tmpfs",
	}
	volumeRefDocker = volumeRef{
		Name: "dockersock",
		Path: "/var/run",
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

type buildType struct {
	os              string
	arch            string
	fips            bool
	centos7         bool
	windowsUnsigned bool
}

// Description provides a human-facing description of the artifact, e.g.:
//   Windows 64-bit (tsh client only)
//   Linux ARMv7 (32-bit)
//   MacOS Intel .pkg installer
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

// dockerService generates a docker:dind service
// It includes the Docker socket volume by default, plus any extra volumes passed in
func dockerService(v ...volumeRef) service {
	return service{
		Name:       "Start Docker",
		Image:      "docker:dind",
		Privileged: true,
		Volumes:    append(v, volumeRefDocker),
	}
}

// dockerVolumes returns a slice of volumes
// It includes the Docker socket volume by default, plus any extra volumes passed in
func dockerVolumes(v ...volume) []volume {
	return append(v, volumeDocker)
}

// dockerVolumeRefs returns a slice of volumeRefs
// It includes the Docker socket volumeRef as a default, plus any extra volumeRefs passed in
func dockerVolumeRefs(v ...volumeRef) []volumeRef {
	return append(v, volumeRefDocker)
}

// releaseMakefileTarget gets the correct Makefile target for a given arch/fips/centos combo
func releaseMakefileTarget(b buildType) string {
	makefileTarget := fmt.Sprintf("release-%s", b.arch)
	if b.centos7 {
		makefileTarget += "-centos7"
	}
	if b.fips {
		makefileTarget += "-fips"
	}
	if b.os == "windows" && b.windowsUnsigned {
		makefileTarget = "release-windows-unsigned"
	}
	return makefileTarget
}

// waitForDockerStep returns a step which checks that the Docker socket is active before trying
// to run container operations
func waitForDockerStep() step {
	return step{
		Name:  "Wait for docker",
		Image: "docker",
		Commands: []string{
			`timeout 30s /bin/sh -c 'while [ ! -S /var/run/docker.sock ]; do sleep 1; done'`,
		},
		Volumes: dockerVolumeRefs(),
	}
}
