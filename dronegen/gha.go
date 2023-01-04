package main

import "fmt"

type ghaBuildType struct {
	buildType
	trigger
	namePrefix      string
	uploadArtifacts bool
	srcRefVar       string
	workflowRefVar  string
	slackOnError    bool
}

func ghaBuildPipeline(b ghaBuildType) pipeline {
	p := newKubePipeline(fmt.Sprintf("%sbuild-%s-%s", b.namePrefix, b.os, b.arch))
	p.Trigger = b.trigger
	p.Workspace = workspace{Path: "/go"}
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"RUNTIME":          goRuntime,
		"UID":              {raw: "1000"},
		"GID":              {raw: "1000"},
	}

	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommands(b.buildType),
		},
		{
			Name:  "Delegate build to GitHub",
			Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
			Environment: map[string]value{
				"GHA_APP_KEY": {fromSecret: "GITHUB_WORKFLOW_APP_PRIVATE_KEY"},
			},
			Commands: []string{
				`cd "/go/src/github.com/gravitational/teleport/build.assets/tooling"`,
				`go run ./cmd/gh-trigger-workflow -owner ${DRONE_REPO_OWNER} -repo teleport.e -workflow release-linux-arm64.yml ` +
					fmt.Sprintf(`-workflow-ref=${%s} `, b.workflowRefVar) +
					fmt.Sprintf(`-input oss-teleport-ref=${%s} `, b.srcRefVar) +
					fmt.Sprintf(`-input upload-artifacts=%t`, b.uploadArtifacts),
			},
		},
	}

	if b.slackOnError {
		p.Steps = append(p.Steps, sendErrorToSlackStep())
	}

	return p
}
