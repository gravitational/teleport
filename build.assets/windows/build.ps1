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
        $GoDownloadUrl = "https://go.dev/dl/go$GoVersion.windows-amd64.zip"
        $GoInstallZip = "go$GoVersion.windows-amd64.zip"
        Invoke-WebRequest -Uri $GoDownloadUrl -OutFile $GoInstallZip
        Expand-Archive -Path $GoInstallZip -DestinationPath $ToolchainDir
        Enable-Go -ToolchainDir $ToolchainDir -GoVersion $GoVersion
    }
}

function Enable-Go {
    <#
    .SYNOPSIS
        Adds the Go toolchaion to the system search path 
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir,
        [string] $GoVersion
    )
    begin {
        $Env:Path = "$Env:Path;$ToolchainDir/go/bin"
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
        $NodeZipfile = "node-$NodeVersion-win-x64.zip"
        Invoke-WebRequest -Uri https://nodejs.org/download/release/v$NodeVersion/node-v$NodeVersion-win-x64.zip -OutFile $NodeZipfile
        Expand-Archive -Path $NodeZipfile -DestinationPath $ToolchainDir
        Enable-Node -ToolchainDir $ToolchainDir -NodeVersion $NodeVersion
        npm config set msvs_version 2017
        corepack enable yarn
    }
}

function Enable-Node {
    <#
    .SYNOPSIS
        Adds the Node toolchaion to the system search path 
    #>
    [CmdletBinding()]
    param(
        [string] $ToolchainDir,
        [string] $NodeVersion
    )
    begin {
        $Env:Path = "$Env:Path;$ToolchainDir/node-v$NodeVersion-win-x64"
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

function Copy-Artifacts {
    <#
    .SYNOPSIS
        Copies all files in the supplied directory into an S3 bucket
    #>
    [CmdletBinding()]
    param(
        [string] $Path,
        [string] $Bucket,
        [string] $DstRoot 
    )
    begin {
        foreach ($file in $(Get-ChildItem $Path)) {
            Write-Output "Uploading $($file.Name)"
            $Key = "$DstRoot/$($file.Name)"
            Write-S3Object -File $file.FullName -Bucket $Bucket -Key $Key 
        }
    }
}