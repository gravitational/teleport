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

package accesslist

import (
	"encoding/json"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/utils"
)

// AccessList describes the basic building block of access grants, which are
// similar to access requests but for longer lived permissions that need to be
// regularly audited.
type AccessList struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access list.
	Spec Spec `json:"spec" yaml:"spec"`
}

// Spec is the specification for an access list.
type Spec struct {
	// Title is a plaintext short description of the access list.
	Title string `json:"title" yaml:"title"`

	// Description is an optional plaintext description of the access list.
	Description string `json:"description" yaml:"description"`

	// Owners is a list of owners of the access list.
	Owners []Owner `json:"owners" yaml:"owners"`

	// Audit describes the frequency that this access list must be audited.
	Audit Audit `json:"audit" yaml:"audit"`

	// MembershipRequires describes the requirements for a user to be a member of the access list.
	// For a membership to an access list to be effective, the user must meet the requirements of
	// MembershipRequires and must be in the members list.
	MembershipRequires Requires `json:"membership_requires" yaml:"membership_requires"`

	// OwnershipRequires describes the requirements for a user to be an owner of the access list.
	// For ownership of an access list to be effective, the user must meet the requirements of
	// OwnershipRequires and must be in the owners list.
	OwnershipRequires Requires `json:"ownership_requires" yaml:"ownership_requires"`

	// Grants describes the access granted by membership to this access list.
	Grants Grants `json:"grants" yaml:"grants"`
}

// Owner is an owner of an access list.
type Owner struct {
	// Name is the username of the owner.
	Name string `json:"name" yaml:"name"`

	// Description is the plaintext description of the owner and why they are an owner.
	Description string `json:"description" yaml:"description"`

	// IneligibleStatus describes the reason why this owner is not eligible.
	IneligibleStatus string `json:"ineligible_status" yaml:"ineligible_status"`
}

// Audit describes the audit configuration for an access list.
type Audit struct {
	// Frequency is a duration that describes how often an access list must be audited.
	Frequency time.Duration `json:"frequency" yaml:"frequency"`

	// NextAuditDate is the date that the next audit should be performed.
	NextAuditDate time.Time `json:"next_audit_date" yaml:"next_audit_date"`
}

// Requires describes a requirement section for an access list. A user must
// meet the following criteria to obtain the specific access to the list.
type Requires struct {
	// Roles are the user roles that must be present for the user to obtain access.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits that must be present for the user to obtain access.
	Traits trait.Traits `json:"traits" yaml:"traits"`
}

// Grants describes what access is granted by membership to the access list.
type Grants struct {
	// Roles are the roles that are granted to users who are members of the access list.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits that are granted to users who are members of the access list.
	Traits trait.Traits `json:"traits" yaml:"traits"`
}

// Member describes a member of an access list.
type Member struct {
	// Name is the name of the member of the access list.
	Name string `json:"name" yaml:"name"`

	// Joined is when the user joined the access list.
	Joined time.Time `json:"joined" yaml:"joined"`

	// expires is when the user's membership to the access list expires.
	Expires time.Time `json:"expires" yaml:"expires"`

	// reason is the reason this user was added to the access list.
	Reason string `json:"reason" yaml:"reason"`

	// added_by is the user that added this user to the access list.
	AddedBy string `json:"added_by" yaml:"added_by"`
}

// NewAccessList will create a new access list.
func NewAccessList(metadata header.Metadata, spec Spec) (*AccessList, error) {
	accessList := &AccessList{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return accessList, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *AccessList) CheckAndSetDefaults() error {
	a.SetKind(types.KindAccessList)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if a.Spec.Title == "" {
		return trace.BadParameter("access list title required")
	}

	if len(a.Spec.Owners) == 0 {
		return trace.BadParameter("owners are missing")
	}

	if a.Spec.Audit.Frequency == 0 {
		return trace.BadParameter("audit frequency must be greater than 0")
	}

	// TODO(mdwn): Next audit date must not be zero.

	if len(a.Spec.Grants.Roles) == 0 && len(a.Spec.Grants.Traits) == 0 {
		return trace.BadParameter("grants must specify at least one role or trait")
	}

	// Deduplicate owners. The backend will currently prevent this, but it's possible that access lists
	// were created with duplicated owners before the backend checked for duplicate owners. In order to
	// ensure that these access lists are backwards compatible, we'll deduplicate them here.
	ownerMap := make(map[string]struct{}, len(a.Spec.Owners))
	deduplicatedOwners := []Owner{}
	for _, owner := range a.Spec.Owners {
		if owner.Name == "" {
			return trace.BadParameter("owner name is missing")
		}

		if _, ok := ownerMap[owner.Name]; ok {
			continue
		}

		ownerMap[owner.Name] = struct{}{}
		deduplicatedOwners = append(deduplicatedOwners, owner)
	}
	a.Spec.Owners = deduplicatedOwners

	return nil
}

// GetOwners returns the list of owners from the access list.
func (a *AccessList) GetOwners() []Owner {
	return a.Spec.Owners
}

// GetOwners returns the list of owners from the access list.
func (a *AccessList) SetOwners(owners []Owner) {
	a.Spec.Owners = owners
}

// GetAuditFrequency returns the audit frequency from the access list.
func (a *AccessList) GetAuditFrequency() time.Duration {
	return a.Spec.Audit.Frequency
}

// GetMembershipRequires returns the membership requires configuration from the access list.
func (a *AccessList) GetMembershipRequires() Requires {
	return a.Spec.MembershipRequires
}

// GetOwnershipRequires returns the ownership requires configuration from the access list.
func (a *AccessList) GetOwnershipRequires() Requires {
	return a.Spec.OwnershipRequires
}

// GetGrants returns the grants from the access list.
func (a *AccessList) GetGrants() Grants {
	return a.Spec.Grants
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *AccessList) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (a *AccessList) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName())
	return types.MatchSearch(fieldVals, values, nil)
}

func (a *Audit) UnmarshalJSON(data []byte) error {
	type Alias Audit
	audit := struct {
		Frequency     string `json:"frequency"`
		NextAuditDate string `json:"next_audit_date"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		return trace.Wrap(err)
	}

	var err error
	a.Frequency, err = time.ParseDuration(audit.Frequency)
	if err != nil {
		return trace.Wrap(err)
	}
	a.NextAuditDate, err = time.Parse(time.RFC3339Nano, audit.NextAuditDate)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a Audit) MarshalJSON() ([]byte, error) {
	type Alias Audit
	return json.Marshal(&struct {
		Frequency     string `json:"frequency"`
		NextAuditDate string `json:"next_audit_date"`
		Alias
	}{
		Alias:         (Alias)(a),
		Frequency:     a.Frequency.String(),
		NextAuditDate: a.NextAuditDate.Format(time.RFC3339Nano),
	})
}
