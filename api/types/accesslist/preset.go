/*
Copyright 2026 Gravitational, Inc.

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

package accesslist

import (
	"strings"

	"github.com/gravitational/teleport/api/types"
)

const (
	// AccessListPresetLabel marks an access list resource (with a label) that
	// is was created using a preset. The label value identifies the preset.
	AccessListPresetLabel = types.TeleportInternalLabelPrefix + "access-list-preset"

	// AccessListPresetRolesLabel is set on a preset-created access list and
	// holds a comma-separated list of the role names created for the list.
	// This is used to track which roles should be deleted when they're removed
	// from the configuration.
	AccessListPresetRolesLabel = types.TeleportInternalLabelPrefix + "access-list-preset-roles"

	// AccessListPresetRoleInfix is the infix used in the names of roles
	// auto-created for a preset-backed access list. The full role name
	// format is "{prefix}-{AccessListPresetRoleInfix}-{accessListUUID}".
	AccessListPresetRoleInfix = "acl-preset"
)

// PresetRoleNames returns the role names recorded on this access list's
// label or nil if the label is unset.
func (a *AccessList) PresetRoleNames() []string {
	rolesStr, ok := a.GetAllLabels()[AccessListPresetRolesLabel]
	if !ok || rolesStr == "" {
		return nil
	}
	return strings.Split(rolesStr, ",")
}
