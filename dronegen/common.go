package main

import (
	"fmt"
	"strings"
)

var (
	triggerTag = trigger{
		Event: triggerRef{Include: []string{"tag"}},
		Ref:   triggerRef{Include: []string{"refs/tags/v*"}},
		Repo:  triggerRef{Include: []string{"gravitational/*"}},
	}
	triggerPush = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
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
	goRuntime = value{raw: "go1.15.5"}
)

type buildType struct {
	os      string
	arch    string
	fips    bool
	centos6 bool
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

	if b.centos6 {
		qualifications = append(qualifications, "RHEL/CentOS 6.x compatible")
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
