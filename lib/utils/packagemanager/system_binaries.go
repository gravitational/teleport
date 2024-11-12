// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package packagemanager

// BinariesLocation contains all the required external binaries used when installing teleport.
// Used for testing.
type BinariesLocation struct {
	Systemctl string

	AptGet string
	AptKey string

	Rpm              string
	Yum              string
	YumConfigManager string

	Zypper string

	// teleport represents the expected location of the teleport binary after installing
	Teleport string
}

// CheckAndSetDefaults fills in the default values for each binary path.
// Default location should be used unless this is part of a test.
func (bi *BinariesLocation) CheckAndSetDefaults() {
	if bi.Systemctl == "" {
		bi.Systemctl = "systemctl"
	}

	if bi.AptGet == "" {
		bi.AptGet = "apt-get"
	}

	if bi.AptKey == "" {
		bi.AptKey = "apt-key"
	}

	if bi.Rpm == "" {
		bi.Rpm = "rpm"
	}

	if bi.Yum == "" {
		bi.Yum = "yum"
	}

	if bi.YumConfigManager == "" {
		bi.YumConfigManager = "yum-config-manager"
	}

	if bi.Zypper == "" {
		bi.Zypper = "zypper"
	}

	if bi.Teleport == "" {
		bi.Teleport = "/usr/local/bin/teleport"
	}
}
