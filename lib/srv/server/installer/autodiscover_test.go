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
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildkite/bintest/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/utils/packagemanager"
)

func buildMockBins(t *testing.T) (map[string]*bintest.Mock, packagemanager.BinariesLocation, []func() error) {
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

	return mapMockBins, packagemanager.BinariesLocation{
		Systemctl:        mapMockBins["systemctl"].Path,
		AptGet:           mapMockBins["apt-get"].Path,
		AptKey:           mapMockBins["apt-key"].Path,
		Rpm:              mapMockBins["rpm"].Path,
		Yum:              mapMockBins["yum"].Path,
		YumConfigManager: mapMockBins["yum-config-manager"].Path,
		Zypper:           mapMockBins["zypper"].Path,
		Teleport:         mapMockBins["teleport"].Path,
	}, releaseMockFNs
}

func setupDirsForTest(t *testing.T, testTempDir string, distroConfig map[string]string) {
	require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, "etc"), fs.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, "usr/local/bin"), fs.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, "usr/share"), fs.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, "var/lock"), fs.ModePerm))

	for fileName, contents := range distroConfig {
		isDir := strings.HasSuffix(fileName, "/")
		if isDir {
			require.Empty(t, contents, "expected no contents for directory %q", fileName)
			require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, fileName), fs.ModePerm))
		} else {
			filePathWithoutParent := filepath.Base(fileName)
			require.NoError(t, os.MkdirAll(filepath.Join(testTempDir, filePathWithoutParent), fs.ModePerm))
			require.NoError(t, os.WriteFile(filepath.Join(testTempDir, fileName), []byte(contents), fs.ModePerm))
		}
	}
}

type mockGCPInstanceGetter struct{}

func (m *mockGCPInstanceGetter) GetInstance(ctx context.Context, req *gcp.InstanceRequest) (*gcp.Instance, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (m *mockGCPInstanceGetter) GetInstanceTags(ctx context.Context, req *gcp.InstanceRequest) (map[string]string, error) {
	return nil, trace.NotImplemented("not implemented")
}

func TestAutoDiscoverNode(t *testing.T) {
	ctx := context.Background()
	productionVersion := &semver.Version{
		Major: 18,
		Minor: 0,
		Patch: 0,
	}
	developmentVersion := &semver.Version{
		Major:      18,
		Minor:      0,
		Patch:      0,
		PreRelease: "alpha-1",
	}

	mockRepo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "gpg" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("my-public-key"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
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

	t.Run("check and set defaults", func(t *testing.T) {
		t.Run("oss package is not allowed with auto upgrades", func(t *testing.T) {
			installerConfig := &AutoDiscoverNodeInstallerConfig{
				RepositoryChannel: "stable/rolling",
				AutoUpgrades:      true,
				ProxyPublicAddr:   "proxy.example.com",
				TeleportPackage:   "teleport",
				TokenName:         "my-token",
				AzureClientID:     "azure-client-id",
			}

			_, err := NewAutoDiscoverNodeInstaller(installerConfig)
			require.Error(t, err)
		})
		t.Run("fips package is allowed", func(t *testing.T) {
			installerConfig := &AutoDiscoverNodeInstallerConfig{
				RepositoryChannel: "stable/rolling",
				AutoUpgrades:      false,
				ProxyPublicAddr:   "proxy.example.com",
				TeleportPackage:   "teleport-ent-fips",
				TokenName:         "my-token",
				AzureClientID:     "azure-client-id",
			}

			_, err := NewAutoDiscoverNodeInstaller(installerConfig)
			require.NoError(t, err)
		})
		t.Run("fips is not allowed with auto upgrades", func(t *testing.T) {
			installerConfig := &AutoDiscoverNodeInstallerConfig{
				RepositoryChannel: "stable/rolling",
				AutoUpgrades:      true,
				ProxyPublicAddr:   "proxy.example.com",
				TeleportPackage:   "teleport-ent-fips",
				TokenName:         "my-token",
				AzureClientID:     "azure-client-id",
			}

			_, err := NewAutoDiscoverNodeInstaller(installerConfig)
			require.Error(t, err)
		})
	})

	t.Run("well known distros", func(t *testing.T) {
		for distroName, distroVersions := range wellKnownOS {
			for distroVersion, distroConfig := range distroVersions {
				t.Run(distroName+":"+distroVersion, func(t *testing.T) {
					testTempDir := t.TempDir()

					// Common folders to all distros
					setupDirsForTest(t, testTempDir, distroConfig)

					installerConfig := &AutoDiscoverNodeInstallerConfig{
						RepositoryChannel: "stable/rolling",
						AutoUpgrades:      false,
						ProxyPublicAddr:   "proxy.example.com",
						TeleportPackage:   "teleport",
						TokenName:         "my-token",
						AzureClientID:     "azure-client-id",

						fsRootPrefix:               testTempDir,
						imdsProviders:              mockIMDSProviders,
						binariesLocation:           binariesLocation,
						aptRepoKeyEndpointOverride: mockRepo.URL,
						defaultVersion:             productionVersion,
					}

					teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
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
					case "amzn", "rhel", "centos", "almalinux", "rocky":
						mockBins["yum"].Expect("install", "-y", "yum-utils")
						mockBins["rpm"].Expect("--eval", bintest.MatchAny())
						mockBins["yum-config-manager"].Expect("--add-repo", bintest.MatchAny())
						mockBins["yum"].Expect("install", "-y", "teleport").AndCallFunc(func(c *bintest.Call) {
							assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
							c.Exit(0)
						})
					case "sles":
						mockBins["rpm"].Expect("--import", "https://zypper.releases.teleport.dev/gpg")
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
						"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
						"--proxy=proxy.example.com",
						"--join-method=azure",
						"--token=my-token",
						"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
						"--azure-client-id=azure-client-id",
					).AndCallFunc(func(c *bintest.Call) {
						// create a teleport.yaml configuration file
						require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
						c.Exit(0)
					})

					mockBins["systemctl"].Expect("enable", "teleport")
					mockBins["systemctl"].Expect("restart", "teleport")

					require.NoError(t, teleportInstaller.Install(ctx))

					for binName, mockBin := range mockBins {
						require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
					}
					require.FileExists(t, testTempDir+"/etc/teleport.yaml")
					require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")

					if distroName == "ubuntu" || distroName == "debian" {
						teleportRepoFile, err := os.ReadFile(testTempDir + "/etc/apt/sources.list.d/teleport.list")
						require.NoError(t, err)
						require.Contains(t, string(teleportRepoFile), "https://apt.releases.teleport.dev/")
					}
				})
			}
		}
	})

	t.Run("with automatic upgrades", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]

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

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      true,
			ProxyPublicAddr:   proxyPublicAddr,
			TeleportPackage:   "teleport-ent",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			autoUpgradesChannelURL:     proxyServer.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
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
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy="+proxyPublicAddr,
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		mockBins["systemctl"].Expect("enable", "teleport")
		mockBins["systemctl"].Expect("restart", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")
		teleportRepoFile, err := os.ReadFile(testTempDir + "/etc/apt/sources.list.d/teleport.list")
		require.NoError(t, err)
		require.Contains(t, string(teleportRepoFile), "https://apt.releases.teleport.dev/ubuntu")
	})

	t.Run("with automatic upgrades using a development version, installs the development repositories", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]

		proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// We only expect calls to the automatic upgrade default channel's version endpoint.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("v15.4.0-alpha-2\n"))
		}))
		t.Cleanup(func() {
			proxyServer.Close()
		})
		proxyPublicAddr := proxyServer.Listener.Addr().String()

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      true,
			ProxyPublicAddr:   proxyPublicAddr,
			TeleportPackage:   "teleport-ent",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			autoUpgradesChannelURL:     proxyServer.URL,
			defaultVersion:             developmentVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// One of the first things the install command does is to check if teleport is already installed.
		// If so, it stops the installation with success.
		// Given that we are mocking the binary, it means it already exists and as such, the installation will stop.
		// To prevent that, we must rename the file, call `<pakageManager> install teleport` and rename it back.
		teleportInitialPath := mockBins["teleport"].Path
		teleportHiddenPath := teleportInitialPath + "-hidden"
		require.NoError(t, os.Rename(teleportInitialPath, teleportHiddenPath))

		mockBins["apt-get"].Expect("update")
		mockBins["apt-get"].Expect("install", "-y", "teleport-ent-updater=15.4.0-alpha-2", "teleport-ent=15.4.0-alpha-2").AndCallFunc(func(c *bintest.Call) {
			assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
			c.Exit(0)
		})

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy="+proxyPublicAddr,
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		mockBins["systemctl"].Expect("enable", "teleport")
		mockBins["systemctl"].Expect("restart", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")

		teleportRepoFile, err := os.ReadFile(testTempDir + "/etc/apt/sources.list.d/teleport.list")
		require.NoError(t, err)
		require.Contains(t, string(teleportRepoFile), "https://apt.releases.development.teleport.dev/ubuntu")
	})

	t.Run("installs the development repositories when the current version is a dev build", func(t *testing.T) {
		testTempDir := t.TempDir()
		distroConfig := wellKnownOS["ubuntu"]["24.04"]
		// Common folders to all distros
		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// One of the first things the install command does is to check if teleport is already installed.
		// If so, it stops the installation with success.
		// Given that we are mocking the binary, it means it already exists and as such, the installation will stop.
		// To prevent that, we must rename the file, call `<pakageManager> install teleport` and rename it back.
		teleportInitialPath := mockBins["teleport"].Path
		teleportHiddenPath := teleportInitialPath + "-hidden"
		require.NoError(t, os.Rename(teleportInitialPath, teleportHiddenPath))

		mockBins["apt-get"].Expect("update")
		mockBins["apt-get"].Expect("install", "-y", "teleport").AndCallFunc(func(c *bintest.Call) {
			assert.NoError(t, os.Rename(teleportHiddenPath, teleportInitialPath))
			c.Exit(0)
		})

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		mockBins["systemctl"].Expect("enable", "teleport")
		mockBins["systemctl"].Expect("restart", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")

		teleportRepoFile, err := os.ReadFile(testTempDir + "/etc/apt/sources.list.d/teleport.list")
		require.NoError(t, err)
		require.Contains(t, string(teleportRepoFile), "https://apt.releases.teleport.dev/")
	})

	t.Run("gcp adds a label with the project id", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		mockIMDSProviders := []func(ctx context.Context) (imds.Client, error){
			func(ctx context.Context) (imds.Client, error) {
				return gcp.NewInstanceMetadataClient(
					&mockGCPInstanceGetter{},
					gcp.WithMetadataClient(func(ctx context.Context, path string) (string, error) {
						switch path {
						case "instance/id":
							return "123", nil
						case "instance/name":
							return "my-name", nil
						case "instance/zone":
							return "my-zone", nil
						case "project/project-id":
							return "my-project", nil
						}
						return "", trace.BadParameter("path %q not found in metadata", path)
					}),
				)
			},
		}

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		// create an existing teleport.yaml configuration file
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("has wrong config"), 0o644))
		// create a teleport.yaml.discover to indicate that this host is controlled by the discover flow
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.discover", []byte(""), 0o644))

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=gcp",
			"--token=my-token",
			"--labels=teleport.dev/project-id=my-project,teleport.internal/name=my-name,teleport.internal/project-id=my-project,teleport.internal/zone=my-zone",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		mockBins["systemctl"].Expect("enable", "teleport")
		mockBins["systemctl"].Expect("restart", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}

		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml.new")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")
		bs, err := os.ReadFile(testTempDir + "/etc/teleport.yaml")
		require.NoError(t, err)
		require.Equal(t, "teleport.yaml configuration bytes", string(bs))
	})

	t.Run("fails when imds server is not available", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]
		proxyPublicAddr := "proxy.example.com"

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
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
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		err = teleportInstaller.Install(ctx)
		require.ErrorContains(t, err, "Auto Discover only runs on Cloud instances with IMDS/Metadata service enabled. Ensure the service is running and try again.")

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}
		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml")
		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml.discover")
	})

	t.Run("reconfigures and restarts if target teleport.yaml is different and discover file exists", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		// create an existing teleport.yaml configuration file
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("has wrong config"), 0o644))
		// create a teleport.yaml.discover to indicate that this host is controlled by the discover flow
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.discover", []byte(""), 0o644))

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		mockBins["systemctl"].Expect("enable", "teleport")
		mockBins["systemctl"].Expect("restart", "teleport")

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}

		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml.new")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml.discover")
		bs, err := os.ReadFile(testTempDir + "/etc/teleport.yaml")
		require.NoError(t, err)
		require.Equal(t, "teleport.yaml configuration bytes", string(bs))
	})

	t.Run("does not reconfigure if teleport.yaml exists but discover file does not exists", func(t *testing.T) {
		distroConfig := wellKnownOS["ubuntu"]["24.04"]

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		// create an existing teleport.yaml configuration file
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("has wrong config"), 0o644))

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		require.ErrorContains(t, teleportInstaller.Install(ctx), "missing discover notice file")

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}

		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml.new")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		bs, err := os.ReadFile(testTempDir + "/etc/teleport.yaml")
		require.NoError(t, err)
		require.Equal(t, "has wrong config", string(bs))
	})

	t.Run("does nothing if teleport is already installed and target teleport.yaml configuration already exists", func(t *testing.T) {
		distroName := "ubuntu"
		distroVersion := "24.04"
		distroConfig := wellKnownOS[distroName][distroVersion]

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		// create an existing teleport.yaml configuration file
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("teleport.yaml configuration bytes"), 0o644))

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		require.NoError(t, teleportInstaller.Install(ctx))

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}

		require.NoFileExists(t, testTempDir+"/etc/teleport.yaml.new")
		require.FileExists(t, testTempDir+"/etc/teleport.yaml")
		bs, err := os.ReadFile(testTempDir + "/etc/teleport.yaml")
		require.NoError(t, err)
		require.Equal(t, "teleport.yaml configuration bytes", string(bs))
	})

	t.Run("does nothing if target teleport.yaml configuration exists but was manually created/edited", func(t *testing.T) {
		distroName := "ubuntu"
		distroVersion := "24.04"
		distroConfig := wellKnownOS[distroName][distroVersion]

		testTempDir := t.TempDir()

		setupDirsForTest(t, testTempDir, distroConfig)

		installerConfig := &AutoDiscoverNodeInstallerConfig{
			RepositoryChannel: "stable/rolling",
			AutoUpgrades:      false,
			ProxyPublicAddr:   "proxy.example.com",
			TeleportPackage:   "teleport",
			TokenName:         "my-token",
			AzureClientID:     "azure-client-id",

			fsRootPrefix:               testTempDir,
			imdsProviders:              mockIMDSProviders,
			binariesLocation:           binariesLocation,
			aptRepoKeyEndpointOverride: mockRepo.URL,
			defaultVersion:             productionVersion,
		}

		teleportInstaller, err := NewAutoDiscoverNodeInstaller(installerConfig)
		require.NoError(t, err)

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		// create an existing teleport.yaml configuration file
		require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("teleport.yaml bytes"), 0o644))

		// package manager is not called in this scenario because teleport binary already exists in the system
		require.FileExists(t, mockBins["teleport"].Path)

		mockBins["teleport"].Expect("node",
			"configure",
			"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
			"--proxy=proxy.example.com",
			"--join-method=azure",
			"--token=my-token",
			"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			"--azure-client-id=azure-client-id",
		).AndCallFunc(func(c *bintest.Call) {
			// create a teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
			c.Exit(0)
		})

		require.ErrorContains(t, teleportInstaller.Install(ctx), "missing discover notice file")

		for binName, mockBin := range mockBins {
			require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
		}

		t.Run("even if it runs multiple times", func(t *testing.T) {

			// create an existing teleport.yaml configuration file
			require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml", []byte("manual configuration already exists"), 0o644))

			// package manager is not called in this scenario because teleport binary already exists in the system
			require.FileExists(t, mockBins["teleport"].Path)

			mockBins["teleport"].Expect("node",
				"configure",
				"--output=file://"+testTempDir+"/etc/teleport.yaml.new",
				"--proxy=proxy.example.com",
				"--join-method=azure",
				"--token=my-token",
				"--labels=teleport.internal/region=eastus,teleport.internal/resource-group=TestGroup,teleport.internal/subscription-id=5187AF11-3581-4AB6-A654-59405CD40C44,teleport.internal/vm-id=ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
				"--azure-client-id=azure-client-id",
			).AndCallFunc(func(c *bintest.Call) {
				// create a teleport.yaml configuration file
				require.NoError(t, os.WriteFile(testTempDir+"/etc/teleport.yaml.new", []byte("teleport.yaml configuration bytes"), 0o644))
				c.Exit(0)
			})

			require.ErrorContains(t, teleportInstaller.Install(ctx), "missing discover notice file")

			for binName, mockBin := range mockBins {
				require.True(t, mockBin.Check(t), "mismatch between expected invocations and actual calls for %q", binName)
			}
		})
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
		"9":  {etcOSReleaseFile: debian9OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"10": {etcOSReleaseFile: debian10OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"11": {etcOSReleaseFile: debian11OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"12": {etcOSReleaseFile: debian12OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
	},
	"ubuntu": {
		"18.04": {etcOSReleaseFile: ubuntu1804OSRelease, "/etc/apt/sources.list.d/": ""}, // No /etc/apt/keyrings/ by default
		"20.04": {etcOSReleaseFile: ubuntu2004OSRelease, "/etc/apt/sources.list.d/": ""}, // No /etc/apt/keyrings/ by default
		"22.04": {etcOSReleaseFile: ubuntu2204OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
		"24.04": {etcOSReleaseFile: ubuntu2404OSRelease, "/etc/apt/keyrings/": "", "/etc/apt/sources.list.d/": ""},
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
	"almalinux": {
		"9": {etcOSReleaseFile: alma9OSRelease},
	},
	"rocky": {
		"9": {etcOSReleaseFile: rocky9OSRelease},
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

	alma9OSRelease = `NAME="AlmaLinux"
VERSION="9.4 (Seafoam Ocelot)"
ID="almalinux"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.4"
PLATFORM_ID="platform:el9"
PRETTY_NAME="AlmaLinux 9.4 (Seafoam Ocelot)"
ANSI_COLOR="0;34"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:almalinux:almalinux:9::baseos"
HOME_URL="https://almalinux.org/"
DOCUMENTATION_URL="https://wiki.almalinux.org/"
BUG_REPORT_URL="https://bugs.almalinux.org/"

ALMALINUX_MANTISBT_PROJECT="AlmaLinux-9"
ALMALINUX_MANTISBT_PROJECT_VERSION="9.4"
REDHAT_SUPPORT_PRODUCT="AlmaLinux"
REDHAT_SUPPORT_PRODUCT_VERSION="9.4"
SUPPORT_END=2032-06-01`

	rocky9OSRelease = `NAME="Rocky Linux"
VERSION="9.3 (Blue Onyx)"
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.3"
PLATFORM_ID="platform:el9"
PRETTY_NAME="Rocky Linux 9.3 (Blue Onyx)"
ANSI_COLOR="0;32"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:rocky:rocky:9::baseos"
HOME_URL="https://rockylinux.org/"
BUG_REPORT_URL="https://bugs.rockylinux.org/"
SUPPORT_END="2032-05-31"
ROCKY_SUPPORT_PRODUCT="Rocky-Linux-9"
ROCKY_SUPPORT_PRODUCT_VERSION="9.3"
REDHAT_SUPPORT_PRODUCT="Rocky Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="9.3"`

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
