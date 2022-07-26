package main

func promoteBuildPipeline() pipeline {
	// TODO: migrate
	return pipeline{}
}

func updateDocsPipeline() pipeline {
	// TODO: migrate
	return pipeline{}
}

func verifyTaggedBuildStep() step {
	return step{
		Name:  "Verify build is tagged",
		Image: "alpine:latest",
		Commands: []string{
			"[ -n ${DRONE_TAG} ] || (echo 'DRONE_TAG is not set. Is the commit tagged?' && exit 1)",
		},
	}
}
