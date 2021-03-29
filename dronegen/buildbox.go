package main

import "fmt"

func buildboxPipelines() []pipeline {
	var pipelines []pipeline
	for _, arch := range []string{"buildbox", "buildbox-centos6", "buildbox-arm"} {
		for _, fips := range []bool{false, true} {
			pipelines = append(pipelines, buildboxPipeline(arch, fips))
		}
	}
	return pipelines
}

func buildboxPipeline(buildboxName string, fips bool) pipeline {
	pipelineName := fmt.Sprintf("build-%s", buildboxName)
	if fips {
		pipelineName += "-fips"
	}

	p := newKubePipeline(pipelineName)
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
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: []string{
				`git clone --shallow-since=$$(date) https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
				`git checkout ${DRONE_COMMIT}`,
			},
		},
		{
			Name:  "Build and push container",
			Image: "docker",
			Environment: map[string]value{
				"QUAYIO_DOCKER_USERNAME": {fromSecret: "QUAYIO_DOCKER_USERNAME"},
				"QUAYIO_DOCKER_PASSWORD": {fromSecret: "QUAYIO_DOCKER_PASSWORD"},
			},
			Volumes: dockerVolumeRefs(),
			Commands: []string{
				`apk add --no-cache make`,
				`chown -R $UID:$GID /go`,
				`docker login -u="$$QUAYIO_DOCKER_USERNAME" -p="$$QUAYIO_DOCKER_PASSWORD" quay.io`,
				fmt.Sprintf(`make -C build.assets %s`, buildboxName),
				//fmt.Sprintf(`docker push quay.io/gravitational/teleport-%s:$RUNTIME`, buildboxName),
			},
		},
	}
	return p
}
