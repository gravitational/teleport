package main

import "fmt"

var (
	triggerPullRequest = trigger{
		Event: triggerRef{Include: []string{"pull_request"}},
		Repo:  triggerRef{Include: []string{"gravitational/*"}},
	}
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
	volumeTmpDind = volume{
		Name: "tmp-dind",
		Temp: &volumeTemp{},
	}
	volumeTmpIntegration = volume{
		Name: "tmp-integration",
		Temp: &volumeTemp{},
	}

	volumeRefTmpfs = volumeRef{
		Name: "tmpfs",
		Path: "/tmpfs",
	}
	volumeRefDocker = volumeRef{
		Name: "dockersock",
		Path: "/var/run",
	}
	volumeRefTmpDind = volumeRef{
		Name: "tmp-dind",
		Path: "/tmp",
	}
	volumeRefTmpIntegration = volumeRef{
		Name: "tmp-integration",
		Path: "/tmp",
	}

	// TODO(gus): Set this from `make -C build.assets print-runtime-version` or similar rather
	// than hardcoding it. Also remove the usage of RUNTIME as a pipeline-level environment variable
	// (as support for these varies among Drone runners) and only set it for steps that need it.
	goRuntime = value{raw: "go1.16.2"}
)

type buildType struct {
	os      string
	arch    string
	fips    bool
	centos6 bool
}

// dockerService generates a docker:dind service
// It includes the Docker socket volume by default, plus any extra volumes passed in
func dockerService(v ...volumeRef) service {
	return service{
		Name:    "Start Docker",
		Image:   "docker:dind",
		Volumes: append(v, volumeRefDocker),
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
	return makefileTarget
}
