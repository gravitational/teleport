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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

// DestinationMixin is a reusable struct for all config elements that accept a
// destination. Note that if embedded, DestinationMixin.CheckAndSetDefaults()
// must be called.
type DestinationMixin struct {
	Directory *DestinationDirectory `yaml:"directory,omitempty"`
	Memory    *DestinationMemory    `yaml:"memory,omitempty"`
}

type DestinationDefaults = func(*DestinationMixin) error

// checkAndSetDefaultsInner performs member initialization that won't recurse
func (dm *DestinationMixin) checkAndSetDefaultsInner() (int, error) {
	notNilCount := 0
	if dm.Directory != nil {
		if err := dm.Directory.CheckAndSetDefaults(); err != nil {
			return 0, trace.Wrap(err)
		}

		notNilCount++
	}

	if dm.Memory != nil {
		if err := dm.Memory.CheckAndSetDefaults(); err != nil {
			return 0, trace.Wrap(err)
		}

		notNilCount++
	}
	return notNilCount, nil
}

func (dm *DestinationMixin) CheckAndSetDefaults(applyDefaults DestinationDefaults) error {
	notNilCount, err := dm.checkAndSetDefaultsInner()
	if err != nil {
		return trace.Wrap(err)
	}

	if notNilCount == 0 {
		// use defaults
		if err := applyDefaults(dm); err != nil {
			return trace.Wrap(err)
		}

		// CheckAndSetDefaults() again
		notNilCount, err := dm.checkAndSetDefaultsInner()
		if err != nil {
			return trace.Wrap(err)
		}

		if notNilCount == 0 {
			return trace.BadParameter("a destination is required")
		}
	} else if notNilCount > 1 {
		return trace.BadParameter("only one destination backend may be specified at a time")
	}

	return nil
}

// GetDestination returns the first non-nil Destination set. Note that
// CheckAndSetDefaults() does attempt to ensure that only a single
// destination is set, though this may change at runtime.
func (dm *DestinationMixin) GetDestination() (bot.Destination, error) {
	if dm.Directory != nil {
		return dm.Directory, nil
	}

	if dm.Memory != nil {
		return dm.Memory, nil
	}

	return nil, trace.BadParameter("no valid destination exists")
}
