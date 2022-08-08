// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "path"

const (
	perBuildWorkspace   = `$Env:WORKSPACE_DIR/$Env:DRONE_BUILD_NUMBER`
	windowsToolchainDir = perBuildWorkspace + `/toolchains`
	perBuildTeleportSrc = perBuildWorkspace + "/go/src/github.com/gravitational/teleport"
	perBuildWebappsSrc  = perBuildWorkspace + "/go/src/github.com/gravitational/webapps"

	// Hardcoded tool versions that we would normally pull from the makefile,
	// but unfortunately our Makefiles use too many POSIX-isms for us to use on
	// Windows, so for now we will just say the versions we want here.

	windowsNodeVersion = "16.13.2"
)

func newWindowsPipeline(name string) pipeline {
	p := newExecPipeline(name)
	p.Workspace.Path = path.Join("C:/Drone/Workspace", name)
	p.Concurrency.Limit = 1
	p.Platform = platform{OS: "windows", Arch: "amd64"}
	return p
}

func windowsTagPipeline() pipeline {
	p := newWindowsPipeline("build-native-windows-amd64")
	p.Trigger = triggerTag
	p.DependsOn = []string{"build-windows-amd64"}
	p.Steps = []step{
		cloneWindowsRepositoriesStep(p.Workspace.Path),
		updateWindowsSubreposStep(p.Workspace.Path),
		installWindowsNodeToolchainStep(p.Workspace.Path),
		{
			Name: "Fetch pre-built tsh",
			Environment: map[string]value{
				"WORKSPACE_DIR":         {raw: p.Workspace.Path},
				"AWS_REGION":            {raw: "us-west-2"},
				"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
				`$TeleportVersion=$Env:DRONE_TAG.TrimStart('v')`,
				`$InputsDir="$Workspace/inputs"`,
				`$DownloadDir="$Workspace/downloads"`,
				`$TshZip="$InputsDir/teleport.zip"`,
				`Read-S3Object -File $TshZip -Bucket $Env:AWS_S3_BUCKET -Key "/teleport/tag/$TeleportVersion/teleport-v$TeleportVersion-windows-amd64-bin.zip" | Out-Null`,
				`Expand-Archive -Path $TshZip -DestinationPath $InputsDir`,
				`New-Item -Path "$TeleportSrc/build" -ItemType 'Directory' | Out-Null`,
				`Copy-Item -Path "$InputsDir/teleport/tsh.exe" -Destination "$TeleportSrc/build/tsh.exe"`,
			},
		},
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		{
			Name: "Upload Artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR":         {raw: p.Workspace.Path},
				"AWS_REGION":            {raw: "us-west-2"},
				"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
				`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
				`$TeleportVersion=$Env:DRONE_TAG.TrimStart('v')`,
				`$OutputsDir="$Workspace/outputs"`,
				`New-Item -Path "$OutputsDir" -ItemType 'Directory' | Out-Null`,
				`Get-ChildItem "$WebappsSrc/packages/teleterm/build/release`,
				`Copy-Item -Path "$WebappsSrc/packages/teleterm/build/release/Teleport Connect Setup*.exe" -Destination $OutputsDir`,
				`. "$TeleportSrc/build.assets/windows/build.ps1"`,
				`Format-FileHashes -PathGlob "$OutputsDir/*.exe"`,
				`Copy-Artifacts -Path $OutputsDir -Bucket $Env:AWS_S3_BUCKET -DstRoot "/teleport/tag/$TeleportVersion"`,
			},
		},
		cleanUpWindowsWorkspaceStep(p.Workspace.Path),
	}
	return p
}

func windowsPushPipeline() pipeline {
	p := newWindowsPipeline("push-build-native-windows-amd64")
	p.Trigger = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}

	p.Steps = []step{
		cloneWindowsRepositoriesStep(p.Workspace.Path),
		updateWindowsSubreposStep(p.Workspace.Path),
		installWindowsNodeToolchainStep(p.Workspace.Path),
		{
			Name: "Create Phoney tsh",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
			},
			Commands: []string{
				`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
				`New-Item -Path "$TeleportSrc/build/tsh.exe" -Force -ItemType 'File'`,
			},
		},
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		cleanUpWindowsWorkspaceStep(p.Workspace.Path),
	}

	return p
}

func cloneWindowsRepositoriesStep(workspace string) step {
	return step{
		Name: "Check out Teleport",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Env:GOCACHE = "` + perBuildWorkspace + `/gocache"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`$TeleportRev = if ($Env:DRONE_TAG -ne $null) { $Env:DRONE_TAG } else { $Env:DRONE_COMMIT }`,
			`New-Item -Path $TeleportSrc -ItemType Directory | Out-Null`,
			`cd $TeleportSrc`,
			`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
			`git checkout $TeleportRev`,
			`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
			`New-Item -Path $WebappsSrc -ItemType Directory | Out-Null`,
			`cd $WebappsSrc`,
			`git clone https://github.com/gravitational/webapps.git .`,
			`git checkout $(go run $TeleportSrc/build.assets/tooling/cmd/get-webapps-version/main.go)`,
		},
	}
}

func updateWindowsSubreposStep(workspace string) step {
	return step{
		Name: "Checkout Submodules",
		Environment: map[string]value{
			"WORKSPACE_DIR":      {raw: workspace},
			"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
		},
		Commands: []string{
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Git -Workspace $Workspace -PrivateKey $Env:GITHUB_PRIVATE_KEY`,
			`cd $TeleportSrc`,
			`git submodule update --init e`,
			`git submodule update --init --recursive webassets`,
			`Reset-Git -Workspace $Workspace`,
		},
	}
}

func installWindowsNodeToolchainStep(workspacePath string) step {
	return step{
		Name:        "Install Node Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		Commands: []string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$ErrorActionPreference = 'Stop'`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			// We can't use make, as there are too many posix dependencies in our makefile
			// to abstract away right now, so instead of `$(make -C $TeleportSrc/build.assets print-node-version)`,
			// we will just hardcode it for now
			`$NodeVersion = "` + windowsNodeVersion + `"`,
			`Install-Node -NodeVersion $NodeVersion -ToolchainDir "` + windowsToolchainDir + `"`,
		},
	}
}

func buildWindowsTeleportConnectStep(workspace string) step {
	return step{
		Name: "Build Teleport Connect",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
		},
		Commands: []string{
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
			`$NodeVersion = "` + windowsNodeVersion + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Node -NodeVersion $NodeVersion -ToolchainDir "` + windowsToolchainDir + `"`,
			`cd $WebappsSrc`,
			`yarn install`,
			`yarn build-term`,
			`yarn package-term`,
		},
	}
}

func cleanUpWindowsWorkspaceStep(workspacePath string) step {
	return step{
		Name:        "Clean up workspace (post)",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		When: &condition{
			Status: []string{"success", "failure"},
		},
		Commands: []string{
			// We don't want to break the build based on just a failed cleanup,
			// so we just tell PowerShell to carry on as best it can in the
			// face of an error
			`$ErrorActionPreference = 'Continue'`,
			`Remove-Item -Recurse -Force -Path "$Env:WORKSPACE_DIR/$Env:DRONE_BUILD_NUMBER"`,
		},
	}
}
