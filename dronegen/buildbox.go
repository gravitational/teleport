package main

import "fmt"

func buildboxPipelineSteps() []step {
	steps := []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: []string{
				`git clone --depth 1 --single-branch --branch ${DRONE_SOURCE_BRANCH} https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
				`git checkout ${DRONE_COMMIT}`,
			},
		},
		waitForDockerStep(),
	}

	for _, name := range []string{"buildbox", "buildbox-centos6", "buildbox-arm"} {
		for _, fips := range []bool{false, true} {
			steps = append(steps, buildboxPipelineStep(name, fips))
		}
	}
	return steps
}

func buildboxPipelineStep(buildboxName string, fips bool) step {
	if fips {
		buildboxName += "-fips"
	}
	return step{
		Name:  buildboxName,
		Image: "docker",
		Environment: map[string]value{
			"QUAYIO_DOCKER_USERNAME": {fromSecret: "QUAYIO_DOCKER_USERNAME"},
			"QUAYIO_DOCKER_PASSWORD": {fromSecret: "QUAYIO_DOCKER_PASSWORD"},
		},
		Failure: "ignore",
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			`apk add --no-cache make`,
			`chown -R $UID:$GID /go`,
			`docker login -u="$$QUAYIO_DOCKER_USERNAME" -p="$$QUAYIO_DOCKER_PASSWORD" quay.io`,
			fmt.Sprintf(`make -C build.assets %s`, buildboxName),
			//fmt.Sprintf(`docker push quay.io/gravitational/teleport-%s:$RUNTIME`, buildboxName),
		},
	}
}

func buildboxPipeline() pipeline {
	p := newKubePipeline("build-buildboxes")
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
		"UID":     {raw: "1000"},
		"GID":     {raw: "1000"},
	}
	p.Trigger = triggerBuildbox
	p.Workspace = workspace{Path: "/go/src/github.com/gravitational/teleport"}
	p.Volumes = dockerVolumes()
	p.Services = []service{
		dockerService(),
	}
	p.Steps = buildboxPipelineSteps()
	return p
}
