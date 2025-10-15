/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/utils"
)

// TestAccessListCRUD tests backend operations with access list resources.
func TestAccessListCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	// Initially we expect no access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	}

	// Create both access lists.
	accessList, err := service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// Fetch all access lists.
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList1, accessList2}, out, cmpOpts...))

	// Fetch a paginated list of access lists
	paginatedOut := make([]*accesslist.AccessList, 0, 2)
	var nextToken string
	for {
		out, nextToken, err = service.ListAccessLists(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Len(t, paginatedOut, 2)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList1, accessList2}, paginatedOut, cmpOpts...))

	// Fetch a specific access list.
	accessList, err = service.GetAccessList(ctx, accessList2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// Try to fetch an access list that doesn't exist.
	_, err = service.GetAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Update an access list.
	accessList1.SetExpiry(clock.Now().Add(30 * time.Minute))
	accessList, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))

	// Delete an access list.
	err = service.DeleteAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList2}, out, cmpOpts...))

	// Try to delete an access list that doesn't exist.
	err = service.DeleteAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all access lists.
	err = service.DeleteAllAccessLists(ctx)
	require.NoError(t, err)
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Try to create an access list with duplicate owners.
	accessListDuplicateOwners := newAccessList(t, "accessListDuplicateOwners", clock)
	expectedAccessList := accessListDuplicateOwners.Spec.Owners
	accessListDuplicateOwners.Spec.Owners = append(accessListDuplicateOwners.Spec.Owners, accessListDuplicateOwners.Spec.Owners[0])

	created, err := service.UpsertAccessList(ctx, accessListDuplicateOwners)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedAccessList, created.Spec.Owners)
}

func requireAccessDenied(t require.TestingT, err error, i ...any) {
	require.True(
		t,
		trace.IsAccessDenied(err),
		"err should be access denied, was: %s", err,
	)
}

func Test_AccessList_validation_noTypeChange(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	type testCase struct {
		name         string
		accessList   *accesslist.AccessList
		illegalTypes []accesslist.Type
	}

	for _, tc := range []testCase{
		{
			name:         "from default",
			accessList:   newAccessList(t, "test-default-access-list-1", clock),
			illegalTypes: []accesslist.Type{accesslist.Static, accesslist.SCIM},
		},
		{
			name:         "from static",
			accessList:   newAccessList(t, "test-static-access-list-1", clock, withType(accesslist.Static)),
			illegalTypes: []accesslist.Type{accesslist.Default, accesslist.SCIM},
		},
		{
			name:         "from scim",
			accessList:   newAccessList(t, "test-scim-access-list-1", clock, withType(accesslist.SCIM)),
			illegalTypes: []accesslist.Type{accesslist.Default, accesslist.Static},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accessList, err := service.UpsertAccessList(ctx, tc.accessList)
			require.NoError(t, err)

			for _, typ := range tc.illegalTypes {
				t.Run(string(typ), func(t *testing.T) {
					accessList.Spec.Type = typ
					if !typ.IsReviewable() {
						accessList.Spec.Audit = accesslist.Audit{}
					}

					_, err := service.UpdateAccessList(ctx, accessList)
					require.Error(t, err)
					require.ErrorContains(t, err, "cannot be changed")
					require.True(t, trace.IsBadParameter(err))

					_, err = service.UpsertAccessList(ctx, accessList)
					require.Error(t, err)
					require.ErrorContains(t, err, "cannot be changed")
					require.True(t, trace.IsBadParameter(err))

					_, _, err = service.UpsertAccessListWithMembers(ctx, accessList, nil)
					require.Error(t, err)
					require.ErrorContains(t, err, "cannot be changed")
					require.True(t, trace.IsBadParameter(err))
				})
			}
		})
	}
}

func Test_AccessList_validation_DeprecatedDynamic_special_case(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	accessList := newAccessList(t, "test-scim-access-list-1", clock)

	t.Run("dynamic is stored as default", func(t *testing.T) {
		_, err := backend.NewSanitizer(mem).Get(ctx, service.service.MakeKey(backend.NewKey(accessList.GetName())))
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))

		accessList.Spec.Type = accesslist.DeprecatedDynamic
		_, err = service.UpsertAccessList(ctx, accessList)
		require.NoError(t, err)

		accessList = getAccessListDirectlyFromBackend(t, mem, service.service.MakeKey(backend.NewKey(accessList.GetName())))
		require.NoError(t, err)
		require.Equal(t, accesslist.Default, accessList.Spec.Type)
	})

	t.Run("if stored already stored as dynamic", func(t *testing.T) {
		verificationDescValue := "updated to deprecated dynamic bypassing defaulting"

		t.Run("store with dynamic type directly in the backend", func(t *testing.T) {
			accessList, err = service.GetAccessList(ctx, accessList.GetName())
			require.NoError(t, err)
			require.NotEqual(t, verificationDescValue, accessList.Spec.Description)

			modifyAccessListDirectlyInBackend(t, mem, accessList, service.service.MakeBackendItem, func(al *accesslist.AccessList) {
				al.Spec.Type = accesslist.DeprecatedDynamic
				al.Spec.Description = verificationDescValue
			})
		})

		t.Run("getting through service return default when stored as deprecated dynamic", func(t *testing.T) {
			accessList, err := service.GetAccessList(ctx, accessList.GetName())
			require.NoError(t, err)
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.Default, accessList.Spec.Type)

			accessList = getAccessListDirectlyFromBackend(t, mem, service.service.MakeKey(backend.NewKey(accessList.GetName())))
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.DeprecatedDynamic, accessList.Spec.Type)
		})

		t.Run("modifying access list type stored as deprecated dynamic is still not allowed", func(t *testing.T) {
			accessList = getAccessListDirectlyFromBackend(t, mem, service.service.MakeKey(backend.NewKey(accessList.GetName())))
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.DeprecatedDynamic, accessList.Spec.Type)

			accessList, err := service.GetAccessList(ctx, accessList.GetName())
			require.NoError(t, err)

			accessList.Spec.Type = accesslist.SCIM
			accessList.Spec.Audit = accesslist.Audit{}

			_, err = service.UpsertAccessList(ctx, accessList)
			require.Error(t, err)
			require.ErrorContains(t, err, `type "" cannot be changed to "scim"`)
			require.True(t, trace.IsBadParameter(err))

			accessList.Spec.Type = accesslist.Static
			accessList.Spec.Audit = accesslist.Audit{}

			_, err = service.UpsertAccessList(ctx, accessList)
			require.Error(t, err)
			require.ErrorContains(t, err, `type "" cannot be changed to "static"`)
			require.True(t, trace.IsBadParameter(err))
		})

		t.Run("modifying through service changes stored as deprecated dynamic type to default", func(t *testing.T) {
			accessList = getAccessListDirectlyFromBackend(t, mem, service.service.MakeKey(backend.NewKey(accessList.GetName())))
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.DeprecatedDynamic, accessList.Spec.Type)

			accessList, err := service.GetAccessList(ctx, accessList.GetName())
			require.NoError(t, err)
			require.Equal(t, verificationDescValue, accessList.Spec.Description)

			accessList.Spec.Type = accesslist.DeprecatedDynamic
			_, err = service.UpsertAccessList(ctx, accessList)
			require.NoError(t, err)

			accessList, err = service.GetAccessList(ctx, accessList.GetName())
			require.NoError(t, err)
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.Default, accessList.Spec.Type)

			accessList = getAccessListDirectlyFromBackend(t, mem, service.service.MakeKey(backend.NewKey(accessList.GetName())))
			require.Equal(t, verificationDescValue, accessList.Spec.Description)
			require.Equal(t, accesslist.Default, accessList.Spec.Type)
		})

	})
}

func getAccessListDirectlyFromBackend(t *testing.T, storage backend.Backend, key backend.Key) *accesslist.AccessList {
	t.Helper()
	ctx := context.Background()

	item, err := backend.NewSanitizer(storage).Get(ctx, key)
	require.NoError(t, err)
	accessList := new(accesslist.AccessList)
	err = utils.FastUnmarshal(item.Value, &accessList)
	require.NoError(t, err)
	return accessList
}

func modifyAccessListDirectlyInBackend(
	t *testing.T,
	storage backend.Backend,
	accessList *accesslist.AccessList,
	makeItemFn func(*accesslist.AccessList, ...any) (backend.Item, error),
	modifyFn func(*accesslist.AccessList),
) {
	t.Helper()
	ctx := context.Background()

	item, err := makeItemFn(accessList)
	require.NoError(t, err)

	// Now, because makeItemFn calls CheckAndSetDefault and we don't want to have any values
	// defaulted/validated do the unmarshal/modify/marshal dance.

	accessList = new(accesslist.AccessList)
	err = utils.FastUnmarshal(item.Value, &accessList)
	require.NoError(t, err)

	modifyFn(accessList)
	item.Value, err = utils.FastMarshal(accessList)
	require.NoError(t, err)

	// And finally store the modified, non-defaulted, non-validated item.

	_, err = backend.NewSanitizer(storage).Put(ctx, item)
	require.NoError(t, err)
}

// TestAccessList_EntitlementLimits asserts that any limits on creating
// AccessLists are correctly enforced at Upsert time.
func TestAccessList_EntitlementLimits(t *testing.T) {
	type aclSelector func([]*accesslist.AccessList) *accesslist.AccessList

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// an ACL selection function that creates a new AccessList to insert
	createNew := func([]*accesslist.AccessList) *accesslist.AccessList {
		return newAccessList(t, "test-target", clock)
	}

	// an ACL selection function that selects the nth access list from the list
	// of pre-created ACLs. Used to simulate an update.
	update := func(n int) aclSelector {
		return func(acls []*accesslist.AccessList) *accesslist.AccessList {
			return acls[n]
		}
	}

	testCases := []struct {
		name             string
		igsEnabled       bool
		aclName          string
		entitlement      modules.EntitlementInfo
		existingACLCount int
		aclSelector      aclSelector
		expectErrorFn    require.ErrorAssertionFunc
		expectedACLCount int
	}{
		{
			name:             "igs-enabled-no-limit-on-create",
			igsEnabled:       true,
			entitlement:      modules.EntitlementInfo{Enabled: false},
			existingACLCount: 3,
			aclSelector:      createNew,
			expectErrorFn:    require.NoError,
			expectedACLCount: 4,
		},
		{
			name:             "can-create-one-access-list-when-disabled",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: false},
			existingACLCount: 0,
			aclSelector:      createNew,
			expectErrorFn:    require.NoError,
			expectedACLCount: 1,
		},
		{
			name:             "cant-create-a-second-access-list-when-disabled",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: false},
			existingACLCount: 1,
			aclSelector:      createNew,
			expectErrorFn:    requireAccessDenied,
			expectedACLCount: 1,
		},
		{
			name:             "disabled-allows-update",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: false},
			existingACLCount: 3,
			aclSelector:      update(1),
			expectErrorFn:    require.NoError,
			expectedACLCount: 3,
		},
		{
			name:             "under-default-limit-succeeds",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 1},
			existingACLCount: 0,
			aclSelector:      createNew,
			expectErrorFn:    require.NoError,
			expectedACLCount: 1,
		},
		{
			name:             "at-default-limit-fails",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 1},
			existingACLCount: 1,
			aclSelector:      createNew,
			expectErrorFn:    requireAccessDenied,
			expectedACLCount: 1,
		},
		{
			name:             "at-default-limit-allows-update",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 1},
			existingACLCount: 1,
			aclSelector:      update(0),
			expectErrorFn:    require.NoError,
			expectedACLCount: 1,
		},
		{
			name:             "infinite-limit-succeeds",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 0},
			existingACLCount: 5,
			aclSelector:      createNew,
			expectErrorFn:    require.NoError,
			expectedACLCount: 6,
		},
		{
			name:             "above-limit-fails",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 10},
			existingACLCount: 20,
			aclSelector:      createNew,
			expectErrorFn:    requireAccessDenied,
			expectedACLCount: 20,
		},
		{
			name:             "above-limit-allows-update",
			igsEnabled:       false,
			entitlement:      modules.EntitlementInfo{Enabled: true, Limit: 10},
			existingACLCount: 20,
			aclSelector:      update(15),
			expectErrorFn:    require.NoError,
			expectedACLCount: 20,
		},
	}

	// aclOperation abstracts over Upsert and UpsertWithMembers so we can use
	// the same test code and cases for both operations.
	type aclOperation func(context.Context, *AccessListService, *accesslist.AccessList) (*accesslist.AccessList, error)

	operations := []struct {
		name   string
		invoke aclOperation
	}{
		{
			name: "Upsert",
			invoke: func(ctx context.Context, uut *AccessListService, acl *accesslist.AccessList) (*accesslist.AccessList, error) {
				return uut.UpsertAccessList(ctx, acl)
			},
		},
		{
			name: "UpsertWithMembers",
			invoke: func(ctx context.Context, uut *AccessListService, acl *accesslist.AccessList) (*accesslist.AccessList, error) {
				updatedACL, _, err := uut.UpsertAccessListWithMembers(ctx, acl, []*accesslist.AccessListMember{})
				return updatedACL, err
			},
		},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// GIVEN an AccessList service specifically configured with/without
					// IGS and a specific AccessList entitlement...
					mem, err := memory.New(memory.Config{
						Context: ctx,
						Clock:   clock,
					})
					require.NoError(t, err, "failed creating in-memory backend")
					uut := newAccessListService(t, mem, clock, tc.igsEnabled)

					// note - we do this _after_ creating the AccessList Service test
					// target because the `newAccessListService()` fixture also sets the
					// test modules, and that would clobber our test setup if we went
					// first
					modulestest.SetTestModules(t, modulestest.Modules{
						TestBuildType: modules.BuildEnterprise,
						TestFeatures: modules.Features{
							Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
								entitlements.Identity:    {Enabled: tc.igsEnabled},
								entitlements.AccessLists: tc.entitlement,
							},
						},
					})

					// ALSO GIVEN a number of pre-created AccessLists...
					var preCreatedACLs []*accesslist.AccessList
					for i := range tc.existingACLCount {
						// note that we write these setup resources directly to the back-end
						// service in order to bypass any limit enforcement. This lets us
						// set up a wider range of interesting test cases
						acl, err := uut.service.UpsertResource(ctx,
							newAccessList(t, fmt.Sprintf("accessList-%02d", i), clock))
						require.NoError(t, err, "Creating existing AccessLists for test")
						preCreatedACLs = append(preCreatedACLs, acl)
					}

					// WHEN I attempt to create a new AccessList or update an existing
					// one...
					testACL := tc.aclSelector(preCreatedACLs)
					_, err = op.invoke(ctx, uut, testACL)

					// EXPECT that the error state will match the expectation in the
					// test case

					tc.expectErrorFn(t, err)

					// ALSO EXPECT that the number of AccessLists stored by the service
					// matches the expectation in the test case
					out, err := uut.GetAccessLists(ctx)
					require.NoError(t, err)
					require.Len(t, out, tc.expectedACLCount)
				})
			}
		})
	}
}

// TestAccessListCreate_UpdateAccessList tests creating access list
// and updating access list with the same name.
func TestAccessListCreate_UpdateAccessList(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// No limit to creating access list.
	result, err := service.UpsertAccessList(ctx, newAccessList(t, "accessList1", clock))
	require.NoError(t, err)
	// Fetch all access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 1)

	result.Spec.Description = "changing description"
	// Update access list with the correct revision.
	_, err = service.UpdateAccessList(ctx, result)
	require.NoError(t, err)
	result.Spec.Description = "changing description again"
	result.Metadata.Revision = "fake revision"
	// Update access list with wrong revision should return an error.
	_, err = service.UpdateAccessList(ctx, result)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected precondition failed error, got %v", err)
}

func TestAccessListDedupeOwnersBackwardsCompat(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Put an unduplicated owners access list in the backend.
	accessListDuplicateOwners := newAccessList(t, "accessListDuplicateOwners", clock)
	accessListDuplicateOwners.Spec.Owners = append(accessListDuplicateOwners.Spec.Owners, accessListDuplicateOwners.Spec.Owners[0])
	require.Len(t, accessListDuplicateOwners.Spec.Owners, 3)

	item, err := service.service.MakeBackendItem(accessListDuplicateOwners)
	require.NoError(t, err)
	_, err = mem.Put(ctx, item)
	require.NoError(t, err)

	accessList, err := service.GetAccessList(ctx, accessListDuplicateOwners.GetName())
	require.NoError(t, err)

	require.Len(t, accessList.Spec.Owners, 2)
}

func TestAccessListUpsertWithMembers(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	}

	t.Run("create access list", func(t *testing.T) {
		// Create both access lists.
		accessList, _, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	})

	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "alice")

	t.Run("add member to the access list", func(t *testing.T) {
		// Add access list members.
		updatedAccessList, updatedMembers, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessList1Member1})
		require.NoError(t, err)
		// Assert that access list is returned.
		require.Empty(t, cmp.Diff(updatedAccessList, updatedAccessList, cmpOpts...))
		// Assert that the member is returned.
		require.Len(t, updatedMembers, 1)
		require.Empty(t, cmp.Diff(updatedMembers[0], accessList1Member1, cmpOpts...))

		listMembers, err := service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member1, cmpOpts...))
	})

	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "bob")

	t.Run("add another member to the access list", func(t *testing.T) {
		// Add access list members.
		updatedAccessList, updatedMembers, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessList1Member1, accessList1Member2})
		require.NoError(t, err)
		// Assert that access list is returned.
		require.Empty(t, cmp.Diff(updatedAccessList, updatedAccessList, cmpOpts...))
		// Assert that the member is returned.
		require.Len(t, updatedMembers, 2)
		require.Empty(t, cmp.Diff(updatedMembers, []*accesslist.AccessListMember{accessList1Member1, accessList1Member2}, cmpOpts...))

		listMembers, err := service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member1, cmpOpts...))

		listMembers, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member2, cmpOpts...))
	})

	t.Run("empty members removes all members", func(t *testing.T) {
		_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{})
		require.NoError(t, err)

		members, _, err := service.ListAccessListMembers(ctx, accessList1.GetName(), 0 /* default size*/, "")
		require.NoError(t, err)
		require.Empty(t, members)
	})
}

func TestAccessListMembersCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	}

	// Create both access lists.
	accessList, err := service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// There should be no access list members for either list.
	members, _, err := service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	// Listing members of a non existent list should produce an error.
	_, _, err = service.ListAccessListMembers(ctx, "non-existent", 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent\" doesn't exist"))

	// Verify access list members are not present.
	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "alice")
	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "bob")

	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.True(t, trace.IsNotFound(err))
	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.True(t, trace.IsNotFound(err))

	// Add access list members.
	member, err := service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member1, member, cmpOpts...))
	member, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member2, member, cmpOpts...))

	member, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member1, member, cmpOpts...))
	member, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member2, member, cmpOpts...))

	// Add access list member for non existent list should produce an error.
	_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t, "non-existent-list", "nobody"))
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	accessList2Member1 := newAccessListMember(t, accessList2.GetName(), "bob")
	accessList2Member2 := newAccessListMember(t, accessList2.GetName(), "jim")
	member, err = service.UpsertAccessListMember(ctx, accessList2Member1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2Member1, member, cmpOpts...))
	member, err = service.UpsertAccessListMember(ctx, accessList2Member2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2Member2, member, cmpOpts...))

	// Fetch a paginated list of access lists members
	var paginatedMembers []*accesslist.AccessListMember
	var nextToken string
	const pageSize = 1
	for {
		members, nextToken, err = service.ListAccessListMembers(ctx, accessList1.GetName(), pageSize, nextToken)
		require.NoError(t, err)

		paginatedMembers = append(paginatedMembers, members...)
		if nextToken == "" {
			break
		}
	}
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{accessList1Member1, accessList1Member2}, paginatedMembers, cmpOpts...))

	// Delete a member from an access list.
	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.NoError(t, err)

	require.NoError(t, service.DeleteAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName()))

	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.True(t, trace.IsNotFound(err))

	// Delete from a non-existent access list should return an error.
	err = service.DeleteAccessListMember(ctx, "non-existent-list", "nobody")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	// Delete an access list.
	err = service.DeleteAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)

	// Verify that the access list's members have been removed and that the other has not been affected.
	_, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.True(t, trace.IsNotFound(err), "missing access list should produce not found error during list")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList1.GetName()))

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, members)

	// Re-add access list 1 with its members.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	// Verify that the members were previously removed.
	members, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	_, err = service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	_, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)

	// try to update a member with the wrong revision.
	accessList1Member2.Metadata.Revision = "fake revision"
	_, err = service.UpdateAccessListMember(ctx, accessList1Member2)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected precondition failed error, got %v", err)

	// Delete all members from access list 1.
	require.NoError(t, service.DeleteAllAccessListMembersForAccessList(ctx, accessList1.GetName()))

	members, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	// Try to delete all members from a non-existent list.
	err = service.DeleteAllAccessListMembersForAccessList(ctx, "non-existent-list")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, members)

	// Try to delete an access list that doesn't exist.
	err = service.DeleteAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all access lists.
	err = service.DeleteAllAccessLists(ctx)
	require.NoError(t, err)

	// Verify that access lists are gone.
	_, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList1.GetName()))

	_, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList2.GetName()))
}

func Test_AccessListMember_Validation(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	accessList := newAccessList(t, "test-access-list-1", clock)
	accessList, err = service.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	accessListMember := newAccessListMember(t, accessList.GetName(), "test-access-list-member-1")

	t.Run("modifying member fails if name is empty", func(t *testing.T) {
		oldName := accessListMember.Spec.Name
		runAccessListMemberValidationSuite(
			t, service, accessList, accessListMember,
			func(m *accesslist.AccessListMember) { m.Spec.Name = "" },
			func(m *accesslist.AccessListMember) { m.Spec.Name = oldName },
			func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "spec name")
				require.True(t, trace.IsBadParameter(err))
			},
		)
	})

	t.Run("modifying member fails if spec.name and metadata.name do not match", func(t *testing.T) {
		oldName := accessListMember.Spec.Name
		runAccessListMemberValidationSuite(
			t, service, accessList, accessListMember,
			func(m *accesslist.AccessListMember) { m.Spec.Name = "some_other_name" },
			func(m *accesslist.AccessListMember) { m.Spec.Name = oldName },
			func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "spec name")
				require.True(t, trace.IsBadParameter(err))
			},
		)
	})

	t.Run("modifying member fails if access_list is empty", func(t *testing.T) {
		oldAccessList := accessListMember.Spec.AccessList
		runAccessListMemberValidationSuite(
			t, service, accessList, accessListMember,
			func(m *accesslist.AccessListMember) { m.Spec.AccessList = "" },
			func(m *accesslist.AccessListMember) { m.Spec.AccessList = oldAccessList },
			func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "access_list field empty")
				require.True(t, trace.IsBadParameter(err))
			},
		)
	})

	t.Run("modifying member fails if joined is empty", func(t *testing.T) {
		oldJoined := accessListMember.Spec.Joined
		runAccessListMemberValidationSuite(
			t, service, accessList, accessListMember,
			func(m *accesslist.AccessListMember) { m.Spec.Joined = time.Time{} },
			func(m *accesslist.AccessListMember) { m.Spec.Joined = oldJoined },
			func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "joined field empty")
				require.True(t, trace.IsBadParameter(err))
			},
		)
	})

	t.Run("modifying member fails if added_by is empty", func(t *testing.T) {
		oldAddedBy := accessListMember.Spec.AddedBy
		runAccessListMemberValidationSuite(
			t, service, accessList, accessListMember,
			func(m *accesslist.AccessListMember) { m.Spec.AddedBy = "" },
			func(m *accesslist.AccessListMember) { m.Spec.AddedBy = oldAddedBy },
			func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "added_by field is empty")
				require.True(t, trace.IsBadParameter(err))
			},
		)
	})
}

func runAccessListMemberValidationSuite(
	t *testing.T,
	service *AccessListService,
	accessList *accesslist.AccessList, member *accesslist.AccessListMember,
	makeBad, makeGood func(*accesslist.AccessListMember),
	errorCheck func(t *testing.T, err error),
) {
	t.Helper()
	ctx := context.Background()

	makeBad(member)

	_, err := service.UpsertAccessListMember(ctx, member)
	errorCheck(t, err)

	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList, []*accesslist.AccessListMember{member})
	errorCheck(t, err)

	makeGood(member)

	member, err = service.UpsertAccessListMember(ctx, member)
	require.NoError(t, err)

	makeBad(member)

	_, err = service.UpdateAccessListMember(ctx, member)
	errorCheck(t, err)

	_, err = service.UpsertAccessListMember(ctx, member)
	errorCheck(t, err)

	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList, []*accesslist.AccessListMember{member})
	errorCheck(t, err)

	makeGood(member)

	err = service.DeleteAccessListMember(ctx, member.Spec.AccessList, member.Spec.Name)
	require.NoError(t, err)
}

func TestUpsertAndUpdateAccessListWithMembers_PreservesIdentityCenterLablesForExistingMembers(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	accessList1 := newAccessList(t, "accessList1", clock)
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "aws-ic-user")
	accessList1Member1.SetOrigin(common.OriginAWSIdentityCenter)
	accessList1Member1.Metadata.Labels["foo"] = "bar"

	_, err = service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)

	member, err := service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.NoError(t, err)
	require.Empty(
		t,
		cmp.Diff(
			accessList1Member1,
			member,
			cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
			cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "Joined"),
		))

	dupeMemberButWithoutOriginLabel := newAccessListMember(t, accessList1.GetName(), "aws-ic-user")
	_, updatedMembers, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{dupeMemberButWithoutOriginLabel})
	require.NoError(t, err)
	require.Equal(t, "bar", updatedMembers[0].GetMetadata().Labels["foo"])

	updatedMember, err := service.UpdateAccessListMember(ctx, dupeMemberButWithoutOriginLabel)
	require.NoError(t, err)
	require.Equal(t, "bar", updatedMember.GetMetadata().Labels["foo"])

	upsertedMember, err := service.UpdateAccessListMember(ctx, dupeMemberButWithoutOriginLabel)
	require.NoError(t, err)
	require.Equal(t, "bar", upsertedMember.GetMetadata().Labels["foo"])
}

func TestAccessListReviewCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock, withType(accesslist.DeprecatedDynamic))

	accessList1OrigDate := accessList1.Spec.Audit.NextAuditDate
	accessList2OrigDate := accessList2.Spec.Audit.NextAuditDate

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		cmpopts.SortSlices(func(review1, review2 *accesslist.Review) bool {
			return review1.GetName() < review2.GetName()
		}),
	}

	// Create both access lists.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	_, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)

	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "member1")
	_, err = service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "member2")
	_, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)
	accessList2Member1 := newAccessListMember(t, accessList2.GetName(), "member1")
	_, err = service.UpsertAccessListMember(ctx, accessList2Member1)
	require.NoError(t, err)
	accessList2Member2 := newAccessListMember(t, accessList2.GetName(), "member2")
	_, err = service.UpsertAccessListMember(ctx, accessList2Member2)
	require.NoError(t, err)

	// There should be no access list reviews for either list.
	reviews, _, err := service.ListAccessListReviews(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	// Listing reviews of a non existent list should produce an error.
	_, _, err = service.ListAccessListReviews(ctx, "non-existent", 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent\" doesn't exist"))

	accessList1Review1 := newAccessListReview(t, accessList1.GetName(), "al1-review1")
	accessList1Review2 := newAccessListReview(t, accessList1.GetName(), "al1-review2")
	accessList1Review2.Spec.Changes.RemovedMembers = nil
	accessList2Review1 := newAccessListReview(t, accessList2.GetName(), "al2-review1")
	accessList2Review1.Spec.Changes.MembershipRequirementsChanged = nil
	accessList2Review1.Spec.Changes.RemovedMembers = nil
	accessList2Review1.Spec.Changes.ReviewFrequencyChanged = 0
	accessList2Review1.Spec.Changes.ReviewDayOfMonthChanged = 0
	var nextReviewDate time.Time

	// Add access list review.
	accessList1Review1, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList1Review1)
	require.NoError(t, err)

	// Verify changes to access list.
	accessList1Updated, err := service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList1OrigDate.Year(),
			accessList1OrigDate.Month()+time.Month(accessList1Updated.Spec.Audit.Recurrence.Frequency),
			int(accessList1Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList1Updated.Spec.Audit.NextAuditDate,
	)
	require.Empty(t, cmp.Diff(*(accessList1Review1.Spec.Changes.MembershipRequirementsChanged), accessList1Updated.Spec.MembershipRequires))
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewFrequencyChanged, accessList1Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewDayOfMonthChanged, accessList1Updated.Spec.Audit.Recurrence.DayOfMonth)
	// The Correct value is returned through the API.
	require.Equal(t, accessList1Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.True(t, trace.IsNotFound(err))
	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.True(t, trace.IsNotFound(err))

	// Add another review
	accessList1Review2, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList1Review2)
	require.NoError(t, err)

	// Verify changes to the access list again.
	accessList1Updated, err = service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList1OrigDate.Year(),
			accessList1OrigDate.Month()+time.Month(accessList1Updated.Spec.Audit.Recurrence.Frequency)*2,
			int(accessList1Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList1Updated.Spec.Audit.NextAuditDate,
	)

	// Attempting to apply changes already reflected in the access list should modify the original review.
	require.Nil(t, accessList1Review2.Spec.Changes.MembershipRequirementsChanged)
	require.Equal(t, 0, int(accessList1Review2.Spec.Changes.ReviewFrequencyChanged))
	require.Equal(t, 0, int(accessList1Review2.Spec.Changes.ReviewDayOfMonthChanged))

	// No changes should have been made.
	require.Empty(t, cmp.Diff(*(accessList1Review1.Spec.Changes.MembershipRequirementsChanged), accessList1Updated.Spec.MembershipRequires))
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewFrequencyChanged, accessList1Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewDayOfMonthChanged, accessList1Updated.Spec.Audit.Recurrence.DayOfMonth)
	require.Equal(t, accessList1Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	// Review that doesn't change anything
	accessList2Review1, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList2Review1)
	require.NoError(t, err)

	accessList2Updated, err := service.GetAccessList(ctx, accessList2.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList2OrigDate.Year(),
			accessList2OrigDate.Month()+time.Month(accessList2Updated.Spec.Audit.Recurrence.Frequency),
			int(accessList2Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList2Updated.Spec.Audit.NextAuditDate,
	)
	require.Empty(t, cmp.Diff(accessList2.Spec.MembershipRequires, accessList2Updated.Spec.MembershipRequires))
	require.Equal(t, accessList2.Spec.Audit.Recurrence.Frequency, accessList2Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList2.Spec.Audit.Recurrence.DayOfMonth, accessList2Updated.Spec.Audit.Recurrence.DayOfMonth)
	require.Equal(t, accessList2Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.NoError(t, err)
	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member2.GetName())
	require.NoError(t, err)

	// Fetch a paginated list of access lists reviews
	var paginatedReviews []*accesslist.Review
	var nextToken string
	const pageSize = 1
	for {
		reviews, nextToken, err = service.ListAccessListReviews(ctx, accessList1.GetName(), pageSize, nextToken)
		require.NoError(t, err)

		paginatedReviews = append(paginatedReviews, reviews...)
		if nextToken == "" {
			break
		}
	}
	require.Empty(t, cmp.Diff([]*accesslist.Review{accessList1Review1, accessList1Review2}, paginatedReviews, cmpOpts...))

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 1, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.Review{accessList2Review1}, reviews, cmpOpts...))

	// Delete a review from an access list.
	require.NoError(t, service.DeleteAccessListReview(ctx, accessList2.GetName(), accessList2Review1.GetName()))

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 1, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	// Delete from a non-existent access list should return an error.
	err = service.DeleteAccessListReview(ctx, "non-existent-list", "no-review")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	// Delete a non-existent access list review.
	err = service.DeleteAccessListReview(ctx, accessList2.GetName(), "no-review")
	require.ErrorIs(t, err, trace.NotFound("access_list_review \"no-review\" doesn't exist"))

	// Try to delete all reviews from a non-existent list.
	// Delete all access list reviews.
	err = service.DeleteAllAccessListReviews(ctx)
	require.NoError(t, err)

	// Verify that access lists reviews are gone.
	_, _, err = service.ListAccessListReviews(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
}

func Test_CreateAccessListReview_NestedListMemberRemoval(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	a1, err := service.UpsertAccessList(ctx, newAccessList(t, "test_list_1", clock))
	require.NoError(t, err)

	memberList1, err := service.UpsertAccessList(ctx, newAccessList(t, "test_list_member_1", clock))
	require.NoError(t, err)

	assertMemberOf(t, ctx, service, memberList1.GetName(), nil)

	// When nested access list member is added, expect it's status.member_of to be updated.

	_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t,
		a1.GetName(),
		memberList1.GetName(),
		withMembershipKind(accesslist.MembershipKindList),
	))
	require.NoError(t, err)

	assertMemberOf(t, ctx, service, memberList1.GetName(), []string{a1.GetName()})

	// When nested access list member is removed via the review, expect it's status.member_of
	// to be updated.

	a1r1 := newAccessListReview(t, a1.GetName(), "al1-review1")
	a1r1.Spec.Changes.RemovedMembers = []string{
		memberList1.GetName(),
	}
	_, _, err = service.CreateAccessListReview(ctx, a1r1)
	require.NoError(t, err)

	assertMemberOf(t, ctx, service, memberList1.GetName(), nil)
}

func Test_CreateAccessListReview_FailForNonReviewable(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock, withType(accesslist.Static))
	accessList2 := newAccessList(t, "accessList2", clock, withType(accesslist.SCIM))

	// Create both access lists.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	_, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)

	accessList1Review := newAccessListReview(t, accessList1.GetName(), "al1-review")
	accessList2Review := newAccessListReview(t, accessList2.GetName(), "al2-review")

	// Add access list review.
	_, _, err = service.CreateAccessListReview(ctx, accessList1Review)
	require.Error(t, err)
	require.ErrorContains(t, err, "is not reviewable")
	require.True(t, trace.IsBadParameter(err))
	_, _, err = service.CreateAccessListReview(ctx, accessList2Review)
	require.Error(t, err)
	require.ErrorContains(t, err, "is not reviewable")
	require.True(t, trace.IsBadParameter(err))
}

func TestAccessListRequiresEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        accesslist.Requires
		b        accesslist.Requires
		expected bool
	}{
		{
			name:     "empty",
			expected: true,
		},
		{
			name: "both equal",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: true,
		},
		{
			name: "roles length",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "c", "d"},
			},
			expected: false,
		},
		{
			name: "roles content",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "d"},
			},
			expected: false,
		},
		{
			name: "trait length",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
					"trait3": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait key different",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait3": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait values length",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2", "val3"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait values different",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val3"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, accessListRequiresEqual(test.a, test.b))
		})
	}
}

func TestAccessListMemberOwnerEligibility(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	acl := newAccessList(t, "test-access-list-1", clock,
		withOwners([]accesslist.Owner{{Name: "test-owner-1"}}),
		withOwnerRequires(accesslist.Requires{}),
		withMemberRequires(accesslist.Requires{}),
	)

	aclR := newAccessList(t, "test-access-list-2", clock,
		withOwners([]accesslist.Owner{{Name: "test-owner-1"}}),
		withOwnerRequires(accesslist.Requires{Roles: []string{"role1"}}),
		withMemberRequires(accesslist.Requires{Roles: []string{"role2"}}),
	)

	_, err = service.UpsertAccessList(ctx, acl)
	require.NoError(t, err)
	item, err := service.GetAccessList(ctx, acl.GetName())
	require.NoError(t, err)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(), item.GetOwners()[0].IneligibleStatus)

	_, err = service.UpsertAccessList(ctx, aclR)
	require.NoError(t, err)
	item2, err := service.GetAccessList(ctx, aclR.GetName())
	require.NoError(t, err)
	require.Empty(t, item2.GetOwners()[0].IneligibleStatus)

	_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t, acl.GetName(), "member1", withExpire(time.Time{})))
	require.NoError(t, err)
	m, err := service.GetAccessListMember(ctx, acl.GetName(), "member1")
	require.NoError(t, err)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(), m.Spec.IneligibleStatus)

	_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t, aclR.GetName(), "member1"))
	require.NoError(t, err)
	m, err = service.GetAccessListMember(ctx, aclR.GetName(), "member1")
	require.NoError(t, err)
	require.Empty(t, m.Spec.IneligibleStatus)

	require.NoError(t, service.DeleteAllAccessLists(ctx))

	_, _, err = service.UpsertAccessListWithMembers(ctx, acl, []*accesslist.AccessListMember{
		newAccessListMember(t, acl.GetName(), "member1", withExpire(time.Time{})),
		newAccessListMember(t, acl.GetName(), "member2", withExpire(time.Now().Add(time.Hour))),
	})
	require.NoError(t, err)
	m1, err := service.GetAccessListMember(ctx, acl.GetName(), "member1")
	require.NoError(t, err)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(), m1.Spec.IneligibleStatus)

	m2, err := service.GetAccessListMember(ctx, acl.GetName(), "member2")
	require.NoError(t, err)
	require.Empty(t, m2.Spec.IneligibleStatus)

	require.NoError(t, service.DeleteAllAccessLists(ctx))
	member := newAccessListMember(t, acl.GetName(), "member1", withExpire(time.Time{}))
	member.Spec.IneligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST.String()
	_, _, err = service.UpsertAccessListWithMembers(ctx, acl, []*accesslist.AccessListMember{member})
	require.NoError(t, err)

	m3, err := service.GetAccessListMember(ctx, acl.GetName(), "member1")
	require.NoError(t, err)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST.String(), m3.Spec.IneligibleStatus)
}

type newAccessListOptions struct {
	typ            accesslist.Type
	owners         []accesslist.Owner
	ownerRequires  accesslist.Requires
	memberRequires accesslist.Requires
}

type newAccessListOpt func(*newAccessListOptions)

func withType(typ accesslist.Type) newAccessListOpt {
	return func(o *newAccessListOptions) {
		o.typ = typ
	}
}

func withOwners(owners []accesslist.Owner) newAccessListOpt {
	return func(o *newAccessListOptions) {
		o.owners = owners
	}
}

func withOwnerRequires(req accesslist.Requires) newAccessListOpt {
	return func(o *newAccessListOptions) {
		o.ownerRequires = req
	}
}

func withMemberRequires(req accesslist.Requires) newAccessListOpt {
	return func(o *newAccessListOptions) {
		o.memberRequires = req
	}
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock, opts ...newAccessListOpt) *accesslist.AccessList {
	t.Helper()

	options := newAccessListOptions{
		owners: []accesslist.Owner{
			{
				Name:        "test-user1",
				Description: "test user 1",
			},
			{
				Name:        "test-user2",
				Description: "test user 2",
			},
		},
		memberRequires: accesslist.Requires{
			Roles: []string{"mrole1", "mrole2"},
			Traits: map[string][]string{
				"mtrait1": {"mvalue1", "mvalue2"},
				"mtrait2": {"mvalue3", "mvalue4"},
			},
		},
		ownerRequires: accesslist.Requires{
			Roles: []string{"orole1", "orole2"},
			Traits: map[string][]string{
				"otrait1": {"ovalue1", "ovalue2"},
				"otrait2": {"ovalue3", "ovalue4"},
			},
		},
	}
	for _, o := range opts {
		o(&options)
	}

	audit := accesslist.Audit{}
	if options.typ.IsReviewable() {
		audit.NextAuditDate = clock.Now()
	}

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Type:               options.typ,
			Title:              name + " title",
			Description:        "test access list",
			Owners:             options.owners,
			Audit:              audit,
			MembershipRequires: options.memberRequires,
			OwnershipRequires:  options.ownerRequires,
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}

func createAccessList(t *testing.T, service *AccessListService, name string, clock clockwork.Clock, opts ...newAccessListOpt) *accesslist.AccessList {
	t.Helper()
	ctx := context.Background()
	accessList := newAccessList(t, name, clock, opts...)
	upserted, err := service.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)
	return upserted
}

type accessListMemberOptions struct {
	membershipKind string
	expires        time.Time
}

type accessListMemberOpt func(*accessListMemberOptions)

func withMembershipKind(membershipKind string) accessListMemberOpt {
	return func(o *accessListMemberOptions) {
		o.membershipKind = membershipKind
	}
}

func withExpire(t time.Time) accessListMemberOpt {
	return func(o *accessListMemberOptions) {
		o.expires = t
	}
}

func newAccessListMember(t *testing.T, accessList, name string, opts ...accessListMemberOpt) *accesslist.AccessListMember {
	t.Helper()

	options := accessListMemberOptions{
		expires: time.Now().Add(time.Hour * 24),
	}
	for _, o := range opts {
		o(&options)
	}

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     accessList,
			Name:           name,
			Joined:         time.Now(),
			Expires:        options.expires,
			Reason:         "a reason",
			AddedBy:        "dummy",
			MembershipKind: options.membershipKind,
		},
	)
	require.NoError(t, err)

	return member
}

func newAccessListReview(t *testing.T, accessList, name string) *accesslist.Review {
	t.Helper()

	review, err := accesslist.NewReview(
		header.Metadata{
			Name: name,
		},
		accesslist.ReviewSpec{
			AccessList: accessList,
			Reviewers: []string{
				"user1",
				"user2",
			},
			ReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Notes:      "Some notes",
			Changes: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{
						"role1",
						"role2",
					},
					Traits: trait.Traits{
						"trait1": []string{
							"value1",
							"value2",
						},
						"trait2": []string{
							"value1",
							"value2",
						},
					},
				},
				RemovedMembers: []string{
					"member1",
					"member2",
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
		},
	)
	require.NoError(t, err)

	return review
}

func TestAccessListService_ListAllAccessListMembers(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	const numAccessLists = 10
	const numAccessListMembersPerAccessList = 250
	totalMembers := numAccessLists * numAccessListMembersPerAccessList

	// Create several access lists.
	expectedMembers := make([]*accesslist.AccessListMember, totalMembers)
	for i := range numAccessLists {
		alName := strconv.Itoa(i)
		_, err := service.UpsertAccessList(ctx, newAccessList(t, alName, clock))
		require.NoError(t, err)

		for j := range numAccessListMembersPerAccessList {
			member := newAccessListMember(t, alName, fmt.Sprintf("%03d", j))
			expectedMembers[i*numAccessListMembersPerAccessList+j] = member
			_, err := service.UpsertAccessListMember(ctx, member)
			require.NoError(t, err)
		}
	}

	allMembers := make([]*accesslist.AccessListMember, 0, totalMembers)
	var nextToken string
	for {
		var members []*accesslist.AccessListMember
		var err error
		members, nextToken, err = service.ListAllAccessListMembers(ctx, 0, nextToken)
		require.NoError(t, err)

		allMembers = append(allMembers, members...)

		if nextToken == "" {
			break
		}
	}

	require.Empty(t, cmp.Diff(expectedMembers, allMembers, cmpopts.IgnoreFields(header.Metadata{}, "Revision")))
}

func TestAccessListService_ListAllAccessListReviews(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	const numAccessLists = 10
	const numAccessListReviewsPerAccessList = 250
	totalReviews := numAccessLists * numAccessListReviewsPerAccessList

	// Create several access lists.
	expectedReviews := make([]*accesslist.Review, totalReviews)
	for i := range numAccessLists {
		alName := strconv.Itoa(i)
		_, err := service.UpsertAccessList(ctx, newAccessList(t, alName, clock))
		require.NoError(t, err)

		for j := range numAccessListReviewsPerAccessList {
			review, err := accesslist.NewReview(
				header.Metadata{
					Name: strconv.Itoa(j),
				},
				accesslist.ReviewSpec{
					AccessList: alName,
					Reviewers: []string{
						"user1",
					},
					ReviewDate: time.Now(),
				},
			)
			require.NoError(t, err)
			review, _, err = service.CreateAccessListReview(ctx, review)
			expectedReviews[i*numAccessListReviewsPerAccessList+j] = review
			require.NoError(t, err)
		}
	}

	allReviews := make([]*accesslist.Review, 0, totalReviews)
	var nextToken string
	for {
		var reviews []*accesslist.Review
		var err error
		reviews, nextToken, err = service.ListAllAccessListReviews(ctx, 0, nextToken)
		require.NoError(t, err)

		allReviews = append(allReviews, reviews...)

		if nextToken == "" {
			break
		}
	}

	require.Empty(t, cmp.Diff(expectedReviews, allReviews, cmpopts.IgnoreFields(header.Metadata{}, "Revision"), cmpopts.SortSlices(
		func(r1, r2 *accesslist.Review) bool {
			return r1.GetName() < r2.GetName()
		}),
	))
}

func TestAccessListService_Status_OwnerOf(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	ownerAccessList1 := createAccessList(t, service, "test-owners-acl-"+uuid.NewString(), clock)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), nil)
	ownerAccessList2 := createAccessList(t, service, "test-owners-acl-"+uuid.NewString(), clock)
	requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), nil)

	accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock,
		withOwners([]accesslist.Owner{
			{
				Name:           ownerAccessList1.GetName(),
				MembershipKind: accesslist.MembershipKindList,
			},
		}),
	)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

	ownerAccessList1, _, err = service.UpsertAccessListWithMembers(ctx, ownerAccessList1, nil)
	require.NoError(t, err)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

	ownerAccessList1, err = service.UpsertAccessList(ctx, ownerAccessList1)
	require.NoError(t, err)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

	ownerAccessList1, err = service.UpdateAccessList(ctx, ownerAccessList1)
	require.NoError(t, err)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

	t.Run("origin access list updates and upserts fix status.owner_of of existing list owners", func(t *testing.T) {
		requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

		err = service.updateAccessListOwnerOf(ctx, accessList.GetName(), ownerAccessList1.GetName(), false /* new - this will delete */)
		require.NoError(t, err)
		requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{})

		accessList, err = service.UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
		requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})

		err = service.updateAccessListOwnerOf(ctx, accessList.GetName(), ownerAccessList1.GetName(), false /* new - this will delete */)
		require.NoError(t, err)
		requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{})

		accessList, err = service.UpdateAccessList(ctx, accessList)
		require.NoError(t, err)
		requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), []string{accessList.GetName()})
	})

	t.Run("when list owner is deleted during update or upsert former owners list status.owner_of is updated", func(t *testing.T) {
		requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), nil)

		owner2 := accesslist.Owner{
			Name:           ownerAccessList2.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		}

		accessList.Spec.Owners = append(accessList.Spec.Owners, owner2)
		accessList, err = service.UpsertAccessList(ctx, accessList)
		requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), []string{accessList.GetName()})

		accessList.Spec.Owners = slices.DeleteFunc(accessList.Spec.Owners, func(o accesslist.Owner) bool {
			return o.Name == owner2.Name
		})
		accessList, err = service.UpsertAccessList(ctx, accessList)
		requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), nil)

		accessList.Spec.Owners = append(accessList.Spec.Owners, owner2)
		accessList, err = service.UpdateAccessList(ctx, accessList)
		requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), []string{accessList.GetName()})

		accessList.Spec.Owners = slices.DeleteFunc(accessList.Spec.Owners, func(o accesslist.Owner) bool {
			return o.Name == owner2.Name
		})
		accessList, err = service.UpdateAccessList(ctx, accessList)
		requireStatusOwnerOf(t, service, ownerAccessList2.GetName(), nil)
	})

	err = service.DeleteAccessList(ctx, accessList.GetName())
	require.NoError(t, err)
	requireStatusOwnerOf(t, service, ownerAccessList1.GetName(), nil)
}

func TestAccessListService_Status_MemberOf(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	t.Run("creation for UpsertAccessListMember", func(t *testing.T) {
		accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock)
		nestedAccessList := createAccessList(t, service, "test-nested-acl-"+uuid.NewString(), clock)

		_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t,
			accessList.GetName(),
			nestedAccessList.GetName(),
			withMembershipKind(accesslist.MembershipKindList),
		))
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		err = service.DeleteAccessListMember(ctx, accessList.GetName(), nestedAccessList.GetName())
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), nil)
	})

	t.Run("creation for UpsertAccessListWithMembers", func(t *testing.T) {
		accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock)
		nestedAccessList := createAccessList(t, service, "test-nested-acl-"+uuid.NewString(), clock)

		_, _, err = service.UpsertAccessListWithMembers(
			ctx,
			accessList,
			[]*accesslist.AccessListMember{
				newAccessListMember(t,
					accessList.GetName(),
					nestedAccessList.GetName(),
					withMembershipKind(accesslist.MembershipKindList),
				),
			},
		)
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		_, _, err = service.UpsertAccessListWithMembers(
			ctx,
			accessList,
			[]*accesslist.AccessListMember{
				// delete the member
			},
		)
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), nil)
	})

	t.Run("member updates and upserts do not affect MemberOf", func(t *testing.T) {
		accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock)
		nestedAccessList := createAccessList(t, service, "test-nested-acl-"+uuid.NewString(), clock)

		member, err := service.UpsertAccessListMember(ctx, newAccessListMember(t,
			accessList.GetName(),
			nestedAccessList.GetName(),
			withMembershipKind(accesslist.MembershipKindList),
		))
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		updatedMember, err := service.UpdateAccessListMember(ctx, member)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		_, err = service.UpsertAccessListMember(ctx, updatedMember)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})
	})

	t.Run("member access list updates and upserts do not affect its MemberOf", func(t *testing.T) {
		accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock)
		nestedAccessList := createAccessList(t, service, "test-nested-acl-"+uuid.NewString(), clock)

		_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t,
			accessList.GetName(),
			nestedAccessList.GetName(),
			withMembershipKind(accesslist.MembershipKindList),
		))
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		nestedAccessList, _, err = service.UpsertAccessListWithMembers(ctx, nestedAccessList, nil)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		nestedAccessList, err = service.UpdateAccessList(ctx, nestedAccessList)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		nestedAccessList, err = service.UpsertAccessList(ctx, nestedAccessList)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})
	})

	t.Run("member updates and upserts fix status.member_of of existing members", func(t *testing.T) {
		accessList := createAccessList(t, service, "test-acl-"+uuid.NewString(), clock)
		nestedAccessList := createAccessList(t, service, "test-nested-acl-"+uuid.NewString(), clock)

		member, err := service.UpsertAccessListMember(ctx, newAccessListMember(t,
			accessList.GetName(),
			nestedAccessList.GetName(),
			withMembershipKind(accesslist.MembershipKindList),
		))
		require.NoError(t, err)

		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		err = service.updateAccessListMemberOf(ctx, accessList.GetName(), nestedAccessList.GetName(), false /* new - this will delete */)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{})

		updatedMember, err := service.UpdateAccessListMember(ctx, member)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})

		err = service.updateAccessListMemberOf(ctx, accessList.GetName(), nestedAccessList.GetName(), false /* new - this will delete */)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{})

		_, err = service.UpsertAccessListMember(ctx, updatedMember)
		require.NoError(t, err)
		requireStatusMemberOf(t, service, nestedAccessList.GetName(), []string{accessList.GetName()})
	})
}

func requireStatusOwnerOf(t *testing.T, service *AccessListService, accessListName string, ownerOf []string) {
	t.Helper()
	ctx := context.Background()
	accessList, err := service.GetAccessList(ctx, accessListName)
	require.NoError(t, err)
	slices.Sort(ownerOf)
	slices.Sort(accessList.Status.OwnerOf)
	require.ElementsMatch(t, ownerOf, accessList.Status.OwnerOf)
}

func requireStatusMemberOf(t *testing.T, service *AccessListService, accessListName string, memberOf []string) {
	t.Helper()
	ctx := context.Background()
	accessList, err := service.GetAccessList(ctx, accessListName)
	require.NoError(t, err)
	slices.Sort(memberOf)
	slices.Sort(accessList.Status.MemberOf)
	require.ElementsMatch(t, memberOf, accessList.Status.MemberOf)
}

func newAccessListService(t *testing.T, mem *memory.Memory, clock clockwork.Clock, igsEnabled bool) *AccessListService {
	t.Helper()

	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity:    {Enabled: igsEnabled},
				entitlements.AccessLists: {Enabled: true, Limit: 1},
			},
		},
	})

	service, err := NewAccessListService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	return service
}

func assertMemberOf(t *testing.T, ctx context.Context, svc *AccessListService, name string, expected []string) {
	t.Helper()
	item, err := svc.GetAccessList(ctx, name)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, item.Status.MemberOf)
}
