package main

import "fmt"

type buildType struct {
	os   string
	arch string
	fips bool
}

// makefileTarget gets the correct Makefile target for a given arch/fips combo
func makefileTarget(params buildType) string {
	makefileTarget := fmt.Sprintf("release-%s", params.arch)
	if params.fips {
		makefileTarget += "-fips"
	}
	return makefileTarget
}

// pushPipelines builds all applicable push pipeline combinations
func pushPipelines() []pipeline {
	var ps []pipeline
	for _, arch := range []string{"amd64", "386", "arm", "arm64"} {
		for _, fips := range []bool{false, true} {
			if arch == "386" && fips {
				// FIPS mode not supported on i386
				continue
			}
			ps = append(ps, pushPipeline(buildType{os: "linux", arch: arch, fips: fips}))
		}
	}
	// Only amd64 Windows is supported for now.
	ps = append(ps, pushPipeline(buildType{os: "windows", arch: "amd64"}))
	return ps
}

// pushPipeline generates a push pipeline for a given combination of os/arch/FIPS
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
		"ARCH":   value{raw: params.arch},
	}
	if params.fips {
		pipelineName += "-fips"
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
			Commands: buildCheckoutCommands(params.fips),
		},
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Environment: pushEnvironment,
			Volumes: []volumeRef{
				{Name: "dockersock", Path: "/var/run"},
			},
			Commands: []string{
				`apk add --no-cache make`,
				`chown -R $UID:$GID /go`,
				`cd /go/src/github.com/gravitational/teleport`,
				fmt.Sprintf(`make -C build.assets %s`, makefileTarget(params)),
			},
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
