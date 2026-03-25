// Copyright 2023 Gravitational, Inc
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

package types

import "github.com/gravitational/trace"

// DatabasePermissions is a list of DatabasePermission objects.
type DatabasePermissions []DatabasePermission

func (m *DatabasePermission) CheckAndSetDefaults() error {
	if len(m.Permissions) == 0 {
		return trace.BadParameter("database permission list cannot be empty")
	}
	for _, permission := range m.Permissions {
		if permission == "" {
			return trace.BadParameter("individual database permissions cannot be empty strings")
		}
	}
	for key, val := range m.Match {
		if key == Wildcard && (len(val) != 1 || val[0] != Wildcard) {
			return trace.BadParameter("database permission: selector *:<val> is not supported")
		}
	}
	return nil
}
