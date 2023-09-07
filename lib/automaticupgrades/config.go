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
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

const (
	// automaticUpgradesEnvar defines the env var to lookup when deciding whether to enable AutomaticUpgrades feature.
	automaticUpgradesEnvar = "TELEPORT_AUTOMATIC_UPGRADES"

	// automaticUpgradesChannelEnvar defines a customer automatic upgrades version release channel.
	automaticUpgradesChannelEnvar = "TELEPORT_AUTOMATIC_UPGRADES_CHANNEL"
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
func GetChannel() string {
	return os.Getenv(automaticUpgradesChannelEnvar)
}
