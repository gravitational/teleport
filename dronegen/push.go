package main

import "fmt"

// pushCheckoutCommands builds a list of commands for Drone to check out a git commit on a push build
func pushCheckoutCommands(fips bool) []string {
	commands := []string{
		`mkdir -p /go/src/github.com/gravitational/teleport /go/cache`,
		`cd /go/src/github.com/gravitational/teleport`,
		`git init && git remote add origin ${DRONE_REMOTE_URL}`,
		`git fetch origin`,
		`git checkout -qf ${DRONE_COMMIT_SHA}`,
		// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
		`git submodule update --init webassets || true`,
		`mkdir -m 0700 /root/.ssh && echo "$GITHUB_PRIVATE_KEY" > /root/.ssh/id_rsa && chmod 600 /root/.ssh/id_rsa`,
		`ssh-keyscan -H github.com > /root/.ssh/known_hosts 2>/dev/null && chmod 600 /root/.ssh/known_hosts`,
		`git submodule update --init e`,
		// do a recursive submodule checkout to get both webassets and webassets/e
		// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
		`git submodule update --init --recursive webassets || true`,
		`rm -f /root/.ssh/id_rsa`,
	}
	if fips {
		commands = append(commands, `if [[ "${DRONE_TAG}" != "" ]]; then echo "${DRONE_TAG##v}" > /go/.version.txt; else egrep ^VERSION Makefile | cut -d= -f2 > /go/.version.txt; fi; cat /go/.version.txt`)
	}
	return commands
}

// pushBuildCommands generates a list of commands for Drone to build an artifact as part of a push build
func pushBuildCommands(b buildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
		`cd /go/src/github.com/gravitational/teleport`,
	}
	if b.fips {
		commands = append(commands,
			`export VERSION=$(cat /go/.version.txt)`,
		)
	}
	commands = append(commands,
		fmt.Sprintf(`make -C build.assets %s`, releaseMakefileTarget(b)),
	)
	return commands
}

// pushPipelines builds all applicable push pipeline combinations
func pushPipelines() []pipeline {
	var ps []pipeline
	for _, arch := range []string{"amd64", "386", "arm", "arm64"} {
		for _, fips := range []bool{false, true} {
			if arch != "amd64" && fips {
				// FIPS mode only supported on linux/amd64
				continue
			}
			ps = append(ps, pushPipeline(buildType{os: "linux", arch: arch, fips: fips}))
		}
	}
	// Only amd64 Windows is supported for now.
	ps = append(ps, pushPipeline(buildType{os: "windows", arch: "amd64"}))
	// Darwin-specific push pipelines
	ps = append(ps, darwinPushPipelines()...)
	return ps
}

// pushPipeline generates a push pipeline for a given combination of os/arch/FIPS
func pushPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("push-build-%s-%s", b.os, b.arch)
	pushEnvironment := map[string]value{
		"UID":    {raw: "1000"},
		"GID":    {raw: "1000"},
		"GOPATH": {raw: "/go"},
		"OS":     {raw: b.os},
		"ARCH":   {raw: b.arch},
	}
	if b.fips {
		pipelineName += "-fips"
		pushEnvironment["FIPS"] = value{raw: "yes"}
	}

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
		"UID":     {raw: "1000"},
		"GID":     {raw: "1000"},
	}
	p.Trigger = triggerPush
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = dockerVolumes()
	p.Services = []service{
		dockerService(),
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommands(b.fips),
		},
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Environment: pushEnvironment,
			Volumes:     dockerVolumeRefs(),
			Commands:    pushBuildCommands(b),
		},
		{
			Name:  "Send Slack notification",
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
		},
	}
	return p
}
