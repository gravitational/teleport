package main

const relcliImage = "146628656107.dkr.ecr.us-west-2.amazonaws.com/gravitational/relcli:v1.1.65"

func publishReleasePipeline() pipeline {
	p := newKubePipeline("publish-rlz")
	p.Environment = map[string]value{
		"RELCLI_IMAGE": {raw: relcliImage},
	}
	p.Trigger = trigger{
		Event:  triggerRef{Include: []string{"promote"}},
		Target: triggerRef{Include: []string{"production"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}
	p.Steps = []step{
		{
			Name:  "Check if commit is tagged",
			Image: "alpine",
			Commands: []string{
				`[ -n ${DRONE_TAG} ] || (echo 'DRONE_TAG is not set. Is the commit tagged?' && exit 1)`,
			},
		},
		pullRelcliStep(),
		executeRelcliStep("Publish in Release API", "relcli auto_publish -f -v 6"),
	}
	p.Services = []service{
		dockerService(volumeRef{
			Name: "tmpfs",
			Path: "/tmpfs",
		}),
	}
	p.Volumes = dockerVolumes(volume{
		Name: "tmpfs",
		Temp: &volumeTemp{Medium: "memory"},
	})

	return p
}

func pullRelcliStep() step {
	return step{
		Name:  "Pull relcli",
		Image: "docker:git",
		Environment: map[string]value{
			"AWS_ACCESS_KEY_ID":     {fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_KEY"},
			"AWS_SECRET_ACCESS_KEY": {fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_SECRET"},
			"AWS_DEFAULT_REGION":    {raw: "us-west-2"},
		},
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			`apk add --no-cache aws-cli`,
			`aws ecr get-login-password | docker login -u="AWS" --password-stdin 146628656107.dkr.ecr.us-west-2.amazonaws.com`,
			`docker pull $RELCLI_IMAGE`,
		},
	}
}

func executeRelcliStep(name string, command string) step {
	return step{
		Name:  name,
		Image: "docker:git",
		Environment: map[string]value{
			"RELCLI_BASE_URL": {raw: releasesHost},
			"RELEASES_CERT":   {fromSecret: "RELEASES_CERT_STAGING"},
			"RELEASES_KEY":    {fromSecret: "RELEASES_KEY_STAGING"},
			"RELCLI_CERT":     {raw: "/tmpfs/creds/releases.crt"},
			"RELCLI_KEY":      {raw: "/tmpfs/creds/releases.key"},
		},
		Volumes: dockerVolumeRefs(volumeRef{
			Name: "tmpfs",
			Path: "/tmpfs",
		}),
		Commands: []string{
			`mkdir -p /tmpfs/creds`,
			`echo "$RELEASES_CERT" | base64 -d > "$RELCLI_CERT"`,
			`echo "$RELEASES_KEY" | base64 -d > "$RELCLI_KEY"`,
			`trap "rm -rf /tmpfs/creds" EXIT`,
			`docker run -i -v /tmpfs/creds:/tmpfs/creds \
  -e DRONE_REPO -e DRONE_TAG -e RELCLI_BASE_URL -e RELCLI_CERT -e RELCLI_KEY \
  $RELCLI_IMAGE ` + command,
		},
		Failure: "ignore",
	}
}
