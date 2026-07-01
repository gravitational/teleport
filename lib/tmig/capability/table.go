// Copyright 2025 Gravitational, Inc.
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

package capability

import "strings"

// CapabilitySet describes which roles and methods are scopable for a version.
type CapabilitySet struct {
	Roles   map[string]bool
	Methods []string
}

// table is the versioned local table. Keyed by major.minor.
var table = map[string]CapabilitySet{
	"17.3": {
		Roles:   map[string]bool{"Node": true, "Kube": true, "Bot": true, "App": false, "Db": false},
		Methods: []string{"token", "iam", "ec2", "gcp", "azure", "azure_devops", "oracle", "kubernetes", "bound_keypair"},
	},
	"18.0": {
		Roles:   map[string]bool{"Node": true, "Kube": true, "Bot": true, "App": false, "Db": false},
		Methods: []string{"token", "iam", "ec2", "gcp", "azure", "azure_devops", "oracle", "kubernetes", "bound_keypair"},
	},
	"19.0": {
		Roles:   map[string]bool{"Node": true, "Kube": true, "Bot": true, "App": false, "Db": false},
		Methods: []string{"token", "iam", "ec2", "gcp", "azure", "azure_devops", "oracle", "kubernetes", "bound_keypair"},
	},
}

// Lookup returns the capability set for a version, using major.minor matching.
func Lookup(version string) (*CapabilitySet, bool) {
	key := majorMinor(version)
	cap, ok := table[key]
	if !ok {
		return nil, false
	}
	return &cap, true
}

// NeedsDriftProbe returns true if the version is not in the local table.
func NeedsDriftProbe(version string) bool {
	_, ok := table[majorMinor(version)]
	return !ok
}

func majorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
