package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// AccessListWithPresetRequest is a UI representation for creating or updating
// an access list with a preset configuration. Presets automatically configure roles
// and grants for common access list patterns such as long-term access or short-term
// access requests.
//
// When updating an existing access list that was created with a preset, set
// AccessList.Metadata.Name to the existing access list ID and AccessList.Metadata.Revisio
// to the current revision. The revision field enables Compare-And-Swap semantics for
// optimistic locking. If the revision matches the stored revision, the update succeeds.
// If the revision does not match due to concurrent modification, the update fails to
// prevent lost updates.
//
// The preset type cannot be changed after creation.
type AccessListWithPresetRequest struct {
	// PresetType specifies the type of preset configuration to apply.
	// Valid values are "long-term" or "short-term".
	//   - "long-term": Members receive direct access to resources through assigned roles
	//   - "short-term": Members can request temporary access to resources
	PresetType string `json:"presetType,omitempty"`
	// AccessList contains the full access list configuration.
	// During creation the accesslist.meta can be empty
	// where during update operation FE needs to set the correcter acceslist.meta.revision
	AccessList *AccessList `json:"accessList,omitempty"`
	// AccessRoles defines the roles that control access to resources.
	// Teleport manages the full lifecycle of these roles (create, update, delete).
	AccessRoles []*types.RoleV6 `json:"accessRoles,omitempty"`
}

// AccessListWithPresetResponse is the response returned after successfully
// creating or updating an access list with a preset configuration.
// It includes the access list, all members, and the roles that were created or updated.
type AccessListWithPresetResponse struct {
	// AccessList is the created or updated access list with all fields populated,
	// including grants that were automatically configured by the preset.
	AccessList *AccessList `json:"accessList,omitempty"`
	// AccessRoles contains all the roles that provide direct access to resources.
	// These roles are managed by Teleport and should not be modified directly.
	// The number and content of roles matches what was specified in the request.
	AccessRoles []*types.RoleV6 `json:"accessRoles,omitempty"`
	// RolesToBeDeleted contains role names that are no longer valid for this access list preset.
	// These roles may still be in use, so instead of deleting them in the backend and relying on
	// "roles in use" error handling, deletion is deferred to the UI. The user must confirm before
	// the frontend triggers the actual role deletion flow.
	RolesToBeDeleted []string `json:"rolesToBeDeleted,omitempty"`
}

// DeleteAccessListWithPresetResponse is a response returned the preset access role was deleted.
type DeleteAccessListWithPresetResponse struct {
	// Roles contains information about related roles used in access list allowing
	// to defer role deletion to the end user.
	Roles []string `json:"roles,omitempty"`
}

// CheckAndSetDefaults validates and sets default values.
func (u *AccessListWithPresetRequest) CheckAndSetDefaults() error {
	if u.AccessList == nil {
		return trace.BadParameter("accessList is required")
	}
	if u.PresetType == "" {
		return trace.BadParameter("presetType is required")
	}
	if u.PresetType != "long-term" && u.PresetType != "short-term" {
		return trace.BadParameter("presetType must be either 'long-term' or 'short-term', got %q", u.PresetType)
	}
	if u.AccessList.GetName() == "" {
		return trace.BadParameter("access list name is required")
	}
	for _, m := range u.AccessList.Members {
		m.AccessList = u.AccessList.GetName()
	}
	return nil
}
