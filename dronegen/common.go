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

import "fmt"

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

	// TODO(gus): Set this from `make -C build.assets print-runtime-version` or similar rather
	// than hardcoding it. Also remove the usage of RUNTIME as a pipeline-level environment variable
	// (as support for these varies among Drone runners) and only set it for steps that need it.
	goRuntime = value{raw: "go1.16.14"}
)

type buildType struct {
	os              string
	arch            string
	fips            bool
	centos6         bool
	windowsUnsigned bool
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

// releaseMakefileTarget gets the correct Makefile target for a given arch/fips/centos6 combo
func releaseMakefileTarget(b buildType) string {
	makefileTarget := fmt.Sprintf("release-%s", b.arch)
	if b.centos6 {
		makefileTarget += "-centos6"
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
