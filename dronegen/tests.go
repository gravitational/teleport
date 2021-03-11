package main

import "fmt"

func testPipelines() []pipeline {
	return []pipeline{
		testCodePipeline(),
		testDocsPipeline(false),
		testDocsPipeline(true),
	}
}

func testCodePipeline() pipeline {
	p := newKubePipeline("test")
	p.Environment = map[string]value{
		"RUNTIME": value{raw: "go1.15.5"},
		"UID":     value{raw: "1000"},
		"GID":     value{raw: "1000"},
	}
	p.Trigger = triggerPullRequest
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{
		volumeDocker,
		volumeTmpfs,
		{Name: "tmp-dind", Temp: &volumeTemp{}},
		{Name: "tmp-integration", Temp: &volumeTemp{}},
	}
	p.Services = []service{
		{
			Name:  "Start Docker",
			Image: "docker:dind",
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
				{Name: "tmp-dind", Path: "/tmp"},
			},
		},
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
			Commands: []string{
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
				// this is allowed to fail because pre-4.3 Teleport versions
				// don't use the webassets submodule.
				`git submodule update --init webassets || true`,
				// use the Github API to check whether this PR comes from a
				// forked repo or not.
				// if it does, don't check out the Enterprise code.
				`if [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
  apk add --no-cache curl jq
  export PR_REPO=$(curl -Ls https://api.github.com/repos/gravitational/${DRONE_REPO_NAME}/pulls/${DRONE_PULL_REQUEST} | jq -r '.head.repo.full_name')
  echo "---> Source repo for PR ${DRONE_PULL_REQUEST}: $${PR_REPO}"
  # if the source repo for the PR matches DRONE_REPO, then        this is not a PR raised from a fork
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
			},
		},
		{
			Name:  "Build buildbox",
			Image: "docker",
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
			},
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
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
			},
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
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
			},
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
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
				{Name: "tmp-integration", Path: "/tmp"},
			},
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
			Volumes: []volumeRef{
				volumeRefDocker,
				volumeRefTmpfs,
				{Name: "tmp-integration", Path: "/tmp"},
			},
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

func testDocsPipeline(external bool) pipeline {
	label := "internal"
	milvFlag := "-ignore-external"
	if external {
		label = "external"
		milvFlag = "-ignore-internal"
	}
	p := newKubePipeline(fmt.Sprintf("test-docs-%s", label))
	p.Trigger = triggerPullRequest
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{
		volumeTmpfs,
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "golang:1.15.5",
			Volumes: []volumeRef{
				volumeRefTmpfs,
			},
			Commands: []string{
				`mkdir -p /tmpfs/go/src/github.com/gravitational/teleport`,
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
			},
		},
		{
			Name:  fmt.Sprintf("Run docs tests (%s links only)", label),
			Image: "golang:1.15.5",
			Volumes: []volumeRef{
				volumeRefTmpfs,
			},
			Commands: []string{
				`cd /tmpfs/go/src/github.com/gravitational/teleport`,
				`git diff --raw ${DRONE_COMMIT}..origin/${DRONE_COMMIT_BRANCH:-master} | awk '{print $6}' | grep -E '^docs' | grep -v ^$ | cut -d/ -f2 | sort | uniq > /tmp/docs-versions-changed.txt`,
				fmt.Sprintf(`if [ $(stat --printf="%%s" /tmp/docs-versions-changed.txt) -gt 0 ]; then
  echo "---> Changes to docs detected, versions $(cat /tmp/docs-versions-changed.txt | tr '\n' ' ')"
  # Check trailing whitespace
  make docs-test-whitespace
  # Check links
  for VERSION in $(cat /tmp/docs-versions-changed.txt); do
    if [ -f docs/$VERSION/milv.config.yaml ]; then
      go get github.com/magicmatatjahu/milv
      cd docs/$VERSION
      echo "---> Running milv on docs/$VERSION:"
      milv %s
      echo "------------------------------\n"
      cd -
    else
      echo "---> No milv config found, skipping docs/$VERSION"
    fi
  done
  else echo "---> No changes to docs detected, not running tests"
fi
`, milvFlag),
			},
		},
	}
	return p
}
