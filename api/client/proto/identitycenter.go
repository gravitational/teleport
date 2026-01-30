// Copyright 2024 Gravitational, Inc.
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

package proto

import (
	"slices"
	time "time"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

// PackICAccountAssignment packs an Identity Center Account Assignment in to its
// wire format.
func PackICAccountAssignment(assignment *identitycenterv1.AccountAssignment) isPaginatedResource_Resource {
	return &PaginatedResource_IdentityCenterAccountAssignment{
		IdentityCenterAccountAssignment: &IdentityCenterAccountAssignment{
			Kind:        types.KindIdentityCenterAccountAssignment,
			Version:     assignment.GetVersion(),
			Metadata:    types.Metadata153ToLegacy(assignment.Metadata),
			DisplayName: assignment.GetSpec().GetDisplay(),
			Account: &IdentityCenterAccount{
				AccountName: assignment.GetSpec().GetAccountName(),
				ID:          assignment.GetSpec().GetAccountId(),
			},
			PermissionSet: &IdentityCenterPermissionSet{
				ARN:  assignment.GetSpec().GetPermissionSet().GetArn(),
				Name: assignment.GetSpec().GetPermissionSet().GetName(),
			},
		},
	}
}

// UnpackICAccountAssignment converts a wire-format IdentityCenterAccountAssignment
// resource back into an identitycenterv1.AccountAssignment instance.
func UnpackICAccountAssignment(src *IdentityCenterAccountAssignment) types.ResourceWithLabels {
	dst := &identitycenterv1.AccountAssignment{
		Kind:     types.KindIdentityCenterAccountAssignment,
		Version:  src.Version,
		Metadata: types.LegacyTo153Metadata(src.Metadata),
		Spec: &identitycenterv1.AccountAssignmentSpec{
			AccountId:   src.Account.ID,
			AccountName: src.Account.AccountName,
			Display:     src.DisplayName,
			PermissionSet: &identitycenterv1.PermissionSetInfo{
				Arn:  src.PermissionSet.ARN,
				Name: src.PermissionSet.Name,
			},
		},
	}
	return types.Resource153ToResourceWithLabels(dst)
}

// PackICResource packs an Identity Center Account Assignment in to its
// wire format.
func PackICResource(resource *identitycenterv1.Resource) isPaginatedResource_Resource {
	spec := resource.GetSpec()

	return &PaginatedResource_IdentityCenterResource{
		IdentityCenterResource: &IdentityCenterResource{
			Kind:         resource.Kind,
			SubKind:      resource.SubKind,
			Version:      resource.Version,
			Metadata:     types.Metadata153ToLegacy(resource.Metadata),
			DisplayName:  spec.GetName(),
			AWSAccount:   spec.GetAccount(),
			ARN:          spec.GetArn(),
			Dependencies: spec.GetDependencies(),
		},
	}
}

// UnpackICResource converts a wire-format IdentityCenterResource resource back
// into an identitycenterv1.Resource instance.
func UnpackICResource(src *IdentityCenterResource) types.ResourceWithLabels {
	dst := &identitycenterv1.Resource{
		Kind:     src.Kind,
		SubKind:  src.SubKind,
		Version:  src.Version,
		Metadata: types.LegacyTo153Metadata(src.Metadata),
		Spec: &identitycenterv1.ResourceSpec{
			Name:         src.DisplayName,
			Account:      src.AWSAccount,
			Arn:          src.ARN,
			Dependencies: slices.Clone(src.Dependencies),
		},
	}
	return types.Resource153ToResourceWithLabels(dst)
}

// SetSubKind sets resource subkind.
func (r *IdentityCenterResource) SetSubKind(subKind string) {
	r.SubKind = subKind
}

// GetName returns the name of the resource.
func (r *IdentityCenterResource) GetName() string {
	return r.Metadata.Name
}

// SetName sets the name of the resource.
func (r *IdentityCenterResource) SetName(name string) {
	r.Metadata.Name = name
}

// Expiry returns object expiry setting.
func (r *IdentityCenterResource) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetExpiry sets object expiry.
func (r *IdentityCenterResource) SetExpiry(expiry time.Time) {
	r.Metadata.SetExpiry(expiry)
}

// GetRevision returns the revision.
func (r *IdentityCenterResource) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision.
func (r *IdentityCenterResource) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

// Origin returns the origin value of the resource.
func (r *IdentityCenterResource) Origin() string {
	return r.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (r *IdentityCenterResource) SetOrigin(origin string) {
	r.Metadata.SetOrigin(origin)
}

// GetLabel returns the label with the provided key.
func (r *IdentityCenterResource) GetLabel(key string) (string, bool) {
	v, ok := r.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels.
func (r *IdentityCenterResource) GetAllLabels() map[string]string {
	return r.Metadata.Labels
}

// GetStaticLabels returns the resource's static labels.
func (r *IdentityCenterResource) GetStaticLabels() map[string]string {
	return r.Metadata.Labels
}

// SetStaticLabels sets the resource's static labels.
func (r *IdentityCenterResource) SetStaticLabels(sl map[string]string) {
	r.Metadata.Labels = sl
}

// MatchSearch goes through select field values and tries to match against the list of search values.
func (r *IdentityCenterResource) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()),
		r.GetName(),
		r.DisplayName,
		r.AWSAccount)
	return types.MatchSearch(fieldVals, values, nil)
}
