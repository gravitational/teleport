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

    $TempDirectoryPath = Join-Path -Path "$([System.IO.Path]::GetTempPath())" -ChildPath "$([guid]::newguid().Guid)"
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
        # we explicitly set the default host triple to the -windows-gnu
        # toolchain because the default is the -windows-msvc triple and cross
        # compilation with the -gnu target (which is all we use) fails with the
        # e/windowsauth build
        & "$ToolchainDir\rustup-init.exe" --profile minimal -y --default-host x86_64-pc-windows-gnu --default-toolchain "$RustVersion-x86_64-pc-windows-gnu"
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
        corepack enable pnpm
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

function Install-WasmDeps {
    <#
    .SYNOPSIS
        Builds and installs wasm-bindgen-cli, wasm-opt, and wasm32-unknown-unknown toolchain.
    #>

    Write-Host "::group::Installing wasm-bindgen-cli, wasm-opt, and wasm32-unknown-unknown toolchain"
    make -C "$TeleportSourceDirectory" ensure-wasm-deps
    Write-Host "::endgroup::"
}

function Install-Wintun {
    <#
    .SYNOPSIS
        Downloads wintun.dll into the supplied dir
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $InstallDir
    )
    begin {
        Write-Host "::group::Installing wintun.dll to $InstallDir..."
        New-Item -Path "$InstallDir" -ItemType Directory -Force | Out-Null
        $WintunZipfile = "$InstallDir/wintun.zip"
        Invoke-WebRequest -Uri https://www.wintun.net/builds/wintun-0.14.1.zip -OutFile $WintunZipfile
        $ExpectedHash = "07C256185D6EE3652E09FA55C0B673E2624B565E02C4B9091C79CA7D2F24EF51"
        $ZipFileHash = Get-FileHash -Path $WintunZipFile -Algorithm SHA256
        if ($ZipFileHash.Hash -ne $ExpectedHash) {
            Write-Host "checksum: $ZipFileHash"
            throw "Checksum verification for wintun.zip failed! Expected $ExpectedHash but got $($ZipFileHash.Hash)"
        }
        Expand-Archive -Force -Path $WintunZipfile -DestinationPath $InstallDir
        Move-Item -Force -Path "$InstallDir/wintun/bin/amd64/wintun.dll" -Destination "$InstallDir/wintun.dll"
        Write-Host "::endgroup::"
    }
}

function Compile-Message-File {
    <#
    .SYNOPSIS
        Compiles msgfile.mc into msgfile.dll in the supplied directory.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $MessageFile,
        [Parameter(Mandatory)]
        [string] $CompileDir
    )
    begin {
        Write-Host "::group::Compiling msgfile.dll to $CompileDir..."
        New-Item -Path "$CompileDir" -ItemType Directory -Force | Out-Null
        $SDKRegistry = "HKLM:\SOFTWARE\WOW6432Node\Microsoft\Microsoft SDKs\Windows\v10.0"
        $SDKInstallationDir = $(Get-Item $SDKRegistry).GetValue("InstallationFolder")
        $SDKVersion = $(Get-Item $SDKRegistry).GetValue("ProductVersion")
        $SDKBinDir = "${SDKInstallationDir}bin\${SDKVersion}.0\x64\"

        # Compile .mc to .rc.
        .$SDKBinDir\mc.exe -h "$CompileDir" -r "$CompileDir" "$MessageFile"

        # Compile .rc to .res in the same directory as the input file.
        $MessageFileBasename = $(Get-Item $MessageFile).Basename
        .$SDKBinDir\rc.exe "$CompileDir\$MessageFileBasename.rc"

        # Compile .res to .dll.
        $LinkExe = vswhere.exe -find **\Hostx64\x64\link.exe | Select -First 1
        .$LinkExe -dll -noentry -out:"$CompileDir\$MessageFileBasename.dll" "$CompileDir\$MessageFileBasename.res" /MACHINE:X64

        Write-Host "::endgroup::"
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
        [string] $Workspace
    )
    begin {
        New-Item -Path "$Workspace" -ItemType Directory -Force | Out-Null
        Invoke-WebRequest $url -UseBasicParsing -OutFile "$Workspace\relcli.exe"
    }
}

function Generate-Artifacts {
    <#
    .SYNOPSIS
        Invokes relcli to automatically generate manfiests for built artifacts
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $Workspace,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory
    )

    $SearchPath = Join-Path -Path $ArtifactDirectory -ChildPath *
    Get-ChildItem -Path $SearchPath -Include "*.exe","*.zip" | ForEach-Object {
        switch -Wildcard ($_.Name) {
            "Teleport Connect Setup*.exe" {
                $description = "Teleport Connect"
                Break
            }
            "teleport-windows-auth-setup*.exe" {
                $description = "Teleport Authentication Package"
                Break
            }
            "teleport*.zip" {
                $description = "Windows (64-bit, tsh client only)"
                Break
            }
            "*" {
                # Unmatched file, skip it
                Write-Host "Skipping $_"
                return
            }
        }

        & "$Workspace\relcli.exe" generate-manifest --path $_.FullName `
            --products teleport --products teleport-ent `
            --os "windows" --architecture "amd64" `
            --description $description
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

        Install-WasmDeps
    }
    Write-Host $("All build requirements installed in {0:g}" -f $CommandDuration)
}

function Invoke-SignBinary {
    <#
    .SYNOPSIS
    Signs the provided binary with the base64-encoded certificate listed in "$WINDOWS_SIGNING_CERT"
    .PARAMETER UnsignedBinaryPath
    The path to the unsigned binary.
    .PARAMETER SignedBinaryPath
    The path where the signed binary should be written. If not provided, then the signed binary will
    be written to a temporary path, and then moved to the unsigned binary path.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $UnsignedBinaryPath,

        [Parameter()]
        [string] $SignedBinaryPath
    )

    if (! $SignedBinaryPath) {
        $ShouldMoveSignedBinary = $true
        $SignedBinaryPath = Join-Path -Path $(New-TempDirectory) -ChildPath "signed.exe"
    }

    Write-Host "Signing $UnsignedBinaryPath using WSL sign-binary script:"
    wsl-ubuntu-command sign-binary "$UnsignedBinaryPath" "$SignedBinaryPath"

    if ($ShouldMoveSignedBinary) {
        Move-Item -Path $SignedBinaryPath -Destination $UnsignedBinaryPath -Force
    }
}

function Get-SHA256Hex {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $Path
    )

    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-WindowsAuthDLLReady {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [Parameter(Mandatory)]
        [string] $ExpectedSignerCN
    )

    $Bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($Bytes.Length -lt 0x100) {
        throw "windowsauth DLL is too small to be a valid PE file: $Path"
    }
    if ($Bytes[0] -ne 0x4d -or $Bytes[1] -ne 0x5a) {
        throw "windowsauth DLL does not have an MZ header: $Path"
    }

    $PEOffset = [System.BitConverter]::ToUInt32($Bytes, 0x3c)
    if ($PEOffset + 24 + 72 -gt $Bytes.Length) {
        throw "windowsauth DLL has an invalid PE header offset: $Path"
    }
    if ($Bytes[$PEOffset] -ne 0x50 -or $Bytes[$PEOffset + 1] -ne 0x45 -or
        $Bytes[$PEOffset + 2] -ne 0 -or $Bytes[$PEOffset + 3] -ne 0) {
        throw "windowsauth DLL does not have a PE header: $Path"
    }

    $COFFOffset = $PEOffset + 4
    $Characteristics = [System.BitConverter]::ToUInt16($Bytes, $COFFOffset + 18)
    if (($Characteristics -band 0x2000) -eq 0) {
        throw "windowsauth file is not marked as a DLL: $Path"
    }

    $OptionalHeaderOffset = $COFFOffset + 20
    $DLLCharacteristics = [System.BitConverter]::ToUInt16($Bytes, $OptionalHeaderOffset + 70)
    if (($DLLCharacteristics -band 0x80) -eq 0) {
        throw "windowsauth DLL does not have FORCE_INTEGRITY set: DllCharacteristics=0x$($DLLCharacteristics.ToString("x4"))"
    }

    $Signature = Get-AuthenticodeSignature -FilePath $Path
    if ($Signature.Status -ne "Valid") {
        throw "windowsauth DLL Authenticode signature is not valid: $($Signature.Status)"
    }
    $ActualSignerCN = $Signature.SignerCertificate.GetNameInfo(
        [System.Security.Cryptography.X509Certificates.X509NameType]::SimpleName,
        $false
    )
    if ($ActualSignerCN -ne $ExpectedSignerCN) {
        throw "unexpected windowsauth DLL signer CN: $ActualSignerCN"
    }
}

function Get-PinnedWindowsAuthDLL {
    <#
    .SYNOPSIS
    Downloads the Microsoft-signed windowsauth DLL pinned by
    build.assets/windowsauth.version and build.assets/windowsauth.sha256,
    verifies it against the release manifest, and places it at
    e/windowsauth/installer/teleport.dll for the installer build to embed.
    .OUTPUTS
    string - the pinned windowsauth version that was fetched
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $Bucket,
        [Parameter()]
        [string] $ReleasePrefix = ""
    )

    $PinVersionPath = "$TeleportSourceDirectory\build.assets\windowsauth.version"
    $PinChecksumPath = "$TeleportSourceDirectory\build.assets\windowsauth.sha256"
    $DestinationDLLPath = "$TeleportSourceDirectory\e\windowsauth\installer\teleport.dll"
    $ExpectedMicrosoftSignerCN = "Microsoft Windows Software Compatibility Publisher"
    $ExpectedMicrosoftSignerSubject = "CN=$ExpectedMicrosoftSignerCN"

    if ([string]::IsNullOrWhiteSpace($Bucket)) {
        throw "windowsauth releases bucket must be set"
    }
    if (-not (Test-Path $PinVersionPath)) {
        throw "windowsauth pin file not found: $PinVersionPath"
    }
    if (-not (Test-Path $PinChecksumPath)) {
        throw "windowsauth checksum pin file not found: $PinChecksumPath"
    }

    $PinnedVersion = (Get-Content $PinVersionPath -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($PinnedVersion)) {
        throw "windowsauth pin file $PinVersionPath is empty"
    }
    $PinnedVersionKeyPart = $PinnedVersion.TrimStart("v", "V")
    if ([string]::IsNullOrWhiteSpace($PinnedVersionKeyPart)) {
        throw "windowsauth pin version $PinnedVersion is not valid for use in an S3 key"
    }

    $PinnedChecksumFields = (Get-Content $PinChecksumPath -Raw).Trim() -split '\s+', 2
    if ($PinnedChecksumFields.Count -ne 2) {
        throw "windowsauth checksum pin file $PinChecksumPath must use '<sha256>  <filename>' format"
    }
    $PinnedSHA256 = $PinnedChecksumFields[0].ToLowerInvariant()
    if ($PinnedSHA256 -notmatch '^[0-9a-f]{64}$') {
        throw "windowsauth checksum pin file $PinChecksumPath must start with a SHA256 hex digest"
    }
    $PinnedDLLFileName = $PinnedChecksumFields[1].Trim()
    if ([string]::IsNullOrWhiteSpace($PinnedDLLFileName)) {
        throw "windowsauth checksum pin file $PinChecksumPath must include the pinned DLL filename"
    }

    $ManifestFileName = "manifest.json"
    $ManifestKeyPrefix = $PinnedVersionKeyPart
    if (-not [string]::IsNullOrWhiteSpace($ReleasePrefix)) {
        $ManifestKeyPrefix = "$ReleasePrefix/$PinnedVersionKeyPart"
    }
    $ManifestKey = "$ManifestKeyPrefix/$ManifestFileName"
    $ManifestPath = Join-Path (New-TempDirectory) $ManifestFileName
    $WorkDirectory = Split-Path -Parent $ManifestPath

    try {
        $ManifestURI = "s3://$Bucket/$ManifestKey"
        Write-Host "Downloading pinned windowsauth manifest from $ManifestURI"
        aws s3 cp $ManifestURI $ManifestPath --no-progress
        if ($LastExitCode -ne 0) {
            throw "failed to download pinned windowsauth manifest from $ManifestURI"
        }

        $Manifest = Get-Content $ManifestPath -Raw | ConvertFrom-Json
        if ($Manifest.schema_version -ne 1) {
            throw "unsupported windowsauth manifest schema_version: $($Manifest.schema_version)"
        }
        if ($Manifest.windowsauth_version -ne $PinnedVersion) {
            throw "windowsauth manifest version $($Manifest.windowsauth_version) does not match pinned version $PinnedVersion"
        }
        if ("$($Manifest.source_files_sha256)" -notmatch '^[0-9a-fA-F]{64}$') {
            throw "windowsauth manifest is missing a valid source_files_sha256"
        }

        $ManifestDLLSHA256 = "$($Manifest.microsoft_signed_dll.sha256)".ToLowerInvariant()
        if ($ManifestDLLSHA256 -ne $PinnedSHA256) {
            throw "windowsauth manifest DLL checksum $ManifestDLLSHA256 does not match repo pin $PinnedSHA256"
        }
        $ManifestDLLFileName = "$($Manifest.microsoft_signed_dll.filename)"
        if ($ManifestDLLFileName -ne $PinnedDLLFileName) {
            throw "windowsauth manifest DLL filename $ManifestDLLFileName does not match repo pin $PinnedDLLFileName"
        }
        if ($Manifest.microsoft_signed_dll.PSObject.Properties.Name -contains "expected_signer_subject") {
            $ManifestSignerSubject = "$($Manifest.microsoft_signed_dll.expected_signer_subject)"
            if (-not [string]::IsNullOrWhiteSpace($ManifestSignerSubject) -and
                $ManifestSignerSubject -ne $ExpectedMicrosoftSignerSubject) {
                throw "windowsauth manifest signer $ManifestSignerSubject does not match expected signer $ExpectedMicrosoftSignerSubject"
            }
        }

        $DLLKey = "$ManifestKeyPrefix/$ManifestDLLFileName"
        if ($Manifest.microsoft_signed_dll.PSObject.Properties.Name -contains "s3_key") {
            $DLLKey = "$($Manifest.microsoft_signed_dll.s3_key)"
        }
        if ([string]::IsNullOrWhiteSpace($DLLKey) -or $DLLKey.StartsWith("s3://")) {
            throw "windowsauth manifest has invalid microsoft_signed_dll.s3_key: $DLLKey"
        }

        $DownloadedDLLPath = Join-Path $WorkDirectory $ManifestDLLFileName
        $DLLURI = "s3://$Bucket/$DLLKey"
        Write-Host "Downloading pinned Microsoft-signed windowsauth DLL from $DLLURI"
        aws s3 cp $DLLURI $DownloadedDLLPath --no-progress
        if ($LastExitCode -ne 0) {
            throw "failed to download pinned Microsoft-signed windowsauth DLL from $DLLURI"
        }

        $ActualDLLSHA256 = Get-SHA256Hex -Path $DownloadedDLLPath
        if ($ActualDLLSHA256 -ne $PinnedSHA256) {
            throw "checksum mismatch for pinned windowsauth DLL: expected $PinnedSHA256, got $ActualDLLSHA256"
        }

        Assert-WindowsAuthDLLReady -Path $DownloadedDLLPath -ExpectedSignerCN $ExpectedMicrosoftSignerCN

        New-Item -ItemType Directory -Path (Split-Path -Parent $DestinationDLLPath) -Force | Out-Null
        Copy-Item -Path $DownloadedDLLPath -Destination $DestinationDLLPath -Force
    } finally {
        Remove-Item -Path $WorkDirectory -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Verified pinned Microsoft-signed windowsauth DLL $PinnedVersion (sha256 $PinnedSHA256)"
    return $PinnedVersion
}

function Build-WindowsAuthenticationPackage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion,

        [Parameter()]
        [ValidateSet("source", "pinned")]
        [string] $WindowsAuthBuildMode = "pinned",

        [Parameter()]
        [string] $Environment = ""
    )

    if ($Environment.StartsWith("prod") -and $WindowsAuthBuildMode -eq "source") {
        throw "windowsauth build mode 'source' is not allowed when Environment is '$Environment'; production builds must consume a pinned, Microsoft-signed windowsauth DLL"
    }

    $CommandDuration = Measure-Block {
        $WindowsAuthDirectory = "$TeleportSourceDirectory\e\windowsauth"

        if ($WindowsAuthBuildMode -eq "pinned") {
            # The pinned DLL is expected to already be present at
            # installer/teleport.dll, fetched by a dedicated workflow step
            # that runs (and finishes) before the code signing role is
            # assumed -- see build-windows.yaml. `installer-exe` fails
            # clearly if it is missing.
            Write-Host "::group::Building Windows auth installer from pinned DLL..."
            make -C "$WindowsAuthDirectory" VERSION="v$TeleportVersion" installer-exe
            Write-Host "::endgroup::"
        } else {
            Write-Host "::group::Building Windows auth setup from source..."
            make -C "$WindowsAuthDirectory" VERSION="v$TeleportVersion" all
            Write-Host "::endgroup::"
        }

        Write-Host "::group::Signing Windows auth setup..."
        $BinaryName = "teleport-windows-auth-setup-v$TeleportVersion-amd64.exe"
        Invoke-SignBinary -UnsignedBinaryPath "$WindowsAuthDirectory\build\$BinaryName" -SignedBinaryPath "$ArtifactDirectory\$BinaryName"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built Windows authentication package in {0:g}" -f $CommandDuration)
}

function Get-KubectlVersionLDFlag {
    <#
    .SYNOPSIS
        Returns the linker flag that adds the bundled kubectl version.

    .OUTPUTS
    string
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory
    )

    Push-Location "$TeleportSourceDirectory"
    try {
        $KubectlVersion = (go list -m -f '{{.Version}}' k8s.io/kubectl) -replace '^v0\.', 'v1.'
        if ($LastExitCode -ne 0) {
            exit $LastExitCode
        }
    } finally {
        Pop-Location
    }

    return "-X k8s.io/component-base/version.gitVersion=$KubectlVersion"
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
    $BuildTypeLDFlags = "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=community"
    $KubectlLDFlags = Get-KubectlVersionLDFlag -TeleportSourceDirectory "$TeleportSourceDirectory"

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building tsh..."

        # The --target must be set explicitly so the staticlib is emitted to
        # target/x86_64-pc-windows-gnu/release, which is where the cgo LDFLAGS
        # look for it.
        cargo build -p rdp-decoder --release --locked --target x86_64-pc-windows-gnu
        $UnsignedBinaryPath = "$BuildDirectory\unsigned-$BinaryName"
        # -Wl,--gc-sections lets the linker drop unreachable native code.
        go build -tags "grpcnotrace piv rust_rdp_decoder kustomize_disable_go_plugin_support" -trimpath -ldflags "-s -w $BuildTypeLDFlags $KubectlLDFlags -extldflags=-Wl,--gc-sections" -o "$UnsignedBinaryPath" "$TeleportSourceDirectory\tool\tsh"
        if ($LastExitCode -ne 0) {
            exit $LastExitCode
        }
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
    $BuildTypeLDFlags = "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=community"
    $KubectlLDFlags = Get-KubectlVersionLDFlag -TeleportSourceDirectory "$TeleportSourceDirectory"

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building tctl..."
        $UnsignedBinaryPath = "$BuildDirectory\unsigned-$BinaryName"
        go build -tags "grpcnotrace piv kustomize_disable_go_plugin_support" -trimpath -ldflags "-s -w $BuildTypeLDFlags $KubectlLDFlags" -o "$UnsignedBinaryPath" "$TeleportSourceDirectory\tool\tctl"
        if ($LastExitCode -ne 0) {
            exit $LastExitCode
        }
        Write-Host "::endgroup::"

        Write-Host "::group::Signing tctl..."
        Invoke-SignBinary -UnsignedBinaryPath "$UnsignedBinaryPath" -SignedBinaryPath "$SignedBinaryPath"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built TCTL in {0:g}" -f $CommandDuration)

    return "$SignedBinaryPath"  # This is needed for bundling the zip archive
}

function Build-Tbot {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion
    )

    $BinaryName = "tbot.exe"
    $BuildDirectory = "$TeleportSourceDirectory\build"
    $SignedBinaryPath = "$BuildDirectory\$BinaryName"
    $BuildTypeLDFlags = "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=community"
    $KubectlLDFlags = Get-KubectlVersionLDFlag -TeleportSourceDirectory "$TeleportSourceDirectory"

    $CommandDuration = Measure-Block {
        Write-Host "::group::Building tbot..."
        $UnsignedBinaryPath = "$BuildDirectory\unsigned-$BinaryName"
        go build -tags "grpcnotrace kustomize_disable_go_plugin_support" -trimpath -ldflags "-s -w $BuildTypeLDFlags $KubectlLDFlags" -o "$UnsignedBinaryPath" "$TeleportSourceDirectory\tool\tbot"
        if ($LastExitCode -ne 0) {
            exit $LastExitCode
        }
        Write-Host "::endgroup::"

        Write-Host "::group::Signing tbot..."
        Invoke-SignBinary -UnsignedBinaryPath "$UnsignedBinaryPath" -SignedBinaryPath "$SignedBinaryPath"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built tbot in {0:g}" -f $CommandDuration)

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
        [string] $SignedTshBinaryPath,
        [Parameter(Mandatory)]
        [string] $SignedTBotBinaryPath
    )

    $CommandDuration = Measure-Block {
        $PackageDirectory = New-TempDirectory
        Write-Host "Packaging zip archive $PackageDirectory..."
        Copy-Item -Path "$SignedTctlBinaryPath" -Destination "$PackageDirectory"
        Copy-Item -Path "$SignedTshBinaryPath" -Destination "$PackageDirectory"
        Copy-Item -Path "$SignedTbotBinaryPath" -Destination "$PackageDirectory"
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
        Install-Wintun -InstallDir "$TeleportSourceDirectory\wintun"
        Compile-Message-File -MessageFile "$TeleportSourceDirectory\lib\utils\log\eventlog\msgfile.mc" -CompileDir "$TeleportSourceDirectory\msgfile"
        $env:CONNECT_WINTUN_DLL_PATH = "$TeleportSourceDirectory\wintun\wintun.dll"
        $env:CONNECT_MSGFILE_DLL_PATH = "$TeleportSourceDirectory\msgfile\msgfile.dll"
        $env:CONNECT_TSH_BIN_PATH = "$SignedTshBinaryPath"
        pnpm install --frozen-lockfile
        pnpm build-term
        pnpm package-term "-c.extraMetadata.version=$TeleportVersion"
        $BinaryName = "Teleport Connect Setup-$TeleportVersion.exe"
        Invoke-SignBinary -UnsignedBinaryPath "$TeleportSourceDirectory\web\packages\teleterm\build\release\$BinaryName" `
            -SignedBinaryPath "$ArtifactDirectory\$BinaryName"
        Write-Host "::endgroup::"
    }
    Write-Host $("Built Teleport Connect in {0:g}" -f $CommandDuration)
}

function Write-Version-Objects {
    <#
    .SYNOPSIS
    Produces Windows resource files containing version info metadata
    for tsh and tctl. These files are automatically read by the go
    tool during compilation.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion
    )

    Write-Host "Generating version info files for Windows artifacts"

    # install go-winres (v0.3.3)
    go install github.com/tc-hib/go-winres@d743268d7ea168077ddd443c4240562d4f5e8c3e
    $GoWinres = Join-Path -Path $(go env GOPATH) -ChildPath "bin\go-winres.exe"

    $Year = (Get-Date).Year

    # generate tsh version info
    & $GoWinres simply --no-suffix --arch amd64 `
        --file-description "Teleport tsh command-line client" `
        --original-filename tsh.exe `
        --copyright "Copyright (C) $Year Gravitational Inc." `
        --icon "$TeleportSourceDirectory\e\windowsauth\installer\teleport.ico" `
        --product-name Teleport `
        --product-version $TeleportVersion `
        --file-version $TeleportVersion `
        --out "$TeleportSourceDirectory\tool\tsh\resource.syso"

    # generate tctl version info
    & $GoWinres simply --no-suffix --arch amd64 `
        --file-description "Teleport tctl administrative tool" `
        --original-filename tctl.exe `
        --copyright "Copyright (C) $Year Gravitational Inc." `
        --icon "$TeleportSourceDirectory\e\windowsauth\installer\teleport.ico" `
        --product-name Teleport `
        --product-version $TeleportVersion `
        --file-version $TeleportVersion `
        --out "$TeleportSourceDirectory\tool\tctl\resource.syso"

    # generate tbot version info
    & $GoWinres simply --no-suffix --arch amd64 `
        --file-description "Teleport Machine and Workload Identity agent" `
        --original-filename tbot.exe `
        --copyright "Copyright (C) $Year Gravitational Inc." `
        --icon "$TeleportSourceDirectory\e\windowsauth\installer\teleport.ico" `
        --product-name Teleport `
        --product-version $TeleportVersion `
        --file-version $TeleportVersion `
        --out "$TeleportSourceDirectory\tool\tbot\resource.syso"

    # generate windowsauth version info (note the --admin flag, as the installer must run as admin)
    & $GoWinres simply --no-suffix --arch amd64 --admin `
        --file-description "Teleport Authentication Package" `
        --original-filename "teleport-windows-auth-setup-v$TeleportVersion-amd64.exe" `
        --copyright "Copyright (C) $Year Gravitational Inc." `
        --icon "$TeleportSourceDirectory\e\windowsauth\installer\teleport.ico" `
        --product-name Teleport `
        --product-version $TeleportVersion `
        --file-version $TeleportVersion `
        --out "$TeleportSourceDirectory\e\windowsauth\installer\resource.syso"
}

function Build-Artifacts {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $TeleportSourceDirectory,
        [Parameter(Mandatory)]
        [string] $TeleportVersion,
        [Parameter(Mandatory)]
        [string] $ArtifactDirectory,

        [Parameter()]
        [ValidateSet("source", "pinned")]
        [string] $WindowsAuthBuildMode = "pinned",

        [Parameter()]
        [string] $Environment = ""
    )
    if ($Environment.StartsWith("prod") -and $WindowsAuthBuildMode -eq "source") {
        throw "windowsauth build mode 'source' is not allowed when Environment is '$Environment'; production builds must consume a pinned, Microsoft-signed windowsauth DLL"
    }

    Write-Host "Starting build process for Teleport $TeleportVersion..."

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

    # Build TBot
    $SignedTbotBinaryPath = Build-Tbot `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion"

    # Create archive
    Package-Artifacts `
        -TeleportSourceDirectory "$TeleportSourceDirectory" `
        -ArtifactDirectory "$ArtifactDirectory" `
        -TeleportVersion "$TeleportVersion" `
        -SignedTshBinaryPath "$SignedTshBinaryPath" `
        -SignedTctlBinaryPath "$SignedTctlBinaryPath" `
        -SignedTBotBinaryPath "$SignedTBotBinaryPath"

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
        -TeleportVersion "$TeleportVersion" `
        -WindowsAuthBuildMode "$WindowsAuthBuildMode" `
        -Environment "$Environment"

    Write-Host "Build complete"
}
