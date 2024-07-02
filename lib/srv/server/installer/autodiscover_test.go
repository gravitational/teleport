/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package installer

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/buildkite/bintest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
)

func buildMockBins(t *testing.T) (map[string]*bintest.Mock, binariesLocation, []func() error) {
	mockedBins := []string{"systemctl",
		"apt-get", "apt-key",
		"rpm",
		"yum", "yum-config-manager",
		"zypper",
		"teleport",
	}

	mapMockBins := make(map[string]*bintest.Mock)
	releaseMockFNs := make([]func() error, 0, len(mockedBins))
	for _, mockBinName := range mockedBins {
		mockBin, err := bintest.NewMock(mockBinName)
		require.NoError(t, err)
		mapMockBins[mockBinName] = mockBin

		releaseMockFNs = append(releaseMockFNs, mockBin.Close)
	}

	return mapMockBins, binariesLocation{
		systemctl:        mapMockBins["systemctl"].Path,
		aptGet:           mapMockBins["apt-get"].Path,
		aptKey:           mapMockBins["apt-key"].Path,
		rpm:              mapMockBins["rpm"].Path,
		yum:              mapMockBins["yum"].Path,
		yumConfigManager: mapMockBins["yum-config-manager"].Path,
		zypper:           mapMockBins["zypper"].Path,
		teleport:         mapMockBins["teleport"].Path,
	}, releaseMockFNs
}

func TestAutodiscoverNode(t *testing.T) {
	ctx := context.Background()

	mockRepoKeys := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("my-public-key"))
	}))

	mockBins, binariesLocation, releaseMockedBinsFN := buildMockBins(t)
	t.Cleanup(func() {
		for _, releaseMockBin := range releaseMockedBinsFN {
			assert.NoError(t, releaseMockBin())
		}
	})

	azureIMDSServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/versions"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apiVersions":["2019-06-04"]}`))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"resourceId":"test-id", "location":"eastus", "resourceGroupName":"TestGroup", ` +
				`"subscriptionId": "5187AF11-3581-4AB6-A654-59405CD40C44", "vmId":"ED7DAC09-6E73-447F-BD18-AF4D1196C1E4"}`))
		}
	}))
	mockIMDSProviders := []func(ctx context.Context) (imds.Client, error){
		func(ctx context.Context) (imds.Client, error) {
			return azure.NewInstanceMetadataClient(azure.WithBaseURL(azureIMDSServer.URL)), nil
		},
	}

	t.Run("well known distros", func(t *testing.T) {
		for distroName, distroVersions := range wellKnownOS {
			for distroVersion, distroConfig := range distroVersions {
				t.Run(distroName+":"+distroVersion, func(t *testing.T) {
					testTempDir := t.TempDir()

					// Common folders to all distros
					require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/etc"), fs.ModePerm))
					require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/local/bin"), fs.ModePerm))
					require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/share"), fs.ModePerm))
					require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/var/lock"), fs.ModePerm))

					for fileName, contents := range distroConfig {
						isDir := strings.HasSuffix(fileName, "/")
						if isDir {
							require.Empty(t, contents, "expected no contents for directory %q", fileName)
							require.NoError(t, os.MkdirAll(path.Join(testTempDir, fileName), fs.ModePerm))
						} else {
							filePathWithoutParent := path.Base(fileName)
							require.NoError(t, os.MkdirAll(path.Join(testTempDir, filePathWithoutParent), fs.ModePerm))
							require.NoError(t, os.WriteFile(path.Join(testTempDir, fileName), []byte(contents), fs.ModePerm))
						}
					}

					installerConfig := &AutodiscoverNodeInstallerConfig{
						RepositoryChannel: "stable/rolling",
						AutoUpgrades:      false,
						ProxyPublicAddr:   "proxy.example.com",
						TeleportPackage:   "teleport",
						TokenName:         "my-token",
						AzureClientID:     "azure-client-id",

						fsRootPrefix:         testTempDir,
						imdsProviders:        mockIMDSProviders,
						binariesLocation:     binariesLocation,
						aptPublicKeyEndpoint: mockRepoKeys.URL,
					}

					teleportInstaller, err := NewAutodiscoverNodeInstaller(installerConfig)
					require.NoError(t, err)

					// One of the first things the install command does is to check if teleport is already installed.
					// If so, it stops the installation with success.
					// Given that we are mocking the binary, it means it already exists and as such, the installation will stop.
					// To prevent that, we must rename the file, call `<pakageManager> install teleport` and rename it back.
					teleportInitialPath := mockBins["teleport"].Path
					teleportHiddenPath := teleportInitialPath + "-hidden"
					require.NoError(t, os.Rename(teleportInitialPath, teleportHiddenPath))

					switch distroName {
					case "ubuntu", "debian":
						mockBins["apt-get"].Expect("update")
						mockBins["apt-get"].Expect("install", "-y", "teleport").AndCallFunc(func(c *bintest.Call) {
							assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
							c.Exit(0)
						})
					case "amzn", "rhel", "centos":
						mockBins["yum"].Expect("install", "-y", "yum-utils")
						mockBins["rpm"].Expect("--eval", bintest.MatchAny())
						mockBins["yum-config-manager"].Expect("--add-repo", bintest.MatchAny())
						mockBins["yum"].Expect("install", "-y", "teleport").AndCallFunc(func(c *bintest.Call) {
							assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
							c.Exit(0)
						})
					case "sles":
						mockBins["rpm"].Expect("--import", zypperPublicKeyEndpoint)
						mockBins["rpm"].Expect("--eval", bintest.MatchAny())
						mockBins["zypper"].Expect("--non-interactive", "addrepo", bintest.MatchAny())
						mockBins["zypper"].Expect("--gpg-auto-import-keys", "refresh")
						mockBins["zypper"].Expect("--non-interactive", "install", "-y", "teleport").AndCallFunc(func(c *bintest.Call) {
							assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
							c.Exit(0)
						})
					}

					mockBins["teleport"].Expect("node",
						"configure",
						"--output=file://"+testTempDir+"/etc/teleport.yaml",
						"--proxy=proxy.example.com",
						"--join-method=azure",
						"--token=my-token",
						"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
						"--azure-client-id=azure-client-id",
					)

					mockBins["systemctl"].Expect("enable", "--now", "teleport")

					require.NoError(t, teleportInstaller.Install(ctx))

					for binName, mockBin := range mockBins {
						require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
					}
				})
			}
		}
	})

	t.Run("with automatic upgrades", func(t *testing.T) {
		distroName := "ubuntu"
		distroVersion := "24.04"
		distroConfig := wellKnownOS[distroName][distroVersion]

		proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// We only expect calls to the automatic upgrade default channel's version endpoint.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("v15.4.0\n"))
		}))
		t.Cleanup(func() {
			proxyServer.Close()
		})
		proxyPublicAddr := proxyServer.Listener.Addr().String()

		testTempDir := t.TempDir()

		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/etc"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/local/bin"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/share"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/var/lock"), fs.ModePerm))

		for fileName, contents := range distroConfig {
			isDir := strings.HasSuffix(fileName, "/")
			if isDir {
				require.Empty(t, contents, "expected no contents for directory %q", fileName)
				require.NoError(t, os.MkdirAll(path.Join(testTempDir, fileName), fs.ModePerm))
			} else {
				filePathWithoutParent := path.Base(fileName)
				require.NoError(t, os.MkdirAll(path.Join(testTempDir, filePathWithoutParent), fs.ModePerm))
				require.NoError(t, os.WriteFile(path.Join(testTempDir, fileName), []byte(contents), fs.ModePerm))
			}
		}

		installerConfig := &AutodiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      true,
			ProxyPublicAddr:   proxyPublicAddr,
			TeleportPackage:   "teleport-ent",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:           testTempDir,
			imdsProviders:          mockIMDSProviders,
			binariesLocation:       binariesLocation,
			aptPublicKeyEndpoint:   mockRepoKeys.URL,
			autoUpgradesChannelURL: proxyServer.URL,
		}

		teleportInstaller, err := NewAutodiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// One of the first things the install command does is to check if teleport is already installed.
		// If so, it stops the installation with success.
		// Given that we are mocking the binary, it means it already exists and as such, the installation will stop.
		// To prevent that, we must rename the file, call `<pakageManager> install teleport` and rename it back.
		teleportInitialPath := mockBins["teleport"].Path
		teleportHiddenPath := teleportInitialPath + "-hidden"
		require.NoError(t, os.Rename(teleportInitialPath, teleportHiddenPath))

		mockBins["apt-get"].Expect("update")
		mockBins["apt-get"].Expect("install", "-y", "teleport-ent-updater=15.4.0", "teleport-ent=15.4.0").AndCallFunc(func(c *bintest.Call) {
			assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
			c.Exit(0)
		})

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml",
			"--proxy="+proxyPublicAddr,
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		)

		mockBins["systemctl"].Expect("enable", "--now", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
	})

	t.Run("fails when imds server is not available", func(t *testing.T) {
		distroName := "ubuntu"
		distroVersion := "24.04"
		distroConfig := wellKnownOS[distroName][distroVersion]
		proxyPublicAddr := "proxy.example.com"

		testTempDir := t.TempDir()

		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/etc"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/local/bin"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/usr/share"), fs.ModePerm))
		require.NoError(t, os.MkdirAll(path.Join(testTempDir, "/var/lock"), fs.ModePerm))

		for fileName, contents := range distroConfig {
			isDir := strings.HasSuffix(fileName, "/")
			if isDir {
				require.Empty(t, contents, "expected no contents for directory %q", fileName)
				require.NoError(t, os.MkdirAll(path.Join(testTempDir, fileName), fs.ModePerm))
			} else {
				filePathWithoutParent := path.Base(fileName)
				require.NoError(t, os.MkdirAll(path.Join(testTempDir, filePathWithoutParent), fs.ModePerm))
				require.NoError(t, os.WriteFile(path.Join(testTempDir, fileName), []byte(contents), fs.ModePerm))
			}
		}

		installerConfig := &AutodiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      true,
			ProxyPublicAddr:   proxyPublicAddr,
			TeleportPackage:   "teleport-ent",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix: testTempDir,
			imdsProviders: []func(ctx context.Context) (imds.Client, error){
				func(ctx context.Context) (imds.Client, error) {
					return &imds.DisabledClient{}, nil
				},
			},
			binariesLocation:     binariesLocation,
			aptPublicKeyEndpoint: mockRepoKeys.URL,
		}

		teleportInstaller, err := NewAutodiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// One of the first things the install command does is to check if teleport is already installed.
		// If so, it stops the installation with success.
		// Given that we are mocking the binary, it means it already exists and as such, the installation will stop.
		// To prevent that, we must rename the file, call `<pakageManager> install teleport` and rename it back.
		teleportInitialPath := mockBins["teleport"].Path
		teleportHiddenPath := teleportInitialPath + "-hidden"
		require.NoError(t, os.Rename(teleportInitialPath, teleportHiddenPath))

		err = teleportInstaller.Install(ctx)
		require.ErrorContains(t, err, "Auto Discover only runs on Cloud instances with IMDS/Metadata service enabled. Ensure the service is running and try again.")

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
	})
}

// wellKnownOS lists the officially supported repositories for Linux Distros
// (taken from https://goteleport.com/docs/installation/#package-repositories )
// Debian 9, 10, 11, 12
// Ubuntu 16.04 + (only LTS versions are tested)
// Amazon Linux 2 and 2023
// CentOS 7, 8, 9
// RHEL 7, 8, 9
// SLES 12, 15
var wellKnownOS = map[string]map[string]map[string]string{
	"debian": {
		"9":  {etcOSReleaseFile: debian9OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"10": {etcOSReleaseFile: debian10OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"11": {etcOSReleaseFile: debian11OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"12": {etcOSReleaseFile: debian12OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
	},
	"ubuntu": {
		"18.04": {etcOSReleaseFile: ubuntu1804OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"20.04": {etcOSReleaseFile: ubuntu2004OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"22.04": {etcOSReleaseFile: ubuntu2204OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"24.04": {etcOSReleaseFile: ubuntu2404OSRelease, "/usr/share/keyrings/": "", "/etc/apt/sources.list.d/": ""},
	},
	"amzn": {
		"2":    {etcOSReleaseFile: amzn2OSRelease},
		"2023": {etcOSReleaseFile: amazn2023OSRelease},
	},
	"centos": {
		"7": {etcOSReleaseFile: centos7OSRelease},
		"8": {etcOSReleaseFile: centos8OSRelease},
		"9": {etcOSReleaseFile: centos9OSRelease},
	},
	"rhel": {
		"7": {etcOSReleaseFile: rhel7OSRelease},
		"8": {etcOSReleaseFile: rhel8OSRelease},
		"9": {etcOSReleaseFile: rhel9OSRelease},
	},
	"sles": {
		"12": {etcOSReleaseFile: sles12OSRelease},
		"15": {etcOSReleaseFile: sles15OSRelease},
	},
}

const (
	amzn2OSRelease = `NAME="Amazon Linux"
VERSION="2"
ID="amzn"
ID_LIKE="centos rhel fedora"
VERSION_ID="2"
PRETTY_NAME="Amazon Linux 2"
ANSI_COLOR="0;33"
CPE_NAME="cpe:2.3:o:amazon:amazon_linux:2"
HOME_URL="https://amazonlinux.com/"
SUPPORT_END="2025-06-30"`

	amazn2023OSRelease = `NAME="Amazon Linux"
VERSION="2023"
ID="amzn"
ID_LIKE="fedora"
VERSION_ID="2023"
PLATFORM_ID="platform:al2023"
PRETTY_NAME="Amazon Linux 2023.4.20240528"
ANSI_COLOR="0;33"
CPE_NAME="cpe:2.3:o:amazon:amazon_linux:2023"
HOME_URL="https://aws.amazon.com/linux/amazon-linux-2023/"
DOCUMENTATION_URL="https://docs.aws.amazon.com/linux/"
SUPPORT_URL="https://aws.amazon.com/premiumsupport/"
BUG_REPORT_URL="https://github.com/amazonlinux/amazon-linux-2023"
VENDOR_NAME="AWS"
VENDOR_URL="https://aws.amazon.com/"
SUPPORT_END="2028-03-15"`

	centos7OSRelease = `NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"

CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"`

	centos8OSRelease = `NAME="CentOS Linux"
VERSION="8"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="8"
PLATFORM_ID="platform:el8"
PRETTY_NAME="CentOS Linux 8"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:8"
HOME_URL="https://centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"
CENTOS_MANTISBT_PROJECT="CentOS-8"
CENTOS_MANTISBT_PROJECT_VERSION="8"`

	centos9OSRelease = `NAME="CentOS Stream"
VERSION="9"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="9"
PLATFORM_ID="platform:el9"
PRETTY_NAME="CentOS Stream 9"
ANSI_COLOR="0;31"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:centos:centos:9"
HOME_URL="https://centos.org/"
BUG_REPORT_URL="https://issues.redhat.com/"
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux 9"
REDHAT_SUPPORT_PRODUCT_VERSION="CentOS Stream"`

	ubuntu1804OSRelease = `NAME="Ubuntu"
VERSION="18.04.6 LTS (Bionic Beaver)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 18.04.6 LTS"
VERSION_ID="18.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=bionic
UBUNTU_CODENAME=bionic`

	ubuntu2004OSRelease = `NAME="Ubuntu"
VERSION="20.04.6 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.6 LTS"
VERSION_ID="20.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=focal
UBUNTU_CODENAME=focal`

	ubuntu2204OSRelease = `PRETTY_NAME="Ubuntu 22.04 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy`

	ubuntu2404OSRelease = `PRETTY_NAME="Ubuntu 24.04 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
VERSION="24.04 LTS (Noble Numbat)"
VERSION_CODENAME=noble
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=noble
LOGO=ubuntu-logo`

	debian9OSRelease = `PRETTY_NAME="Debian GNU/Linux 9 (stretch)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (stretch)"
VERSION_CODENAME=stretch
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`

	debian10OSRelease = `PRETTY_NAME="Debian GNU/Linux 10 (buster)"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`

	debian11OSRelease = `PRETTY_NAME="Debian GNU/Linux 11 (bullseye)"
NAME="Debian GNU/Linux"
VERSION_ID="11"
VERSION="11 (bullseye)"
VERSION_CODENAME=bullseye
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`

	debian12OSRelease = `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
NAME="Debian GNU/Linux"
VERSION_ID="12"
VERSION="12 (bookworm)"
VERSION_CODENAME=bookworm
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`

	rhel7OSRelease = `NAME="Red Hat Enterprise Linux Server"
VERSION="7.5 (Maipo)"
ID="rhel"
ID_LIKE="fedora"
VARIANT="Server"
VARIANT_ID="server"
VERSION_ID="7.5"
PRETTY_NAME="Red Hat Enterprise Linux Server 7.5 (Maipo)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:redhat:enterprise_linux:7.5:GA:server"
HOME_URL="https://www.redhat.com/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 7"
REDHAT_BUGZILLA_PRODUCT_VERSION=7.5
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="7.5"`

	rhel8OSRelease = `NAME="Red Hat Enterprise Linux"
VERSION="8.10 (Ootpa)"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="8.10"
PLATFORM_ID="platform:el8"
PRETTY_NAME="Red Hat Enterprise Linux 8.10 (Ootpa)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:redhat:enterprise_linux:8::baseos"
HOME_URL="https://www.redhat.com/"
DOCUMENTATION_URL="https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8"
BUG_REPORT_URL="https://bugzilla.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 8"
REDHAT_BUGZILLA_PRODUCT_VERSION=8.10
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="8.10"`

	rhel9OSRelease = `NAME="Red Hat Enterprise Linux"
VERSION="9.4 (Plow)"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="9.4"
PLATFORM_ID="platform:el9"
PRETTY_NAME="Red Hat Enterprise Linux 9.4 (Plow)"
ANSI_COLOR="0;31"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:redhat:enterprise_linux:9::baseos"
HOME_URL="https://www.redhat.com/"
DOCUMENTATION_URL="https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/9"
BUG_REPORT_URL="https://bugzilla.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 9"
REDHAT_BUGZILLA_PRODUCT_VERSION=9.4
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="9.4"`

	sles12OSRelease = `NAME="SLES"
VERSION="12-SP3"
VERSION_ID="12.3"
PRETTY_NAME="SUSE Linux Enterprise Server 12 SP3"
ID="sles"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:suse:sles:12:sp3"`

	sles15OSRelease = `NAME="SLES"
VERSION="12-SP3"
VERSION_ID="12.3"
PRETTY_NAME="SUSE Linux Enterprise Server 12 SP3"
ID="sles"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:suse:sles:12:sp3"`
)
