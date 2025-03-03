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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/imds"
	awsimds "github.com/gravitational/teleport/lib/cloud/imds/aws"
	azureimds "github.com/gravitational/teleport/lib/cloud/imds/azure"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	oracleimds "github.com/gravitational/teleport/lib/cloud/imds/oracle"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/linux"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/packagemanager"
)

const (
	discoverNotice = "" +
		"Teleport Discover has successfully configured /etc/teleport.yaml for this server to join the cluster.\n" +
		"Discover might replace the configuration if the server fails to join the cluster.\n" +
		"Please remove this file if you are managing this instance using another tool or doing so manually.\n"
)

// AutoDiscoverNodeInstallerConfig installs and configures a Teleport Server into the current system.
type AutoDiscoverNodeInstallerConfig struct {
	Logger *slog.Logger

	// ProxyPublicAddr is the proxy public address that the instance will connect to.
	// Eg, example.platform.sh
	ProxyPublicAddr string

	// TeleportPackage contains the teleport package name.
	// Allowed values: teleport, teleport-ent, teleport-ent-fips
	TeleportPackage string

	// RepositoryChannel is the repository channel to use.
	// Eg stable/cloud or stable/rolling
	RepositoryChannel string

	// AutoUpgrades indicates whether the installed binaries should auto upgrade.
	// System must support systemd to enable AutoUpgrades.
	AutoUpgrades           bool
	autoUpgradesChannelURL string

	// AzureClientID is the client ID of the managed identity to use when joining
	// the cluster. Only applicable for the azure join method.
	AzureClientID string

	// TokenName is the token name to be used by the instance to join the cluster.
	TokenName string

	// defaultVersion is the version used to compute whether the production or development repositories should be used.
	// If auto upgrades are enabled, then the defaultVersion is ignored.
	// Defaults to api.SemVersion.
	defaultVersion *semver.Version

	// aptRepoKeyEndpointOverride contains the URL for the APT public key.
	// Used for testing.
	aptRepoKeyEndpointOverride string

	// fsRootPrefix is the prefix to use when reading operating system information and when installing teleport.
	// Used for testing.
	fsRootPrefix string

	// binariesLocation contain the location of each required binary.
	// Used for testing.
	binariesLocation packagemanager.BinariesLocation

	// imdsProviders contains the Cloud Instance Metadata providers.
	// Used for testing.
	imdsProviders []func(ctx context.Context) (imds.Client, error)
}

func (c *AutoDiscoverNodeInstallerConfig) checkAndSetDefaults() error {
	if c == nil {
		return trace.BadParameter("install teleport config is required")
	}

	if c.defaultVersion == nil {
		c.defaultVersion = api.SemVersion
	}

	if c.fsRootPrefix == "" {
		c.fsRootPrefix = "/"
	}

	if c.ProxyPublicAddr == "" {
		return trace.BadParameter("proxy public addr is required")
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	if c.RepositoryChannel == "" {
		return trace.BadParameter("repository channel is required")
	}

	if !slices.Contains(types.PackageNameKinds, c.TeleportPackage) {
		return trace.BadParameter("teleport-package must be one of %+v", types.PackageNameKinds)
	}

	if c.AutoUpgrades && c.TeleportPackage == types.PackageNameOSS {
		return trace.BadParameter("only enterprise package supports auto upgrades")
	}

	if c.AutoUpgrades && c.TeleportPackage == types.PackageNameEntFIPS {
		return trace.BadParameter("auto upgrades are not supported in FIPS environments")
	}

	if c.autoUpgradesChannelURL == "" {
		c.autoUpgradesChannelURL = "https://" + c.ProxyPublicAddr + "/v1/webapi/automaticupgrades/channel/default"
	}

	c.binariesLocation.CheckAndSetDefaults()

	if len(c.imdsProviders) == 0 {
		c.imdsProviders = []func(ctx context.Context) (imds.Client, error){
			func(ctx context.Context) (imds.Client, error) {
				clt, err := awsimds.NewInstanceMetadataClient(ctx)
				return clt, trace.Wrap(err)
			},
			func(ctx context.Context) (imds.Client, error) {
				return azureimds.NewInstanceMetadataClient(), nil
			},
			func(ctx context.Context) (imds.Client, error) {
				instancesClient, err := gcp.NewInstancesClient(ctx)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				clt, err := gcpimds.NewInstanceMetadataClient(instancesClient)
				return clt, trace.Wrap(err)
			},
			func(ctx context.Context) (imds.Client, error) {
				return oracleimds.NewInstanceMetadataClient(), nil
			},
		}
	}

	return nil
}

// AutoDiscoverNodeInstaller will install teleport in the current system.
// It's meant to be used by the Server Auto Discover script.
type AutoDiscoverNodeInstaller struct {
	*AutoDiscoverNodeInstallerConfig
}

// NewAutoDiscoverNodeInstaller returns a new AutoDiscoverNodeInstaller.
func NewAutoDiscoverNodeInstaller(cfg *AutoDiscoverNodeInstallerConfig) (*AutoDiscoverNodeInstaller, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ti := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: cfg,
	}

	return ti, nil
}

const (
	// exclusiveInstallFileLock is the name of the lockfile to be used when installing teleport.
	// Used for the default installers (see api/types/installers/{agentless,}installer.sh.tmpl/).
	exclusiveInstallFileLock = "/var/lock/teleport_install.lock"

	// etcOSReleaseFile is the location of the OS Release information.
	// This is valid for most linux distros, that rely on systemd.
	etcOSReleaseFile = "/etc/os-release"

	// teleportYamlConfigNewExtension is the extension used to indicate that this is a new target teleport.yaml version
	teleportYamlConfigNewExtension = ".new"
)

var imdsClientTypeToJoinMethod = map[types.InstanceMetadataType]types.JoinMethod{
	types.InstanceMetadataTypeAzure: types.JoinMethodAzure,
	types.InstanceMetadataTypeEC2:   types.JoinMethodIAM,
	types.InstanceMetadataTypeGCP:   types.JoinMethodGCP,
}

// Install teleport in the current system.
func (ani *AutoDiscoverNodeInstaller) Install(ctx context.Context) error {
	// Ensure only one installer is running by locking the same file as the script installers.
	lockFile := ani.buildAbsoluteFilePath(exclusiveInstallFileLock)
	unlockFn, err := utils.FSTryWriteLock(lockFile)
	if err != nil {
		return trace.BadParameter("Could not get lock %s. Either remove it or wait for the other installer to finish.", lockFile)
	}
	defer func() {
		if err := unlockFn(); err != nil {
			ani.Logger.WarnContext(ctx, "Failed to remove lock. Please remove it manually.", "file", lockFile)
		}
	}()

	imdsClient, err := ani.getIMDSClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	ani.Logger.InfoContext(ctx, "Detected cloud provider", "cloud", imdsClient.GetType())

	// Check if teleport is already installed and install it, if it's absent.
	// In the new autoupdate install flow, teleport-update should have already
	// taken care of installing teleport.
	if _, err := os.Stat(ani.binariesLocation.Teleport); err != nil {
		ani.Logger.InfoContext(ctx, "Installing teleport")
		if err := ani.installTeleportFromRepo(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := ani.configureTeleportNode(ctx, imdsClient); err != nil {
		if trace.IsAlreadyExists(err) {
			ani.Logger.InfoContext(ctx, "Configuration at /etc/teleport.yaml already exists and has the same values, skipping teleport.service restart")
			// Restarting teleport is not required because the target teleport.yaml
			// is up to date with the existing one.
			return nil
		}

		return trace.Wrap(err)
	}
	ani.Logger.InfoContext(ctx, "Configuration written at /etc/teleport.yaml")

	ani.Logger.InfoContext(ctx, "Enabling and starting teleport.service")
	if err := ani.enableAndRestartTeleportService(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// enableAndRestartTeleportService will enable and (re)start the teleport.service.
// This function must be idempotent because we can call it in either one of the following scenarios:
// - teleport was just installed and teleport.service is inactive
// - teleport was already installed but the service is failing
func (ani *AutoDiscoverNodeInstaller) enableAndRestartTeleportService(ctx context.Context) error {
	systemctlEnableNowCMD := exec.CommandContext(ctx, ani.binariesLocation.Systemctl, "enable", "teleport")
	systemctlEnableNowCMDOutput, err := systemctlEnableNowCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(systemctlEnableNowCMDOutput))
	}

	systemctlRestartCMD := exec.CommandContext(ctx, ani.binariesLocation.Systemctl, "restart", "teleport")
	systemctlRestartCMDOutput, err := systemctlRestartCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(systemctlRestartCMDOutput))
	}

	return nil
}

func (ani *AutoDiscoverNodeInstaller) configureTeleportNode(ctx context.Context, imdsClient imds.Client) error {
	nodeLabels, err := fetchNodeAutoDiscoverLabels(ctx, imdsClient)
	if err != nil {
		return trace.Wrap(err)
	}

	// The last step is to configure the `teleport.yaml`.
	// We could do this using the github.com/gravitational/teleport/lib/config package.
	// However, that would cause the configuration to use the current running binary which is different from the binary that was just installed.
	// That could cause problems if the versions are not compatible.
	// To prevent creating an invalid configuration, the installed binary must be used.

	labelEntries := make([]string, 0, len(nodeLabels))
	for labelKey, labelValue := range nodeLabels {
		labelEntries = append(labelEntries, labelKey+"="+labelValue)
	}
	sort.Strings(labelEntries)
	nodeLabelsCommaSeperated := strings.Join(labelEntries, ",")

	joinMethod, ok := imdsClientTypeToJoinMethod[imdsClient.GetType()]
	if !ok {
		return trace.BadParameter("Unsupported cloud provider: %v", imdsClient.GetType())
	}

	teleportYamlConfigurationPath := ani.buildAbsoluteFilePath(defaults.ConfigFilePath)
	teleportYamlConfigurationPathNew := teleportYamlConfigurationPath + teleportYamlConfigNewExtension
	teleportNodeConfigureArgs := []string{"node", "configure", "--output=file://" + teleportYamlConfigurationPathNew,
		fmt.Sprintf(`--proxy=%s`, shsprintf.EscapeDefaultContext(ani.ProxyPublicAddr)),
		fmt.Sprintf(`--join-method=%s`, shsprintf.EscapeDefaultContext(string(joinMethod))),
		fmt.Sprintf(`--token=%s`, shsprintf.EscapeDefaultContext(ani.TokenName)),
		fmt.Sprintf(`--labels=%s`, shsprintf.EscapeDefaultContext(nodeLabelsCommaSeperated)),
	}
	if ani.AzureClientID != "" {
		teleportNodeConfigureArgs = append(teleportNodeConfigureArgs,
			fmt.Sprintf(`--azure-client-id=%s`, shsprintf.EscapeDefaultContext(ani.AzureClientID)))
	}

	ani.Logger.InfoContext(ctx, "Generating teleport configuration", "teleport", ani.binariesLocation.Teleport, "args", teleportNodeConfigureArgs)
	teleportNodeConfigureCmd := exec.CommandContext(ctx, ani.binariesLocation.Teleport, teleportNodeConfigureArgs...)
	teleportNodeConfigureCmdOutput, err := teleportNodeConfigureCmd.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(teleportNodeConfigureCmdOutput))
	}

	defer func() {
		// If an error occurs before the os.Rename, let's remove the `.new` file to prevent any leftovers.
		// Error is ignored because the file might be already removed.
		_ = os.Remove(teleportYamlConfigurationPathNew)
	}()

	discoverNoticeFile := teleportYamlConfigurationPath + ".discover"
	// Check if file already exists and has the same content that we are about to write
	if _, err := os.Stat(teleportYamlConfigurationPath); err == nil {
		hashExistingFile, err := checksum(teleportYamlConfigurationPath)
		if err != nil {
			return trace.Wrap(err)
		}

		hashNewFile, err := checksum(teleportYamlConfigurationPathNew)
		if err != nil {
			return trace.Wrap(err)
		}

		if hashExistingFile == hashNewFile {
			if err := os.WriteFile(discoverNoticeFile, []byte(discoverNotice), 0o644); err != nil {
				return trace.Wrap(err)
			}

			return trace.AlreadyExists("teleport.yaml is up to date")
		}

		// If a previous /etc/teleport.yaml configuration file exists and is different from the target one, it might be because one of the following reasons:
		// - discover installation params (eg token name) were changed
		// - `$ teleport node configure` command produces a different output
		// - teleport was manually installed / configured
		//
		// For the first two scenarios, it's fine, and even desired in most cases, to restart teleport with the new configuration.
		//
		// However, for the last scenario (teleport was manually installed), this flow must not replace the currently running teleport service configuration.
		// To prevent this, this flow checks for the existence of the discover notice file, and only allows replacement if it does exist.
		if _, err := os.Stat(discoverNoticeFile); err != nil {
			ani.Logger.InfoContext(ctx, "Refusing to replace the existing teleport configuration. For the script to replace the existing configuration remove the Teleport configuration.",
				"teleport_configuration", teleportYamlConfigurationPath,
				"discover_notice_file", discoverNoticeFile)
			return trace.BadParameter("missing discover notice file")
		}
	}

	if err := os.Rename(teleportYamlConfigurationPathNew, teleportYamlConfigurationPath); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(discoverNoticeFile, []byte(discoverNotice), 0o644); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func checksum(filename string) (string, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", trace.Wrap(err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (ani *AutoDiscoverNodeInstaller) installTeleportFromRepo(ctx context.Context) error {
	// Read current system information.
	linuxInfo, err := ani.linuxDistribution()
	if err != nil {
		return trace.Wrap(err)
	}

	ani.Logger.InfoContext(ctx, "Operating system detected.",
		"id", linuxInfo.ID,
		"id_like", linuxInfo.IDLike,
		"codename", linuxInfo.VersionCodename,
		"version_id", linuxInfo.VersionID,
	)

	packageManager, err := packagemanager.PackageManagerForSystem(linuxInfo, ani.fsRootPrefix, ani.binariesLocation, ani.aptRepoKeyEndpointOverride)
	if err != nil {
		return trace.Wrap(err)
	}

	targetVersion := ""
	var packagesToInstall []packagemanager.PackageVersion
	if ani.AutoUpgrades {
		teleportAutoUpdaterPackage := ani.TeleportPackage + "-updater"

		ani.Logger.InfoContext(ctx, "Auto-upgrade enabled: fetching target version", "auto_upgrade_endpoint", ani.autoUpgradesChannelURL)
		targetVersion = ani.fetchTargetVersion(ctx)

		// No target version advertised.
		if targetVersion == constants.NoVersion {
			targetVersion = ""
		}
		ani.Logger.InfoContext(ctx, "Using teleport version", "version", targetVersion)
		packagesToInstall = append(packagesToInstall, packagemanager.PackageVersion{Name: teleportAutoUpdaterPackage, Version: targetVersion})
	}
	packagesToInstall = append(packagesToInstall, packagemanager.PackageVersion{Name: ani.TeleportPackage, Version: targetVersion})

	productionRepo := useProductionRepo(packagesToInstall, ani.defaultVersion)
	if err := packageManager.AddTeleportRepository(ctx, linuxInfo, ani.RepositoryChannel, productionRepo); err != nil {
		return trace.BadParameter("failed to add teleport repository to system: %v", err)
	}
	if err := packageManager.InstallPackages(ctx, packagesToInstall); err != nil {
		return trace.BadParameter("failed to install teleport: %v", err)
	}

	return nil
}

// useProductionRepo returns whether this is a production installation.
// In case of an error, it returns true to default to ensure only production binaries are available.
func useProductionRepo(packagesToInstall []packagemanager.PackageVersion, defaultVersion *semver.Version) bool {
	for _, p := range packagesToInstall {
		if p.Version == "" {
			continue
		}

		ver, err := semver.NewVersion(p.Version)
		if err != nil {
			return true
		}

		return ver.PreRelease == ""
	}

	return defaultVersion.PreRelease == ""
}

func (ani *AutoDiscoverNodeInstaller) getIMDSClient(ctx context.Context) (imds.Client, error) {
	// detect and fetch cloud provider metadata
	imdsClient, err := cloud.DiscoverInstanceMetadata(ctx, ani.imdsProviders)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("Auto Discover only runs on Cloud instances with IMDS/Metadata service enabled. Ensure the service is running and try again.")
		}
		return nil, trace.Wrap(err)
	}

	return imdsClient, nil
}

func (ani *AutoDiscoverNodeInstaller) fetchTargetVersion(ctx context.Context) string {
	upgradeURL, err := url.Parse(ani.autoUpgradesChannelURL)
	if err != nil {
		ani.Logger.WarnContext(ctx, "Failed to parse automatic upgrades default channel url, using api version",
			"channel_url", ani.autoUpgradesChannelURL,
			"error", err,
			"version", api.Version)
		return api.Version
	}

	targetVersion, err := version.NewBasicHTTPVersionGetter(upgradeURL).GetVersion(ctx)
	if err != nil {
		ani.Logger.WarnContext(ctx, "Failed to query target version, using api version",
			"error", err,
			"version", api.Version)
		return api.Version
	}
	ani.Logger.InfoContext(ctx, "Found target version",
		"channel_url", ani.autoUpgradesChannelURL,
		"version", targetVersion)

	return strings.TrimSpace(strings.TrimPrefix(targetVersion, "v"))
}

func fetchNodeAutoDiscoverLabels(ctx context.Context, imdsClient imds.Client) (map[string]string, error) {
	nodeLabels := make(map[string]string)

	switch imdsClient.GetType() {
	case types.InstanceMetadataTypeAzure:
		azureIMDSClient, ok := imdsClient.(*azureimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain azure imds client")
		}

		instanceInfo, err := azureIMDSClient.GetInstanceInfo(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.SubscriptionIDLabel] = instanceInfo.SubscriptionID
		nodeLabels[types.VMIDLabel] = instanceInfo.VMID
		nodeLabels[types.RegionLabel] = instanceInfo.Location
		nodeLabels[types.ResourceGroupLabel] = instanceInfo.ResourceGroupName

	case types.InstanceMetadataTypeEC2:
		awsIMDSClient, ok := imdsClient.(*awsimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain ec2 imds client")
		}
		accountID, err := awsIMDSClient.GetAccountID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		instanceID, err := awsIMDSClient.GetID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.AWSInstanceIDLabel] = instanceID
		nodeLabels[types.AWSAccountIDLabel] = accountID

	case types.InstanceMetadataTypeGCP:
		gcpIMDSClient, ok := imdsClient.(*gcpimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain gcp imds client")
		}

		name, err := gcpIMDSClient.GetName(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		zone, err := gcpIMDSClient.GetZone(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		projectID, err := gcpIMDSClient.GetProjectID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.NameLabelDiscovery] = name
		nodeLabels[types.ZoneLabelDiscovery] = zone
		nodeLabels[types.ProjectIDLabelDiscovery] = projectID
		nodeLabels[types.ProjectIDLabel] = projectID

	default:
		return nil, trace.BadParameter("Unsupported cloud provider: %v", imdsClient.GetType())
	}

	return nodeLabels, nil
}

// buildAbsoluteFilePath creates the absolute file path
func (ani *AutoDiscoverNodeInstaller) buildAbsoluteFilePath(path string) string {
	return filepath.Join(ani.fsRootPrefix, path)
}

// linuxDistribution reads the current file system to detect the Linux Distro and Version of the current system.
//
// https://www.freedesktop.org/software/systemd/man/latest/os-release.html
func (ani *AutoDiscoverNodeInstaller) linuxDistribution() (*linux.OSRelease, error) {
	f, err := os.Open(ani.buildAbsoluteFilePath(etcOSReleaseFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	osRelease, err := linux.ParseOSReleaseFromReader(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return osRelease, nil
}
