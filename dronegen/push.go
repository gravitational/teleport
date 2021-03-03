package main

import "fmt"

func pushPipelines() []pipeline {
	return []pipeline{
		pushPipeline(buildType{os: "linux", arch: "amd64"}),
		pushPipeline(buildType{os: "linux", arch: "i386"}),
		pushPipeline(buildType{os: "linux", arch: "amd64", fips: true}),
		pushPipeline(buildType{os: "windows", arch: "amd64"}),
		pushPipeline(buildType{os: "linux", arch: "arm"}),
		pushPipeline(buildType{os: "linux", arch: "arm", fips: true}),
		pushPipeline(buildType{os: "linux", arch: "arm64"}),
		pushPipeline(buildType{os: "linux", arch: "arm64", fips: true}),
	}
}

// translatePipelineArch converts a nominal arch to a Go architecture representation
// e.g. i386 -> 386, x86_64 -> amd64
// This is required because the existing pipeline uses i386 whereas `GOARCH` is expected to be 386
func translatePipelineArch(arch string) string {
	if arch == "i386" {
		return "386"
	}
	return arch
}

func pushBuildCommands(params buildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
	}
	makefileTarget := "release"
	baseDockerImage := "quay.io/gravitational/teleport-buildbox:$RUNTIME"
	var extraDockerImage string
	var makeCommand string

	if params.fips {
		makefileTarget = "release-fips"
		baseDockerImage = "quay.io/gravitational/teleport-buildbox-fips:$RUNTIME"
		if params.arch == "arm" {
			makefileTarget = "release-arm-fips"
			extraDockerImage = "quay.io/gravitational/teleport-buildbox-arm-fips:$RUNTIME"
		} else if params.arch == "arm64" {
			makefileTarget = "release-arm64-fips"
			extraDockerImage = "quay.io/gravitational/teleport-buildbox-arm-fips:$RUNTIME"
		}
		makeCommand = fmt.Sprintf(`make -C build.assets %s VERSION=$VERSION OS=$OS ARCH=$ARCH FIPS=$FIPS`, makefileTarget)
	} else {
		if params.arch == "arm" {
			makefileTarget = "release-arm"
			extraDockerImage = "quay.io/gravitational/teleport-buildbox-arm:$RUNTIME"
		} else if params.arch == "arm64" {
			makefileTarget = "release-arm64"
			extraDockerImage = "quay.io/gravitational/teleport-buildbox-arm:$RUNTIME"
		}
		makeCommand = fmt.Sprintf(`make -C build.assets %s OS=$OS ARCH=$ARCH`, makefileTarget)
	}

	commands = append(commands, fmt.Sprintf(`docker pull %s || true`, baseDockerImage))
	if extraDockerImage != "" {
		commands = append(commands, fmt.Sprintf(`docker pull %s || true`, extraDockerImage))
	}
	commands = append(commands, `cd /go/src/github.com/gravitational/teleport`)
	commands = append(commands, makeCommand)

	return commands
}

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

func pushPipeline(params buildType) pipeline {
	if params.os == "" {
		panic("params.os must be set")
	}
	if params.arch == "" {
		panic("params.arch must be set")
	}

	pipelineName := fmt.Sprintf("push-build-%s-%s", params.os, params.arch)
	pushEnvironment := map[string]value{
		"UID":    value{raw: "1000"},
		"GID":    value{raw: "1000"},
		"GOPATH": value{raw: "/go"},
		"OS":     value{raw: params.os},
		"ARCH":   value{raw: translatePipelineArch(params.arch)},
	}
	if params.fips {
		pipelineName = fmt.Sprintf("push-build-%s-%s-fips", params.os, params.arch)
		pushEnvironment["FIPS"] = value{raw: "yes"}
	}

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"RUNTIME": value{raw: "go1.15.5"},
		"UID":     value{raw: "1000"},
		"GID":     value{raw: "1000"},
	}
	p.Trigger = triggerPush
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{
		{Name: "dockersock", Temp: &volumeTemp{}},
	}
	p.Services = []service{
		{
			Name:  "Start Docker",
			Image: "docker:dind",
			Volumes: []volumeRef{
				{Name: "dockersock", Path: "/var/run"},
			},
		},
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": value{fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Volumes: []volumeRef{
				volumeRefTmpfs,
			},
			Commands: pushCheckoutCommands(params.fips),
		},
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Environment: pushEnvironment,
			Volumes: []volumeRef{
				{Name: "dockersock", Path: "/var/run"},
			},
			Commands: pushBuildCommands(params),
		},
		{
			Name:  "Send Slack notification",
			Image: "plugins/slack",
			Settings: map[string]value{
				"webhook": value{fromSecret: "SLACK_WEBHOOK_DEV_TELEPORT"},
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
