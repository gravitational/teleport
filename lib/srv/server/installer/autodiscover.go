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
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"sort"
	"strings"

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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/linux"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	filePermsRepository = 0o644
)

// AutodiscoverNodeInstallerConfig installs and configures a Teleport Server into the current system.
type AutodiscoverNodeInstallerConfig struct {
	Logger *slog.Logger

	// ProxyPublicAddr is the proxy public address that the instance will connect to.
	// Eg, example.platform.sh
	ProxyPublicAddr string

	// TeleportPackage contains the teleport package name.
	// Allowed values: teleport, teleport-ent
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

	// packageManagers contains implementation for each supported package manager.
	packageManagers map[packageManagerKind]packageManager

	// aptPublicKeyEndpoint contains the URL for the APT public key.
	// Defaults to: https://apt.releases.teleport.dev/gpg
	aptPublicKeyEndpoint string

	// fsRootPrefix is the prefix to use when reading operating system information and when installing teleport.
	// Used for testing.
	fsRootPrefix string

	// binariesLocation contain the location of each required binary.
	// Used for testing.
	binariesLocation binariesLocation

	// imdsProviders contains the Cloud Instance Metadata providers.
	// Used for testing.
	imdsProviders []func(ctx context.Context) (imds.Client, error)
}

// binariesLocation contains all the required external binaries used install teleport.
// Used for testing.
type binariesLocation struct {
	systemctl string

	aptGet string
	aptKey string

	rpm              string
	yum              string
	yumConfigManager string

	zypper string

	// teleport represents the expected location of the teleport binary after installing
	teleport string
}

func (bi *binariesLocation) checkAndSetDefaults() {
	if bi.systemctl == "" {
		bi.systemctl = "systemctl"
	}

	if bi.aptGet == "" {
		bi.aptGet = "apt-get"
	}

	if bi.aptKey == "" {
		bi.aptKey = "apt-key"
	}

	if bi.rpm == "" {
		bi.rpm = "rpm"
	}

	if bi.yum == "" {
		bi.yum = "yum"
	}

	if bi.yumConfigManager == "" {
		bi.yumConfigManager = "yum-config-manager"
	}

	if bi.zypper == "" {
		bi.zypper = "zypper"
	}

	if bi.teleport == "" {
		bi.teleport = "/usr/local/bin/teleport"
	}
}

func (c *AutodiscoverNodeInstallerConfig) checkAndSetDefaults() error {
	if c == nil {
		return trace.BadParameter("install teleport config is required")
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

	if c.AutoUpgrades && c.TeleportPackage != types.PackageNameEnt {
		return trace.BadParameter("only enterprise package supports auto upgrades")
	}

	if c.autoUpgradesChannelURL == "" {
		c.autoUpgradesChannelURL = "https://" + c.ProxyPublicAddr + "/v1/webapi/automaticupgrades/channel/default"
	}

	c.binariesLocation.checkAndSetDefaults()

	if c.packageManagers == nil {
		packageManagerAPTLegacy, err := newPackageManagerAPTLegacy(&packageManagerAPTConfig{
			fsRootPrefix:         c.fsRootPrefix,
			bins:                 c.binariesLocation,
			aptPublicKeyEndpoint: c.aptPublicKeyEndpoint,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		packageManagerAPT, err := newPackageManagerAPT(&packageManagerAPTConfig{
			fsRootPrefix:         c.fsRootPrefix,
			bins:                 c.binariesLocation,
			aptPublicKeyEndpoint: c.aptPublicKeyEndpoint,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		packageManagerYUM, err := newPackageManagerYUM(&packageManagerYUMConfig{
			fsRootPrefix: c.fsRootPrefix,
			bins:         c.binariesLocation,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		packageManagerZypper, err := newPackageManagerZypper(&packageManagerZypperConfig{
			fsRootPrefix: c.fsRootPrefix,
			bins:         c.binariesLocation,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		c.packageManagers = map[packageManagerKind]packageManager{
			packageManagerKindAPTLegacy: packageManagerAPTLegacy,
			packageManagerKindAPT:       packageManagerAPT,
			packageManagerKindYUM:       packageManagerYUM,
			packageManagerKindZypper:    packageManagerZypper,
		}
	}

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
		}
	}

	return nil
}

type packageVersion struct {
	Name    string
	Version string
}

type packageManager interface {
	AddTeleportRepository(ctx context.Context, ldi *linuxDistroInfo, repoChannel string) error
	InstallPackages(context.Context, []packageVersion) error
}

// AutodiscoverNodeInstaller will install teleport in the current system.
// It's meant to be used by the Server Auto Discover script.
type AutodiscoverNodeInstaller struct {
	*AutodiscoverNodeInstallerConfig
}

// NewAutodiscoverNodeInstaller returns a new AutodiscoverNodeInstaller.
func NewAutodiscoverNodeInstaller(cfg *AutodiscoverNodeInstallerConfig) (*AutodiscoverNodeInstaller, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ti := &AutodiscoverNodeInstaller{
		AutodiscoverNodeInstallerConfig: cfg,
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
)

var imdsClientTypeToJoinMethod = map[types.InstanceMetadataType]types.JoinMethod{
	types.InstanceMetadataTypeAzure: types.JoinMethodAzure,
	types.InstanceMetadataTypeEC2:   types.JoinMethodIAM,
	types.InstanceMetadataTypeGCP:   types.JoinMethodGCP,
}

// Install teleport in the current system.
func (asi *AutodiscoverNodeInstaller) Install(ctx context.Context) error {
	// Ensure only one installer is running by locking the same file as the script installers.
	lockFile := asi.buildAbsoluteFilePath(exclusiveInstallFileLock)
	unlockFn, err := utils.FSTryWriteLock(lockFile)
	if err != nil {
		return trace.BadParameter("Could not get lock %s. Either remove it or wait for the other installer to finish.", lockFile)
	}
	defer func() {
		if err := unlockFn(); err != nil {
			asi.Logger.WarnContext(ctx, "Failed to remove lock. Please remove it manually.", "file", exclusiveInstallFileLock)
		}
	}()

	// Check if teleport is already installed.
	if _, err := os.Stat(asi.binariesLocation.teleport); err == nil {
		asi.Logger.InfoContext(ctx, "Teleport is already installed in the system.")
		return nil
	}

	teleportYamlConfigurationPath := asi.buildAbsoluteFilePath(defaults.ConfigFilePath)
	// Check is teleport was already configured.
	if _, err := os.Stat(teleportYamlConfigurationPath); err == nil {
		return trace.BadParameter("Teleport configuration already exists at %s. Please remove it manually.", teleportYamlConfigurationPath)
	}

	// Read current system information.
	linuxInfo, err := asi.linuxDistribution()
	if err != nil {
		return trace.Wrap(err)
	}

	asi.Logger.InfoContext(ctx, "Operating system detected.",
		"id", linuxInfo.ID,
		"id_like", linuxInfo.IDLike,
		"codename", linuxInfo.VersionCodename,
		"version_id", linuxInfo.VersionID,
	)

	// Pick up the correct package manager/repository for the system.
	packageManager, ok := asi.packageManagers[linuxInfo.packageManagerKind()]
	if !ok {
		return trace.BadParameter("package manager for %s (%s) is not yet supported", linuxInfo.ID, linuxInfo.IDLike)
	}

	imdsClient, err := asi.getIMDSClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	asi.Logger.InfoContext(ctx, "Detected cloud provider", "cloud", imdsClient.GetType())

	targetVersion := ""
	var packagesToInstall []packageVersion
	if asi.AutoUpgrades {
		teleportAutoUpdaterPackage := asi.TeleportPackage + "-updater"

		asi.Logger.InfoContext(ctx, "Auto-upgrade enabled: fetching target version", "auto_upgrade_endpoint", asi.autoUpgradesChannelURL)
		targetVersion = asi.fetchTargetVersion(ctx)

		// No target version advertised.
		if targetVersion == constants.NoVersion {
			targetVersion = ""
		}
		asi.Logger.InfoContext(ctx, "Using teleport version", "version", targetVersion)
		packagesToInstall = append(packagesToInstall, packageVersion{Name: teleportAutoUpdaterPackage, Version: targetVersion})
	}
	packagesToInstall = append(packagesToInstall, packageVersion{Name: asi.TeleportPackage, Version: targetVersion})

	if err := packageManager.AddTeleportRepository(ctx, linuxInfo, asi.RepositoryChannel); err != nil {
		return trace.BadParameter("failed to add teleport repository to system: %v", err)
	}
	if err := packageManager.InstallPackages(ctx, packagesToInstall); err != nil {
		return trace.BadParameter("failed to install teleport: %v", err)
	}

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

	teleportNodeConfigureArgs := []string{"node", "configure", "--output=file://" + teleportYamlConfigurationPath,
		fmt.Sprintf(`--proxy=%s`, shsprintf.EscapeDefaultContext(asi.ProxyPublicAddr)),
		fmt.Sprintf(`--join-method=%s`, shsprintf.EscapeDefaultContext(string(joinMethod))),
		fmt.Sprintf(`--token=%s`, shsprintf.EscapeDefaultContext(asi.TokenName)),
		fmt.Sprintf(`--labels=%s`, shsprintf.EscapeDefaultContext(nodeLabelsCommaSeperated)),
	}
	if asi.AzureClientID != "" {
		teleportNodeConfigureArgs = append(teleportNodeConfigureArgs,
			fmt.Sprintf(`--azure-client-id=%s`, shsprintf.EscapeDefaultContext(asi.AzureClientID)))
	}

	asi.Logger.InfoContext(ctx, "Writing teleport configuration", "teleport", asi.binariesLocation.teleport, "args", teleportNodeConfigureArgs)
	teleportNodeConfigureCmd := exec.CommandContext(ctx, asi.binariesLocation.teleport, teleportNodeConfigureArgs...)
	teleportNodeConfigureCmdOutput, err := teleportNodeConfigureCmd.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(teleportNodeConfigureCmdOutput))
	}

	asi.Logger.InfoContext(ctx, "Enabling and starting teleport service")
	systemctlEnableNowCMD := exec.CommandContext(ctx, asi.binariesLocation.systemctl, "enable", "--now", "teleport")
	systemctlEnableNowCMDOutput, err := systemctlEnableNowCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(systemctlEnableNowCMDOutput))
	}

	return nil
}

func (asi *AutodiscoverNodeInstaller) getIMDSClient(ctx context.Context) (imds.Client, error) {
	// detect and fetch cloud provider metadata
	imdsClient, err := cloud.DiscoverInstanceMetadata(ctx, asi.imdsProviders)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("Auto Discover only runs on Cloud instances with IMDS/Metadata service enabled. Ensure the service is running and try again.")
		}
		return nil, trace.Wrap(err)
	}

	return imdsClient, nil
}

func (asi *AutodiscoverNodeInstaller) fetchTargetVersion(ctx context.Context) string {
	upgradeURL, err := url.Parse(asi.autoUpgradesChannelURL)
	if err != nil {
		asi.Logger.WarnContext(ctx, "Failed to parse automatic upgrades default channel url, using api version",
			"channel_url", asi.autoUpgradesChannelURL,
			"error", err,
			"version", api.Version)
		return api.Version
	}

	autoUpgradesVersionGetter := version.NewBasicHTTPVersionGetter(upgradeURL)
	targetVersion, err := autoUpgradesVersionGetter.GetVersion(ctx)
	if err != nil {
		asi.Logger.WarnContext(ctx, "Failed to query target version, using api version",
			"error", err,
			"version", api.Version)
		return api.Version
	}
	asi.Logger.InfoContext(ctx, "Found target version",
		"channel_url", asi.autoUpgradesChannelURL,
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

		nodeLabels[types.NameLabel] = name
		nodeLabels[types.ZoneLabel] = zone
		nodeLabels[types.ProjectIDLabel] = projectID

	default:
		return nil, trace.BadParameter("Unsupported cloud provider: %v", imdsClient.GetType())
	}

	return nodeLabels, nil
}

// buildAbsoluteFilePath creates the absolute file path
func (asi *AutodiscoverNodeInstaller) buildAbsoluteFilePath(filepath string) string {
	return path.Join(asi.fsRootPrefix, filepath)
}

type linuxDistroInfo struct {
	*linux.OSRelease
}

func (l *linuxDistroInfo) packageManagerKind() packageManagerKind {
	aptWellKnownIDs := []string{"debian", "ubuntu"}
	legacyAPT := []string{"xenial", "trusty"}

	yumWellKnownIDs := []string{"amzn", "rhel", "centos"}

	zypperWellKnownIDs := []string{"sles", "opensuse-tumbleweed", "opensuse-leap"}

	switch {
	case slices.Contains(aptWellKnownIDs, l.ID):
		if slices.Contains(legacyAPT, l.VersionCodename) {
			return packageManagerKindAPTLegacy
		}
		return packageManagerKindAPT

	case slices.Contains(yumWellKnownIDs, l.ID):
		return packageManagerKindYUM

	case slices.Contains(zypperWellKnownIDs, l.ID):
		return packageManagerKindZypper

	default:
		return packageManagerKindUnknown
	}
}

// linuxDistribution reads the current file system to detect the Linux Distro and Version of the current system.
//
// https://www.freedesktop.org/software/systemd/man/latest/os-release.html
func (asi *AutodiscoverNodeInstaller) linuxDistribution() (*linuxDistroInfo, error) {
	f, err := os.Open(asi.buildAbsoluteFilePath(etcOSReleaseFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	osRelease, err := linux.ParseOSReleaseFromReader(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &linuxDistroInfo{
		OSRelease: osRelease,
	}, nil
}

type packageManagerKind int

const (
	packageManagerKindUnknown = iota
	packageManagerKindAPTLegacy
	packageManagerKindAPT
	packageManagerKindYUM
	packageManagerKindZypper
)
