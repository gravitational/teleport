/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package automaticupgrades

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	// EnvUpgrader environment variable specifies the external upgrader type
	// Ex: unit, kube
	EnvUpgrader = "TELEPORT_EXT_UPGRADER"
	// EnvUpgraderVersion environment variable specifies the external upgrader version
	// Ex: v14.3.6
	EnvUpgraderVersion = "TELEPORT_EXT_UPGRADER_VERSION"

	// automaticUpgradesEnvar defines the env var to lookup when deciding whether to enable AutomaticUpgrades feature.
	automaticUpgradesEnvar = "TELEPORT_AUTOMATIC_UPGRADES"

	// automaticUpgradesChannelEnvar defines a customer automatic upgrades version release channel.
	automaticUpgradesChannelEnvar = "TELEPORT_AUTOMATIC_UPGRADES_CHANNEL"

	// teleportUpgradeScript defines the default teleport-upgrade script path
	teleportUpgradeScript = "/usr/sbin/teleport-upgrade"
)

// IsEnabled reads the TELEPORT_AUTOMATIC_UPGRADES and returns whether Automatic Upgrades are enabled or disabled.
// An error is logged (warning) if the variable is present but its value could not be converted into a boolean.
// Acceptable values for TELEPORT_AUTOMATIC_UPGRADES are:
// 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False
func IsEnabled() bool {
	autoUpgradesEnv := os.Getenv(automaticUpgradesEnvar)
	if autoUpgradesEnv == "" {
		return false
	}

	automaticUpgrades, err := strconv.ParseBool(autoUpgradesEnv)
	if err != nil {
		log.Warnf("unexpected value for ENV:%s: %v", automaticUpgradesEnvar, err)
		return false
	}

	return automaticUpgrades
}

// GetChannel returns the TELEPORT_AUTOMATIC_UPGRADES_CHANNEL value.
// Example of an acceptable value for TELEPORT_AUTOMATIC_UPGRADES_CHANNEL is:
// https://updates.releases.teleport.dev/v1/stable/cloud
func GetChannel() string {
	return os.Getenv(automaticUpgradesChannelEnvar)
}

// GetUpgraderVersion returns the teleport upgrader version
func GetUpgraderVersion(ctx context.Context, kind string) string {
	if kind == "" {
		return ""
	}
	if kind == "unit" {
		out, err := exec.CommandContext(ctx, teleportUpgradeScript, "version").Output()
		if err != nil {
			log.WithError(err).Debug("Failed to exec /usr/sbin/teleport-upgrade version command.")
		} else {
			if version := strings.TrimSpace(string(out)); version != "" {
				return version
			}
		}
	}
	return os.Getenv(EnvUpgraderVersion)
}

// GetUpgraderKind returns the upgrader kind.
// If the environment value is 'unit' the upgrade script needs to be verified.
func GetUpgraderKind() string {
	kind := os.Getenv(EnvUpgrader)
	if kind == "unit" {
		// Verify the environment variable is still valid
		if _, err := os.Stat(teleportUpgradeScript); err != nil {
			log.WithError(err).Debugf("Failed to verify %s.", teleportUpgradeScript)
			return ""
		}
	}
	return kind
}
