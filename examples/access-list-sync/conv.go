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

package main

import (
	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

// groupToAccessList converts an upstream Group into a Teleport AccessList,
// stamping the managed-by label so the reconciler can identify it.
func groupToAccessList(g Group, owners []accesslist.Owner) (*accesslist.AccessList, error) {
	acl, err := accesslist.NewAccessList(
		header.Metadata{
			Name:   g.ID,
			Labels: map[string]string{managedByLabel: managedByValue},
		},
		accesslist.Spec{
			Title:  g.DisplayName,
			Owners: owners,
		},
	)
	return acl, trace.Wrap(err)
}

// memberToAccessListMember converts an upstream Member into a Teleport
// AccessListMember for the given access list ID.
func memberToAccessListMember(accessListID string, m Member) (*accesslist.AccessListMember, error) {
	alm, err := accesslist.NewAccessListMember(
		header.Metadata{Name: m.UserName},
		accesslist.AccessListMemberSpec{
			AccessList:       accessListID,
			Name:             m.UserName,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		},
	)
	return alm, trace.Wrap(err)
}

// toOwners converts a slice of Teleport usernames into accesslist.Owner values
// with eligible membership status.
func toOwners(usernames ...string) []accesslist.Owner {
	out := make([]accesslist.Owner, 0, len(usernames))
	for _, name := range usernames {
		out = append(out, accesslist.Owner{
			Name:             name,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		})
	}
	return out
}
