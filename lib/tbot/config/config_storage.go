/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

import (
	"path/filepath"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
)

var defaultStoragePath = filepath.Join(defaults.DataDir, "bot")

// StorageConfig contains config parameters for the bot's internal certificate
// storage.
type StorageConfig struct {
	// Destination's yaml is handled by MarshalYAML/UnmarshalYAML
	Destination bot.Destination
}

func (sc *StorageConfig) CheckAndSetDefaults() error {
	if sc.Destination == nil {
		sc.Destination = &DestinationDirectory{
			Path: defaultStoragePath,
		}
	}

	return trace.Wrap(sc.Destination.CheckAndSetDefaults(), "validating storage")
}

func (sc *StorageConfig) MarshalYAML() (interface{}, error) {
	// Effectively inlines the destination
	return sc.Destination.MarshalYAML()
}

func (sc *StorageConfig) UnmarshalYAML(node *yaml.Node) error {
	// Effectively inlines the destination
	dest, err := unmarshalDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	sc.Destination = dest
	return nil
}

// GetDefaultStoragePath returns the default internal storage path for tbot.
func GetDefaultStoragePath() string {
	return defaultStoragePath
}
