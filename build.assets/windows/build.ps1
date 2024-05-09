# Teleport
# Copyright (C) 2023  Gravitational, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

# #############################################################################
#
# This file contains PowerShell snippets used in the Teleport and/or Teleport
# Connect builds on Windows native builders. These snippets exist both as
# useful abstractions.
#
# Usage: Source this file into your active shell
#
#  PS> . build.assets/Windows/build.ps1
#
# #############################################################################

function New-TempDirectory {
    <#
    .SYNOPSIS
    Creates a uniquely-named temporary directory.
    
    .OUTPUTS
    string
    #>

    $TempDirectoryPath = Join-Path -Path "$([System.IO.Path]::GetTempPath())" -ChildPath "$($(New-Guid).Guid)"
    New-Item -ItemType Directory -Path "$TempDirectoryPath" | Out-Null

    return "$TempDirectoryPath"
}

function Install-Go {
    <#
    .SYNOPSIS
        Downloads ands installs Go into the supplied toolchain dir
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $ToolchainDir,
        [Parameter(Mandatory)]
        [string] $GoVersion
    )
    begin {
        Write-Host "::group::Installing Go $GoVersion to $ToolchainDir..."
        New-Item -Path "$ToolchainDir" -ItemType Directory -Force | Out-Null
        $GoDownloadUrl = "https://go.dev/dl/go$GoVersion.windows-amd64.zip"
        $GoInstallZip = "$ToolchainDir/go$GoVersion.windows-amd64.zip"
        Invoke-WebRequest -Uri $GoDownloadUrl -OutFile $GoInstallZip
        Expand-Archive -Path $GoInstallZip -DestinationPath $ToolchainDir
        Enable-Go -ToolchainDir $ToolchainDir
        Write-Host "::endgroup::"
    }
}

function Enable-Go {
    <#
    .SYNOPSIS
        Adds the Go toolchaion to the system search path 
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $ToolchainDir
    )
    begin {
        # note we prepend the toolchain before the path, otherwise the build 
        # will just use the system Go.
        $Env:Path = "$ToolchainDir/go/bin;$Env:Path"
    }
}

function Install-Rust {
    <#
    .SYNOPSIS
        Downloads and installs Rust into the supplied toolchain dir
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $ToolchainDir,
        [Parameter(Mandatory)]
        [string] $RustVersion
    )
    begin {
        Write-Host "::group::Installing Rust $RustVersion to $ToolchainDir..."
        New-Item -Path "$ToolchainDir" -ItemType Directory -Force | Out-Null
        $RustupFile = "$ToolchainDir/rustup-init.exe"
        Invoke-WebRequest -Uri https://static.rust-lang.org/rustup/dist/x86_64-pc-windows-gnu/rustup-init.exe -OutFile $RustupFile
        $Env:RUSTUP_HOME = "$ToolchainDir/rustup"
        $Env:CARGO_HOME = "$ToolchainDir/cargo"
        & "$ToolchainDir\rustup-init.exe" --profile minimal -y --default-toolchain "$RustVersion-x86_64-pc-windows-gnu"
        Enable-Rust -ToolchainDir $ToolchainDir
        Write-Host "::endgroup::"
    }
}

function Enable-Rust {
    <#
    .SYNOPSIS
        Adds the Rust toolchain to the system search path
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $ToolchainDir
    )
    begin {
        $Env:RUSTUP_HOME = "$ToolchainDir/rustup"
        $Env:CARGO_HOME = "$ToolchainDir/cargo"
        $Env:Path = "$ToolchainDir/cargo/bin;$Env:Path"
    }
}

function Install-Node {
    <#
    .SYNOPSIS
        Downloads ands installs Node into the supplied toolchain dir
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $ToolchainDir,
        [Parameter(Mandatory)]
        [string] $NodeVersion
    )
    begin {
        Write-Host "::group::Installing Node $NodeVersion to $ToolchainDir..."
        New-Item -Path "$ToolchainDir" -ItemType Directory -Force | Out-Null
        $NodeZipfile = "$ToolchainDir/node-$NodeVersion-win-x64.zip"
        Invoke-WebRequest -Uri https://nodejs.org/download/release/v$NodeVersion/node-v$NodeVersion-win-x64.zip -OutFile $NodeZipfile
        Expand-Archive -Path $NodeZipfile -DestinationPath $ToolchainDir
        Rename-Item -Path "$ToolchainDir/node-v$NodeVersion-win-x64" -NewName "$ToolchainDir/node"
        Enable-Node -ToolchainDir $ToolchainDir
        corepack enable yarn
        Write-Host "::endgroup::"
    }
}

function Enable-Node {
    <#
    .SYNOPSIS
        Adds the Node toolchain to the system search path
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
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
        [Parameter(Mandatory)]
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

function Get-Relcli {
    <#
    .SYNOPSIS
        Downloads relcli
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $Url,
        [Parameter(Mandatory)]
        [string] $Sha256,
        [Parameter(Mandatory)]
        [string] $Workspace
    )
    begin {
        New-Item -Path "$Workspace" -ItemType Directory -Force | Out-Null
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
        [Parameter(Mandatory)]
        [string] $Workspace,
        [Parameter(Mandatory)]
        [string] $OutputsDir,
        [string] $ReleaseRepo = $env:RELEASE_REPO,
        [string] $ArtifactVersion = $env:ARTIFACT_VERSION
    )
    begin {
        $certPath = "$Workspace\releases.crt"
        Out-File -FilePath $certPath -Encoding ascii -InputObject "$env:RELEASES_CERT"
        $keyPath = "$Workspace\releases.key"
        Out-File -FilePath $keyPath -Encoding ascii -InputObject "$env:RELEASES_KEY"

        # These must be set for the `auto_upload` command
        $env:DRONE_REPO = "$ReleaseRepo"
        $env:DRONE_TAG = "$ArtifactVersion"

        & "$Workspace\relcli.exe" --cert $certPath --key $keyPath auto_upload -f -v 6 $OutputsDir
    }
}

function Measure-Block {
    <#
    .SYNOPSIS
    Measure the runtime of a provided block while streaming it's output to Out-Default.
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory, Position = 0)]
        [scriptblock]
        $Expression
    )

    return Measure-Command -Expression {
        & $Expression | Out-Default
    }
}

function Install-BuildRequirements {
    <#
    .SYNOPSIS
    Installs the tools required to produce a Windows-native Teleport build
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $InstallDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory
    )

    Write-Host "Installing build requirements..."

    $CommandDuration = Measure-Block {
        New-Item -Path "$InstallDirectory" -ItemType Directory -Force | Out-Null

        $RustVersion = $(make --no-print-directory -C "$TeleportSourceDirectory/build.assets" print-rust-version).Trim()
        Install-Rust -RustVersion "$RustVersion" -ToolchainDir "$InstallDirectory"

        $NodeVersion = $(make --no-print-directory -C "$TeleportSourceDirectory/build.assets" print-node-version).Trim()
        Install-Node -NodeVersion "$NodeVersion" -ToolchainDir "$InstallDirectory"

        $GoVersion = $(make --no-print-directory -C "$TeleportSourceDirectory/build.assets" print-go-version).TrimStart("go")
        Install-Go -GoVersion "$GoVersion" -ToolchainDir "$InstallDirectory"
    }
    Write-Host $("All build requirements installed in {0:g}" -f $CommandDuration)
}

function Invoke-SignBinary {
    <#
    .SYNOPSIS
    Signs the provided binary with the base64-encoded certificate listed in "$WINDOWS_SIGNING_CERT"
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $UnsignedBinaryPath,

        [Parameter(Mandatory)]
        [string] $SignedBinaryPath
    )

    Write-Host "Signing $UnsignedBinaryPath using WSL sign-binary script:"
    wsl-ubuntu-command sign-binary "$UnsignedBinaryPath" "$SignedBinaryPath"
}

function Build-WindowsAuthenticationPackage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion
    )

    $CommandDuration = Measure-Block {
        # Build Windows authentication package
        Write-Host "::group::Building Windows auth setup..."
        $WindowsAuthDirectory = "$TeleportSourceDirectory\e\windowsauth"
        make -C "$WindowsAuthDirectory" VERSION="v$TeleportVersion" all
        Write-Host "::endgroup::"
        Write-Host "::group::Signing Windows auth setup..."
        $BinaryName = "teleport-windows-auth-setup-v$TeleportVersion-amd64.exe"
        Invoke-SignBinary -UnsignedBinaryPath "$WindowsAuthDirectory\build\$BinaryName" -SignedBinaryPath "$ArtifactDirectory\$BinaryName"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built Windows authentication package in {0:g}" -f $CommandDuration)
}

function Build-Tsh {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion
    )

    $BinaryName = "tsh.exe"
    $BuildDirectory = "$TeleportSourceDirectory\build"
    $SignedBinaryPath = "$BuildDirectory\$BinaryName"

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building tsh..."
        $UnsignedBinaryPath = "$BuildDirectory\unsigned-$BinaryName"
        go build -tags piv -o "$UnsignedBinaryPath" "$TeleportSourceDirectory\tool\tsh"
        Write-Host "::endgroup::"

        Write-Host "::group::Signing tsh..."
        Invoke-SignBinary -UnsignedBinaryPath "$UnsignedBinaryPath" -SignedBinaryPath "$SignedBinaryPath"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built TSH in {0:g}" -f $CommandDuration)

    return "$SignedBinaryPath"  # This is needed for building Connect and bundling the zip archive
}

function Build-Tctl {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion
    )

    $BinaryName = "tctl.exe"
    $BuildDirectory = "$TeleportSourceDirectory\build"
    $SignedBinaryPath = "$BuildDirectory\$BinaryName"

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building tctl..."
        $UnsignedBinaryPath = "$BuildDirectory\unsigned-$BinaryName"
        go build -tags piv -o "$UnsignedBinaryPath" "$TeleportSourceDirectory\tool\tctl"
        Write-Host "::endgroup::"

        Write-Host "::group::Signing tctl..."
        Invoke-SignBinary -UnsignedBinaryPath "$UnsignedBinaryPath" -SignedBinaryPath "$SignedBinaryPath"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built TCTL in {0:g}" -f $CommandDuration)

    return "$SignedBinaryPath"  # This is needed for bundling the zip archive
}

function Package-Artifacts {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion,
        [Parameter(Mandatory)]
        [string] $SignedTctlBinaryPath,
        [Parameter(Mandatory)]
        [string] $SignedTshBinaryPath
    )

    $CommandDuration = Measure-Block {
        $PackageDirectory = New-TempDirectory
        Write-Host "Packaging zip archive $PackageDirectory..."
        Copy-Item -Path "$SignedTctlBinaryPath" -Destination "$PackageDirectory"
        Copy-Item -Path "$SignedTshBinaryPath" -Destination "$PackageDirectory"
        Copy-Item -Path "$TeleportSourceDirectory\CHANGELOG.md" -Destination "$PackageDirectory"
        Copy-Item -Path "$TeleportSourceDirectory\README.md" -Destination "$PackageDirectory"
        Out-File -FilePath "$PackageDirectory\VERSION" -InputObject "v$TeleportVersion"
        Compress-Archive -Path "$PackageDirectory\*" -DestinationPath "$ArtifactDirectory\teleport-v$TeleportVersion-windows-amd64-bin.zip"
    }
    Write-Host $("Created archive in {0:g}" -f $CommandDuration)

    return
}

function Build-Connect {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion,
        [Parameter(Mandatory)]
        [string] $SignedTshBinaryPath
    )

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building Teleport Connect..."
        $env:CONNECT_TSH_BIN_PATH = "$SignedTshBinaryPath"
        yarn install --frozen-lockfile
        yarn build-term
        yarn package-term "-c.extraMetadata.version=$TeleportVersion"
        $BinaryName = "Teleport Connect Setup-$TeleportVersion.exe"
        Invoke-SignBinary -UnsignedBinaryPath "$TeleportSourceDirectory\web\packages\teleterm\build\release\$BinaryName" `
            -SignedBinaryPath "$ArtifactDirectory\$BinaryName"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built Teleport Connect in {0:g}" -f $CommandDuration)
}

function Build-Artifacts {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory
    )
    Write-Host "Starting build process for Teleport $TeleportVersion..."

    # Setup for builds
    Install-BuildRequirements -InstallDirectory "C:\toolchains" -TeleportSourceDirectory "$TeleportSourceDirectory"

    # Create the artifact output directory
    New-Item -Path "$ArtifactDirectory" -ItemType Directory -Force | Out-Null

    # Build tctl
    $SignedTctlBinaryPath = Build-Tctl `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion"

    # Build tsh
    $SignedTshBinaryPath = Build-Tsh `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion"

    # Create archive
    Package-Artifacts `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion" `
        -SignedTshBinaryPath "$SignedTshBinaryPath" `
        -SignedTctlBinaryPath "$SignedTctlBinaryPath"

    # Build Teleport Connect
    Build-Connect `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion" `
        -SignedTshBinaryPath "$SignedTshBinaryPath" 

    # Build Windows Authentication Package
    Build-WindowsAuthenticationPackage `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion"

    # Copy artifacts to output directory
    Write-Host "::group::Generating artifact checksums..."
    Format-FileHashes -PathGlob "$ArtifactDirectory\*"
    Write-Host "::endgroup::"

    Write-Host "Build complete"
}
