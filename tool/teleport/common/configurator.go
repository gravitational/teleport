// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"bytes"

	"github.com/gravitational/teleport/lib/config"

	"github.com/gravitational/trace"
)

type installSystemdFlags struct {
	config.SystemdFlags
	// output is the destination to write the systemd unit file to.
	output string
}

// CheckAndSetDefaults checks and sets the defaults
func (flags *installSystemdFlags) CheckAndSetDefaults() error {
	flags.output = normalizeOutput(flags.output)
	return nil
}

// onDumpSystemdUnitFile is the handler of the "install systemd" CLI command.
func onDumpSystemdUnitFile(flags installSystemdFlags) error {
	if err := flags.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	buf := new(bytes.Buffer)
	err := config.WriteSystemdUnitFile(flags.SystemdFlags, buf)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = dumpConfigFile(flags.output, buf.String(), "")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
