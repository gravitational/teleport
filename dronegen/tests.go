package main

func testPipelines() []pipeline {
	return []pipeline{
		testCodePipeline(),
		testDocsPipeline(),
	}
}

// testCheckoutCommands returns a set of commands for checking out Teleport's code
// Setting enterprise to true will also add a check against the Github API to determine whether
// the pull request comes from a code fork (and will only check out Enterprise code if it does not)
func testCheckoutCommands(enterprise bool) []string {
	commands := []string{
		`mkdir -p /tmpfs/go/src/github.com/gravitational/teleport /tmpfs/go/cache`,
		`cd /tmpfs/go/src/github.com/gravitational/teleport`,
		`git init && git remote add origin ${DRONE_REMOTE_URL}`,
		`# handle pull requests
if [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
  git fetch origin +refs/heads/${DRONE_COMMIT_BRANCH}:
  git checkout ${DRONE_COMMIT_BRANCH}
  git fetch origin ${DRONE_COMMIT_REF}:
  git merge ${DRONE_COMMIT}
# handle tags
elif [ "${DRONE_BUILD_EVENT}" = "tag" ]; then
  git fetch origin +refs/tags/${DRONE_TAG}:
  git checkout -qf FETCH_HEAD
# handle pushes/other events
else
  if [ "${DRONE_COMMIT_BRANCH}" = "" ]; then
    git fetch origin
    git checkout -qf ${DRONE_COMMIT_SHA}
  else
    git fetch origin +refs/heads/${DRONE_COMMIT_BRANCH}:
    git checkout ${DRONE_COMMIT} -b ${DRONE_COMMIT_BRANCH}
  fi
fi
`,
	}
	if enterprise {
		commands = append(commands,
			// this is allowed to fail because pre-4.3 Teleport versions
			// don't use the webassets submodule.
			`git submodule update --init webassets || true`,
			// use the Github API to check whether this PR comes from a forked repo or not.
			// if it does, don't check out the Enterprise code.
			`if [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
  apk add --no-cache curl jq
  export PR_REPO=$(curl -Ls https://api.github.com/repos/gravitational/${DRONE_REPO_NAME}/pulls/${DRONE_PULL_REQUEST} | jq -r '.head.repo.full_name')
  echo "---> Source repo for PR ${DRONE_PULL_REQUEST}: $${PR_REPO}"
  # if the source repo for the PR matches DRONE_REPO, then this is not a PR raised from a fork
  if [ "$${PR_REPO}" = "${DRONE_REPO}" ] || [ "${DRONE_REPO}" = "gravitational/teleport-private" ]; then
    mkdir -m 0700 /root/.ssh && echo -n "$GITHUB_PRIVATE_KEY" > /root/.ssh/id_rsa && chmod 600 /root/.ssh/id_rsa
    ssh-keyscan -H github.com > /root/.ssh/known_hosts 2>/dev/null && chmod 600 /root/.ssh/known_hosts
    git submodule update --init e
    # do a recursive submodule checkout to get both webassets and webassets/e
    # this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
    git submodule update --init --recursive webassets || true
    rm -f /root/.ssh/id_rsa
  fi
fi
`,
		)
	}
	return commands
}

// testCodePipeline returns a pipeline which runs the linter plus unit and integration tests
func testCodePipeline() pipeline {
	p := newKubePipeline("test")
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
		"UID":     value{raw: "1000"},
		"GID":     value{raw: "1000"},
	}
	p.Trigger = triggerPullRequest
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = dockerVolumes(
		volumeTmpfs,
		volumeTmpDind,
		volumeTmpIntegration,
		volumeDockerTmpfs,
	)
	p.Services = []service{
		dockerService(
			volumeRefTmpfs,
			volumeRefDockerTmpfs,
			volumeRefTmpDind,
		),
	}
	goEnvironment := map[string]value{
		"GOCACHE": value{raw: "/tmpfs/go/cache"},
		"GOPATH":  value{raw: "/tmpfs/go"},
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
			Commands: testCheckoutCommands(true),
		},
		{
			Name:    "Build buildbox",
			Image:   "docker",
			Volumes: dockerVolumeRefs(volumeRefTmpfs),
			Commands: []string{
				`apk add --no-cache make`,
				`chown -R $UID:$GID /tmpfs/go`,
				`docker pull quay.io/gravitational/teleport-buildbox:$RUNTIME || true`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`make -C build.assets buildbox`,
			},
		},
		{
			Name:        "Run linter",
			Image:       "docker",
			Environment: goEnvironment,
			Volumes:     dockerVolumeRefs(volumeRefTmpfs),
			Commands: []string{
				`apk add --no-cache make`,
				`chown -R $UID:$GID /tmpfs/go`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`make -C build.assets lint`,
			},
		},
		{
			// https://discourse.drone.io/t/how-to-exit-a-pipeline-early-without-failing/3951
			// this step looks at the output of git diff --raw to determine
			// whether any files which don't match the pattern '^docs/',
			// '.mdx$' or '.md$' were changed. if there are no changes to
			// non-docs code, we skip the Teleport tests and exit early with a
			// special Drone exit code to speed up iteration on docs (as milv
			// is much quicker to run)
			Name:  "Optionally skip tests",
			Image: "docker:git",
			Volumes: []volumeRef{
				volumeRefTmpfs,
			},
			Commands: []string{
				`cd /tmpfs/go/src/github.com/gravitational/teleport
echo -e "\n---> git diff --raw ${DRONE_COMMIT}..origin/${DRONE_COMMIT_BRANCH:-master}\n"
git diff --raw ${DRONE_COMMIT}..origin/${DRONE_COMMIT_BRANCH:-master}
git diff --raw ${DRONE_COMMIT}..origin/${DRONE_COMMIT_BRANCH:-master} | awk '{print $6}' | grep -Ev '^docs/' | grep -Ev '.mdx$' | grep -Ev '.md$' | grep -v ^$ | wc -l > /tmp/.change_count.txt
export CHANGE_COUNT=$(cat /tmp/.change_count.txt | tr -d '\n')
echo -e "\n---> Non-docs changes detected: $$CHANGE_COUNT"
if [ $$CHANGE_COUNT -gt 0 ]; then
  echo "---> Teleport tests will run normally"
else
  echo "---> Skipping Teleport tests and exiting early"
  exit 78
fi
echo ""
`,
			},
		},
		{
			Name:        "Run unit and chaos tests",
			Image:       "docker",
			Environment: goEnvironment,
			Volumes:     dockerVolumeRefs(volumeRefTmpfs),
			Commands: []string{
				`apk add --no-cache make`,
				`chown -R $UID:$GID /tmpfs/go`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`make -C build.assets test`,
			},
		},
		{
			// some integration tests can only be run as root, so we handle
			// these separately using a build tag
			Name:        "Run root-only integration tests",
			Image:       "docker",
			Environment: goEnvironment,
			Volumes:     dockerVolumeRefs(volumeRefTmpfs, volumeRefTmpIntegration),
			Commands: []string{
				`apk add --no-cache make`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`make -C build.assets integration-root`,
			},
		},
		{
			Name:  "Run integration tests",
			Image: "docker",
			Environment: map[string]value{
				"GOCACHE":                   value{raw: "/tmpfs/go/cache"},
				"GOPATH":                    value{raw: "/tmpfs/go"},
				"INTEGRATION_CI_KUBECONFIG": value{fromSecret: "INTEGRATION_CI_KUBECONFIG"},
				"KUBECONFIG":                value{raw: "/tmpfs/go/kubeconfig.ci"},
				"TEST_KUBE":                 value{raw: "true"},
			},
			Volumes: dockerVolumeRefs(volumeRefTmpfs, volumeRefTmpIntegration),
			Commands: []string{
				`apk add --no-cache make`,
				// write kubeconfig to disk for use in kube integration tests
				`echo "$INTEGRATION_CI_KUBECONFIG" > "$KUBECONFIG"`,
				`chown -R $UID:$GID /tmpfs/go`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`make -C build.assets integration`,
				`rm -f "$KUBECONFIG"`,
			},
		},
	}
	return p
}

func testDocsPipeline() pipeline {
	p := newKubePipeline("test-docs")
	p.Trigger = triggerPullRequest
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = dockerVolumes(
		volumeTmpfs,
		volumeDockerTmpfs,
	)
	p.Services = []service{
		dockerService(
			volumeRefTmpfs,
			volumeRefDockerTmpfs,
		),
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Volumes: []volumeRef{
				volumeRefTmpfs,
			},
			Commands: testCheckoutCommands(false),
		},
		{
			Name:  "Run docs tests",
			Image: "docker:git",
			Environment: map[string]value{
				"GOCACHE": value{raw: "/tmpfs/go/cache"},
				"UID":     value{raw: "1000"},
				"GID":     value{raw: "1000"},
			},
			Volumes: dockerVolumeRefs(volumeRefTmpfs),
			Commands: []string{
				`apk add --no-cache make`,
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`chown -R $UID:$GID /tmpfs/go`,
				`git diff --raw ${DRONE_COMMIT}..origin/${DRONE_COMMIT_BRANCH:-master} | awk '{print $6}' | grep -E '^docs' | { grep -v ^$ || true; } > /tmp/docs-changes.txt`,
				`if [ $(cat /tmp/docs-changes.txt | wc -l) -gt 0 ]; then
  echo "---> Changes to docs detected"
  cat /tmp/docs-changes.txt
  echo "---> Checking for trailing whitespace"
  make docs-test-whitespace
  echo "---> Checking for dead links"
  make -C build.assets test-docs
else
  echo "---> No changes to docs detected, not running tests"
fi
`,
			},
		},
	}
	return p
}
