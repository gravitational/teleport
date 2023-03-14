/*
Copyright 2022 Gravitational, Inc.

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

package config

import (
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

var defaultStoragePath = filepath.Join(defaults.DataDir, "bot")

// StorageConfig contains config parameters for the bot's internal certificate
// storage.
type StorageConfig struct {
	DestinationMixin `yaml:",inline"`
}

// storageDefaults applies default destinations for the bot's internal storage
// section.
func storageDefaults(dm *DestinationMixin) error {
	dm.Directory = &DestinationDirectory{
		Path: defaultStoragePath,
	}

	return nil
}

func (sc *StorageConfig) CheckAndSetDefaults() error {
	if err := sc.DestinationMixin.CheckAndSetDefaults(storageDefaults); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
