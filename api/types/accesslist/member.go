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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/utils"
)

var _ compare.IsEqual[*AccessListMember] = (*AccessListMember)(nil)

// AccessListMember is an access list member resource.
type AccessListMember struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access list member.
	Spec AccessListMemberSpec `json:"spec" yaml:"spec"`
}

// AccessListMemberSpec describes the specification of a member of an access list.
type AccessListMemberSpec struct {
	// AccessList is the name of the associated access list.
	AccessList string `json:"access_list" yaml:"access_list"`

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

	// IneligibleStatus describes the reason why this member is not eligible.
	IneligibleStatus string `json:"ineligible_status" yaml:"ineligible_status"`
}

// NewAccessListMember will create a new access listm member.
func NewAccessListMember(metadata header.Metadata, spec AccessListMemberSpec) (*AccessListMember, error) {
	member := &AccessListMember{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return member, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *AccessListMember) CheckAndSetDefaults() error {
	a.SetKind(types.KindAccessListMember)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if a.Spec.AccessList == "" {
		return trace.BadParameter("access list is missing")
	}

	if a.Spec.Name == "" {
		return trace.BadParameter("member name is missing")
	}

	if a.Spec.Joined.IsZero() || a.Spec.Joined.Unix() == 0 {
		return trace.BadParameter("member %s: joined field empty or missing", a.Spec.Name)
	}

	if a.Spec.AddedBy == "" {
		return trace.BadParameter("member %s: added_by field is empty", a.Spec.Name)
	}

	return nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *AccessListMember) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// IsEqual defines AccessListMember equality for use with
// `services.CompareResources()` (and hence the services.Reconciler).
//
// For the purposes of reconciliation, we only care that the user and target
// AccessList match.
func (a *AccessListMember) IsEqual(other *AccessListMember) bool {
	return a.Spec.Name == other.Spec.Name &&
		a.Spec.AccessList == other.Spec.AccessList
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (a *AccessListMember) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName())
	return types.MatchSearch(fieldVals, values, nil)
}
