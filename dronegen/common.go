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
	volumeDockerTmpfs = volume{
		Name: "dockertmpfs",
		Temp: &volumeTemp{},
	}
	volumeTmpfs = volume{
		Name: "tmpfs",
		Temp: &volumeTemp{Medium: "memory"},
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
	volumeRefDockerTmpfs = volumeRef{
		Name: "dockertmpfs",
		Path: "/var/lib/docker",
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

// boringCryptoRuntime specifies the version of Go (as the branch name)
// used for FIPS builds
const boringCryptoRuntime = "dev.boringcrypto.go1.16"

func (r buildType) platform() platform {
	return platform{
		OS:   r.os,
		Arch: r.arch,
	}
}

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

func sendSlackNotification() step {
	return step{
		Name:  "Send Slack notification (exec)",
		Image: "plugins/slack",
		Settings: map[string]value{
			"webhook": {fromSecret: "SLACK_WEBHOOK_DEV_TELEPORT"},
		},
		Template: []string{
			`*{{#success build.status}}✔{{ else }}✘{{/success}} {{ uppercasefirst build.status }}: Build #{{ build.number }}* (type: ` + "`{{ build.event }}`" + `)
			` + "`${DRONE_STAGE_NAME}`" + ` artifact build failed.
			*Warning:* This is a genuine failure to build the Teleport binary from ` + "`{{ build.branch }}`" + ` (likely due to a bad merge or commit) and should be investigated immediately.
			Commit: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commit/{{ build.commit }}|{{ truncate build.commit 8 }}>
			Branch: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commits/{{ build.branch }}|{{ repo.owner }}/{{ repo.name }}:{{ build.branch }}>
			Author: <https://github.com/{{ build.author }}|{{ build.author }}>
			<{{ build.link }}|Visit Drone build page ↗>
			`,
		},
		When: &condition{Status: []string{"failure"}},
	}
}
