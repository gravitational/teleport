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

import (
	"path"
)

const (
	perBuildWorkspace = `$Env:WORKSPACE_DIR/$Env:DRONE_BUILD_NUMBER`
	toolchainDir      = `/toolchains`
	teleportSrc       = `/go/src/github.com/gravitational/teleport`

	relcliURL    = `https://cdn.teleport.dev/relcli-master-93a9f40-20230504T2005101-windows.exe`
	relcliSha256 = `22d32a57a4b999e619162bebb96d0adf4b3df2596ef4c89b77154e7f96abbf30`
)

func newWindowsPipeline(name string) pipeline {
	p := newExecPipeline(name)
	p.Workspace.Path = path.Join("C:/Drone/Workspace", name)
	p.Platform = platform{OS: "windows", Arch: "amd64"}
	p.Node = map[string]value{
		"buildbox_version": buildboxVersion,
	}
	return p
}

func windowsTagPipeline() pipeline {
	p := newWindowsPipeline("build-native-windows-amd64")
	p.Concurrency.Limit = 1
	p.DependsOn = []string{tagCleanupPipelineName}
	p.Trigger = triggerTag

	p.Steps = []step{
		cloneWindowsRepositoriesStep(p.Workspace.Path),
		updateWindowsSubreposStep(p.Workspace.Path),
		installWindowsNodeToolchainStep(p.Workspace.Path),
		installWindowsGoToolchainStep(p.Workspace.Path),
		buildWindowsAuthenticationPackageStep(p.Workspace.Path),
		buildWindowsTshStep(p.Workspace.Path),
		signTshStep(p.Workspace.Path),
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		{
			Name: "Assume AWS Role",
			Environment: map[string]value{
				"WORKSPACE_DIR":         {raw: p.Workspace.Path},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
				"AWS_ROLE":              {fromSecret: "AWS_ROLE"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
				`$AwsSharedCredentialsFile = "$Workspace/credentials"`,
				`$SessionName = "drone-$Env:DRONE_REPO-$Env:DRONE_BUILD_NUMBER".replace("/", "-")`,
				`. "$TeleportSrc/build.assets/windows/build.ps1"`,
				`Get-STSCallerIdentity`,
				`Save-Role -RoleArn $Env:AWS_ROLE -RoleSessionName $SessionName -FilePath $AwsSharedCredentialsFile`,
				`Get-ChildItem -Path Env: | Where-Object {($_.Name -Like "AWS_SECRET_ACCESS_KEY") -or ($_.Name -Like "AWS_ACCESS_KEY_ID") } | Remove-Item`,
				`Get-STSCallerIdentity -ProfileLocation $AwsSharedCredentialsFile`,
			},
		},
		{
			Name: "Upload Artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
				"AWS_REGION":    {raw: "us-west-2"},
				"AWS_S3_BUCKET": {fromSecret: "AWS_S3_BUCKET"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
				`$TeleportVersion=$Env:DRONE_TAG.TrimStart('v')`,
				`$AwsSharedCredentialsFile = "$Workspace/credentials"`,
				`$OutputsDir="$Workspace/outputs"`,
				`New-Item -Path "$OutputsDir" -ItemType 'Directory' | Out-Null`,
				`Get-ChildItem "$TeleportSrc/web/packages/teleterm/build/release`,
				`Copy-Item -Path "$TeleportSrc/web/packages/teleterm/build/release/Teleport Connect Setup*.exe" -Destination $OutputsDir`,
				`Copy-Item -Path "$TeleportSrc/e/windowsauth/build/teleport-windows-auth-setup-*.exe" -Destination $OutputsDir`,
				`. "$TeleportSrc/build.assets/windows/build.ps1"`,
				`Format-FileHashes -PathGlob "$OutputsDir/*.exe"`,
				`Copy-Artifacts -ProfileLocation $AwsSharedCredentialsFile -Path $OutputsDir -Bucket $Env:AWS_S3_BUCKET -DstRoot "/teleport/tag/$TeleportVersion"`,
			},
		},
		windowsRegisterArtifactsStep(p.Workspace.Path),
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
		installWindowsGoToolchainStep(p.Workspace.Path),
		buildWindowsTshStep(p.Workspace.Path),
		signTshStep(p.Workspace.Path),
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		buildWindowsAuthenticationPackageStep(p.Workspace.Path),
		cleanUpWindowsWorkspaceStep(p.Workspace.Path),
		{
			Name: "Send Slack notification (exec)",
			Environment: map[string]value{
				"WORKSPACE_DIR":              {raw: p.Workspace.Path},
				"SLACK_WEBHOOK_DEV_TELEPORT": {fromSecret: "SLACK_WEBHOOK_DEV_TELEPORT"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
				`. "$TeleportSrc/build.assets/windows/build.ps1"`,
				`Send-ErrorMessage`,
			},
			When: &condition{Status: []string{"failure"}},
		},
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
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`$TeleportRev = if ($Env:DRONE_TAG -ne $null) { $Env:DRONE_TAG } else { $Env:DRONE_COMMIT }`,
			`New-Item -Path $TeleportSrc -ItemType Directory | Out-Null`,
			`cd $TeleportSrc`,
			`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
			`git checkout $TeleportRev`,
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
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Git -Workspace $Workspace -PrivateKey $Env:GITHUB_PRIVATE_KEY`,
			`cd $TeleportSrc`,
			`git submodule update --init e`,
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
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Push-Location "$TeleportSrc/build.assets"`,
			`$NodeVersion = $(make print-node-version).Trim()`,
			`Pop-Location`,
			`Install-Node -NodeVersion $NodeVersion -ToolchainDir "$Workspace` + toolchainDir + `"`,
		},
	}
}

func installWindowsGoToolchainStep(workspacePath string) step {
	return step{
		Name:        "Install Go Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		Commands: []string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Push-Location "$TeleportSrc/build.assets"`,
			`$GoVersion = $(make print-go-version).TrimStart("go")`,
			`Pop-Location`,
			`Install-Go -GoVersion $GoVersion -ToolchainDir "$Workspace` + toolchainDir + `"`,
		},
	}
}

func buildWindowsTshStep(workspace string) step {
	return step{
		Name: "Build tsh",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$Env:GOCACHE = "$Workspace/gocache"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Go -ToolchainDir "$Workspace` + toolchainDir + `"`,
			`cd $TeleportSrc`,
			`$Env:GCO_ENABLED=1`,
			`go build -o build/tsh-unsigned.exe ./tool/tsh`,
		},
	}
}

func signTshStep(workspace string) step {
	return step{
		Name: "Sign tsh",
		Environment: map[string]value{
			"WORKSPACE_DIR":        {raw: workspace},
			"WINDOWS_SIGNING_CERT": {fromSecret: "WINDOWS_SIGNING_CERT"},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`cd $TeleportSrc`,
			`([System.Convert]::FromBase64String($ENV:WINDOWS_SIGNING_CERT)) | Set-Content windows-signing-cert.pfx -Encoding Byte`,
			`& 'C:\Program Files (x86)\Windows Kits\10\App Certification Kit\signtool.exe' sign /f windows-signing-cert.pfx /d Teleport /t http://timestamp.digicert.com /du https://goteleport.com /fd sha256 build\tsh-unsigned.exe`,
			`mv build\tsh-unsigned.exe build\tsh.exe`,
			`rm -r windows-signing-cert.pfx`,
		},
	}
}

func buildWindowsTeleportConnectStep(workspace string) step {
	return step{
		Name: "Build Teleport Connect",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
			"CSC_LINK":      {fromSecret: "WINDOWS_SIGNING_CERT"},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Node -ToolchainDir "$Workspace` + toolchainDir + `"`,
			`Push-Location $TeleportSrc`,
			`$TeleportVersion=$(make print-version).Trim()`,
			`$Env:CONNECT_TSH_BIN_PATH="$TeleportSrc\build\tsh.exe"`,
			`yarn install --frozen-lockfile`,
			`yarn build-term`,
			`yarn package-term "-c.extraMetadata.version=$TeleportVersion"`,
		},
	}
}

func buildWindowsAuthenticationPackageStep(workspace string) step {
	return step{
		Name: "Build Windows Authentication Package",
		Environment: map[string]value{
			"WORKSPACE_DIR":        {raw: workspace},
			"WINDOWS_SIGNING_CERT": {fromSecret: "WINDOWS_SIGNING_CERT"},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$Env:GOCACHE = "$Workspace/gocache"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Go -ToolchainDir "$Workspace` + toolchainDir + `"`,
			`cd $TeleportSrc`,
			`$TeleportVersion=$(make print-version).Trim()`,
			`cd "$TeleportSrc\e\windowsauth"`,
			`make VERSION=v$TeleportVersion  all`,
			`([System.Convert]::FromBase64String($ENV:WINDOWS_SIGNING_CERT)) | Set-Content windows-signing-cert.pfx -Encoding Byte`,
			`& 'C:\Program Files (x86)\Windows Kits\10\App Certification Kit\signtool.exe' sign /f windows-signing-cert.pfx /d Teleport /t http://timestamp.digicert.com /du https://goteleport.com /fd sha256 build/teleport-windows-auth-setup-v$TeleportVersion-amd64.exe`,
			`rm -r windows-signing-cert.pfx`,
		},
	}
}

func windowsRegisterArtifactsStep(workspace string) step {
	return step{
		Name: "Register artifacts",
		Environment: map[string]value{
			"WORKSPACE_DIR":   {raw: workspace},
			"RELEASES_CERT":   {fromSecret: "RELEASES_CERT"},
			"RELEASES_KEY":    {fromSecret: "RELEASES_KEY"},
			"RELCLI_BASE_URL": {raw: releasesHost},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$ProgressPreference = 'SilentlyContinue'`,
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "$Workspace` + teleportSrc + `"`,
			`$OutputsDir = "$Workspace/outputs"`,
			`$relcliUrl = '` + relcliURL + `'`,
			`$relcliSha256 = '` + relcliSha256 + `'`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Get-Relcli -Url $relcliUrl -Sha256 $relcliSha256 -Workspace $Workspace`,
			`Register-Artifacts -Workspace $Workspace -Outputs $OutputsDir`,
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
