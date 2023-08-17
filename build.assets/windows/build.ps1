# Copyright 2022 Gravitational, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# #############################################################################
#
# This file contains PowerShell snippets used in the Teleport and/or Teleport
# Connect builds on windows native builders. These snippets exist both as
# useful abstractions, and a way to avoid Drone attempting to echo back every
# command we execute. 
#
# Sometimes avoiding command echoing is important because:
#  1. The PowerShell `echo` is not a built-in command, but an alias for 
#     `Write-Output`
#  2. Drone's output escaping is not perfect, and so
#  3. Sometimes arguments to commands get interpreted as arguments to 
#     `echo` when incorrectly escaped and echoed back, which crashes
#     the build.
#
# Unfortunately there is currently no way to disable command echoing in the 
# Windows Drone executor, so we hide the problematic scripts behind the 
# cmdlets definmed in this file.
#
# #############################################################################
#
# Usage: Source this file into your active shell
#
#  PS> . build.assets/Windows/build.ps1
#
# #############################################################################

function Enable-Git {
    <#
    .SYNOPSIS
        Configures git for accessing (possibly private) repos, given a 
        private key
    #>
    [CmdletBinding()]
    param(
        [string] $Workspace,
        [string] $PrivateKey
    )
    begin {
        $SSHDir = "$Workspace/.ssh"
        New-Item -Path "$SSHDir" -ItemType Directory | Out-Null
        $PrivateKey | Out-File -Encoding ascii "$SSHDir/id_rsa"
        Invoke-WebRequest "https://api.github.com/meta" -UseBasicParsing `
            | ConvertFrom-JSON `
            | Select-Object -ExpandProperty "ssh_keys" `
            | ForEach-Object {"github.com $_"} `
            | Out-File -Encoding ASCII "$SSHDir/known_hosts"
        $SSHCmd = "ssh -i $SSHDir/id_rsa -o UserKnownHostsFile=$SSHDir/known_hosts -F/dev/null"
        $Env:GIT_SSH_COMMAND = $SSHCmd
    }
}

function Reset-Git {
[CmdletBinding()]
param(
    <#
    .SYNOPSIS
        Cleans up private git access as configured with Enable-Git.
    #>
    [string] $Workspace
)
    begin {
        Remove-Item -Recurse -Path "$Workspace/.ssh"
    }
}

function Install-Go {
    <#
    .SYNOPSIS
        Downloads ands installs Go into the supplied toolchain dir
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir,
        [string] $GoVersion
    )
    begin {
        New-Item -Path "$ToolchainDir" -ItemType Directory -Force | Out-Null
        $GoDownloadUrl = "https://go.dev/dl/go$GoVersion.windows-amd64.zip"
        $GoInstallZip = "$ToolchainDir/go$GoVersion.windows-amd64.zip"
        Invoke-WebRequest -Uri $GoDownloadUrl -OutFile $GoInstallZip
        Expand-Archive -Path $GoInstallZip -DestinationPath $ToolchainDir
        Enable-Go -ToolchainDir $ToolchainDir
    }
}

function Enable-Go {
    <#
    .SYNOPSIS
        Adds the Go toolchaion to the system search path 
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir
    )
    begin {
        # note we prepend the toolchain before the path, otherwise the build 
        # will just use the system Go.
        $Env:Path = "$ToolchainDir/go/bin;$Env:Path"
    }
}

function Install-Node {
    <#
    .SYNOPSIS
        Downloads ands installs Node into the supplied toolchain dir
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir,
        [string] $NodeVersion
    )
    begin {
        New-Item -Path "$ToolchainDir" -ItemType Directory -Force | Out-Null
        $NodeZipfile = "$ToolchainDir/node-$NodeVersion-win-x64.zip"
        Invoke-WebRequest -Uri https://nodejs.org/download/release/v$NodeVersion/node-v$NodeVersion-win-x64.zip -OutFile $NodeZipfile
        Expand-Archive -Path $NodeZipfile -DestinationPath $ToolchainDir
        Rename-Item -Path "$ToolchainDir/node-v$NodeVersion-win-x64" -NewName "$ToolchainDir/node"
        Enable-Node -ToolchainDir $ToolchainDir
        npm config set msvs_version 2022
        corepack enable yarn
    }
}

function Enable-Node {
    <#
    .SYNOPSIS
        Adds the Node toolchain to the system search path
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir
    )
    begin {
        $Env:Path = "$ToolchainDir/node;$Env:Path"
    }
}


function Format-FileHashes {
    <#
    .SYNOPSIS
        Finds each file matching the supplied path glob and creates a sidecar 
        `*.sha256` file containing the file's hash
    #>
    [CmdletBinding()]
    param(
        [string] $PathGlob
    )
    begin {
        foreach ($file in $(Get-ChildItem $PathGlob)) {
            Write-Output "Hashing  $($file.Name)"
            $Hash = (Get-FileHash $file.FullName).Hash
            "$($Hash.ToLower()) $($file.Name)" `
                | Out-File -Encoding ASCII -FilePath "$($file.FullName).sha256"
        }
    }
}

function Save-Role {
    <#
    .SYNOPSIS
        Assume an AWS role and save the session to the supplied file
    #>
    [CmdletBinding()]
    param(
        [string] $RoleArn,
        [string] $RoleSessionName,
        [string] $FilePath
    )
    begin {
        $RoleCreds = (Use-STSRole -RoleArn $RoleArn -RoleSessionName $RoleSessionName).Credentials
        "[default]`r`naws_access_key_id = {0}`r`naws_secret_access_key = {1}`r`naws_session_token = {2}" -f $RoleCreds.AccessKeyId, $RoleCreds.SecretAccessKey, $RoleCreds.SessionToken | Out-File -FilePath $FilePath
    }
}

function Copy-Artifacts {
    <#
    .SYNOPSIS
        Copies all files in the supplied directory into an S3 bucket
    #>
    [CmdletBinding()]
    param(
        [string] $ProfileLocation,
        [string] $Path,
        [string] $Bucket,
        [string] $DstRoot
    )
    begin {
        foreach ($file in $(Get-ChildItem $Path)) {
            Write-Output "Uploading $($file.Name)"
            $Key = "$DstRoot/$($file.Name)"
            Write-S3Object -ProfileLocation $ProfileLocation -File $file.FullName -Bucket $Bucket -Key $Key
        }
    }
}

function Convert-Base64 {
    [CmdletBinding()]
    param(
        [string] $FilePath,
        [string] $Data
    )
    begin {
        $bytes = [Convert]::FromBase64String($Data)
        Set-Content -Encoding Byte -Path $FilePath -Value $bytes
    }
}

function Get-Relcli {
    <#
    .SYNOPSIS
        Downloads relcli
    #>
    [CmdletBinding()]
    param(
        [string] $Url,
        [string] $Sha256,
        [string] $Workspace
    )
    begin {
        Invoke-WebRequest $url -UseBasicParsing -OutFile "$Workspace\relcli.exe"
        $gotSha256 = (Get-FileHash "$Workspace\relcli.exe").hash
        if ($gotSha256 -ne $Sha256) {
            Write-Output "sha256 mismatch: $gotSha256 != $Sha256"
        }
    }
}

function Register-Artifacts {
    <#
    .SYNOPSIS
        Invokes relcli to automatically upload built artifacts
    #>
    [CmdletBinding()]
    param(
        [string] $Workspace,
        [string] $OutputsDir
    )
    begin {
        $certPath = "$Workspace/releases.crt"
        $keyPath = "$Workspace/releases.key"
        Convert-Base64 -Data $Env:RELEASES_CERT -FilePath $certPath
        Convert-Base64 -Data $Env:RELEASES_KEY -FilePath $keyPath
        & "$Workspace\relcli.exe" --cert $certPath --key $keyPath auto_upload -f -v 6 $OutputsDir
    }
}

function Send-ErrorMessage {
    <#
    .SYNOPSIS
    Formats and sends a build failure message to Slack
    #>
    [CmdletBinding()]
    param ()

    begin {
        $BuildUrl = "$Env:DRONE_SYSTEM_PROTO`://$Env:DRONE_SYSTEM_HOSTNAME/$Env:DRONE_REPO_OWNER/$Env:DRONE_REPO_NAME/$Env:DRONE_BUILD_NUMBER"
        $GoOS = $(go env GOOS)
		$GoArch = $(go env GOARCH)
        $Msg = @"
Warning: ``$GoOS-$GoArch`` artifact build failed for [``$Env:DRONE_REPO_NAME``] - please investigate immediately!
Branch: ``$Env:DRONE_BRANCH``
Commit: ``$Env:DRONE_COMMIT_SHA``
Link: $BuildUrl
"@
        Invoke-RestMethod -Method 'Post' -Uri $Env:SLACK_WEBHOOK_DEV_TELEPORT -Body $(@{"text"=$Msg} | ConvertTo-Json)
    }
}
