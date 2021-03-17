package main

var (
	triggerPullRequest = trigger{
		Event: triggerRef{Include: []string{"pull_request"}},
		Repo:  triggerRef{Include: []string{"gravitational/*"}},
	}
	triggerPush = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}
	triggerTag = trigger{
		Event: triggerRef{Include: []string{"tag"}},
		Ref:   triggerRef{Include: []string{"refs/tags/v*"}},
		Repo:  triggerRef{Include: []string{"gravitational/*"}},
	}

	volumeRefTmpfs = volumeRef{
		Name: "tmpfs",
		Path: "/tmpfs",
	}

	// TODO(gus): Set this from `make -C build.assets print-runtime-version` or similar rather
	// than hardcoding it. Also remove the usage of RUNTIME as a pipeline-level environment variable
	// (as support for these varies among Drone runners) and only set it for steps that need it.
	goRuntime = value{raw: "go1.15.5"}
)

type buildType struct {
	os      string
	arch    string
	fips    bool
	centos6 bool
}

type extraVolumes struct {
	tmpfs          bool
	tmpDind        bool
	tmpIntegration bool
}

// dockerService generates a docker:dind service and includes any extra volumes configured
// in extraVolumes.
func dockerService(v extraVolumes) service {
	return service{
		Name:    "Start Docker",
		Image:   "docker:dind",
		Volumes: volumeRefs(v),
	}
}

// volumes generates a slice of volumes including the Docker socket by default,
// plus any extra volumes configured in extraVolumes.
func volumes(v extraVolumes) []volume {
	volumes := []volume{
		volume{
			Name: "dockersock",
			Temp: &volumeTemp{},
		},
	}
	if v.tmpfs {
		volumes = append(volumes, volume{
			Name: "tmpfs",
			Temp: &volumeTemp{Medium: "memory"},
		})
	}
	if v.tmpDind {
		volumes = append(volumes, volume{
			Name: "tmp-dind",
			Temp: &volumeTemp{},
		})
	}
	if v.tmpIntegration {
		volumes = append(volumes, volume{
			Name: "tmp-integration",
			Temp: &volumeTemp{},
		})
	}
	return volumes
}

// volumeRefs generates a slice of volumeRefs including the Docker socket by default,
// plus any extra volumes configured in extraVolumes.
func volumeRefs(v extraVolumes) []volumeRef {
	volumeRefs := []volumeRef{
		volumeRef{
			Name: "dockersock",
			Path: "/var/run",
		},
	}
	if v.tmpfs {
		volumeRefs = append(volumeRefs, volumeRef{
			Name: "tmpfs",
			Path: "/tmpfs",
		})
	}
	if v.tmpDind {
		volumeRefs = append(volumeRefs, volumeRef{
			Name: "tmp-dind",
			Path: "/tmp",
		})
	}
	if v.tmpIntegration {
		volumeRefs = append(volumeRefs, volumeRef{
			Name: "tmp-integration",
			Path: "/tmp",
		})
	}
	return volumeRefs
}
