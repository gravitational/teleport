// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package access_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/backend/memory"
	cachepkg "github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/observability/tracing"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache/access"
	"github.com/gravitational/teleport/lib/scopes/cache/assignments"
	"github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type materializerTestcase struct {
	// An initial collection of access lists and members for the test case.
	collection accesslists.Collection
	// Expected assignments after cache init (and materialization).
	expectedAssignments []*scopedaccessv1.ScopedRoleAssignment
	// Optional extra mutation steps the test may run.
	steps []materializerTestcaseStep
}

type materializerTestcaseStep struct {
	mutateState         func(t *testing.T, state *materializerTestcaseState)
	expectedAssignments []*scopedaccessv1.ScopedRoleAssignment
}

type materializerTestcaseState struct {
	aclService         *local.AccessListService
	breakableACLReader *breakableAccessListReader
}

func runMaterializerTestcase(t *testing.T, tc materializerTestcase) {
	// Setup prequisites for the cache
	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	defer backend.Close()

	events := local.NewEventsService(backend)
	scopedAccessService := local.NewScopedAccessService(backend)

	aclService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	// Insert the access lists and members into the backend.
	require.NoError(t, aclService.InsertAccessListCollection(t.Context(), &tc.collection))

	// Create the access lists cache.
	aclCache, err := cachepkg.New(cachepkg.Config{
		Context: t.Context(),
		Events:  events,
		Watches: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
		AccessLists: aclService,
	})
	require.NoError(t, err)
	defer aclCache.Close()
	<-aclCache.FirstInit()

	breakableACLReader := &breakableAccessListReader{
		AccessListReader: aclCache,
	}

	// Create the scoped access cache.
	cache, err := access.NewCache(access.CacheConfig{
		Events:           events,
		Reader:           scopedAccessService,
		AccessListEvents: aclCache,
		AccessListReader: breakableACLReader,
	})
	require.NoError(t, err)
	defer cache.Close()
	<-cache.Init()

	state := &materializerTestcaseState{
		aclService:         aclService,
		breakableACLReader: breakableACLReader,
	}

	assertMaterializedAssignments := func(t *testing.T, expectedAssignments []*scopedaccessv1.ScopedRoleAssignment) {
		t.Helper()

		// Assert that all expected assignments were materialized (and there are no extras).
		expectedAssignmentsMap := make(map[string]*scopedaccessv1.ScopedRoleAssignment, len(expectedAssignments))
		for _, assignment := range expectedAssignments {
			expectedAssignmentsMap[assignment.GetMetadata().GetName()] = assignment
		}

		for assignment, err := range utils.RangeScopedRoleAssignments(t.Context(), cache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
			require.NoError(t, err)
			expectedAssignment, ok := expectedAssignmentsMap[assignment.GetMetadata().GetName()]
			if !ok {
				assert.Fail(t, "found unexpected scoped role assignment", "%v", assignment)
				continue
			}
			assert.Empty(t, cmp.Diff(expectedAssignment, assignment, protocmp.Transform()))
			delete(expectedAssignmentsMap, assignment.GetMetadata().GetName())
		}
		for _, expectedAssignment := range expectedAssignmentsMap {
			assert.Fail(t, "expected assignment was not found", "%v", expectedAssignment)
		}
	}

	// Assert the initial materialized assignments.
	assertMaterializedAssignments(t, tc.expectedAssignments)

	// Run through each step and assert the final assignments.
	for i, step := range tc.steps {
		t.Logf("Running step %d", i)
		step.mutateState(t, state)
		synctest.Wait()
		assertMaterializedAssignments(t, step.expectedAssignments)
	}
}

func TestMaterializerSimpleChain(t *testing.T) {
	t.Parallel()
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.Collection{
			AccessListsByName: map[string]*accesslist.AccessList{
				"grandparent": newAccessList(t, "grandparent", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb/cc",
					Role:  "grandparentrole",
				}})),
				"parent": newAccessList(t, "parent", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb",
					Role:  "parentrole",
				}})),
				"base": newAccessList(t, "base", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "baserole",
				}})),
			},
			MembersByAccessList: map[string][]*accesslist.AccessListMember{
				"grandparent": {
					newAccessListMember(t, "grandparent", "parent", accesslist.MembershipKindList),
				},
				"parent": {
					newAccessListMember(t, "parent", "base", accesslist.MembershipKindList),
				},
				"base": {
					newAccessListMember(t, "base", "tester", accesslist.MembershipKindUser),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			expectedScopedRoleAssignment("tester", "base", []*scopedaccessv1.Assignment{{
				Role:  "baserole",
				Scope: "/aa",
			}}),
			expectedScopedRoleAssignment("tester", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentrole",
				Scope: "/aa/bb",
			}}),
			expectedScopedRoleAssignment("tester", "grandparent", []*scopedaccessv1.Assignment{{
				Role:  "grandparentrole",
				Scope: "/aa/bb/cc",
			}}),
		},
	})
}

func TestMaterializerDoubleListMembership(t *testing.T) {
	t.Parallel()
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.Collection{
			AccessListsByName: map[string]*accesslist.AccessList{
				"parent": newAccessList(t, "parent", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/parentscope",
					Role:  "parentrole",
				}})),
				"memberListA": newAccessList(t, "memberListA"),
				"memberListB": newAccessList(t, "memberListB"),
			},
			MembersByAccessList: map[string][]*accesslist.AccessListMember{
				"parent": {
					newAccessListMember(t, "parent", "memberListA", accesslist.MembershipKindList),
					newAccessListMember(t, "parent", "memberListB", accesslist.MembershipKindList),
					newAccessListMember(t, "parent", "testerD", accesslist.MembershipKindUser),
				},
				"memberListA": {
					newAccessListMember(t, "memberListA", "testerA", accesslist.MembershipKindUser),
					newAccessListMember(t, "memberListA", "testerC", accesslist.MembershipKindUser),
				},
				"memberListB": {
					newAccessListMember(t, "memberListB", "testerB", accesslist.MembershipKindUser),
					newAccessListMember(t, "memberListB", "testerC", accesslist.MembershipKindUser),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// All users should be direct or nested members of the parent role,
			// expect exactly one materialized assignment each.
			expectedScopedRoleAssignment("testerA", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentrole",
				Scope: "/parentscope",
			}}),
			expectedScopedRoleAssignment("testerB", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentrole",
				Scope: "/parentscope",
			}}),
			expectedScopedRoleAssignment("testerC", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentrole",
				Scope: "/parentscope",
			}}),
			expectedScopedRoleAssignment("testerD", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentrole",
				Scope: "/parentscope",
			}}),
		},
	})
}

func TestMaterializerDoubleListParents(t *testing.T) {
	t.Parallel()
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.Collection{
			AccessListsByName: map[string]*accesslist.AccessList{
				"parentA": newAccessList(t, "parentA", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "roleA",
				}})),
				"parentB": newAccessList(t, "parentB", withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/bb",
					Role:  "roleB",
				}})),
				"child": newAccessList(t, "child"),
			},
			MembersByAccessList: map[string][]*accesslist.AccessListMember{
				"parentA": {
					newAccessListMember(t, "parentA", "child", accesslist.MembershipKindList),
				},
				"parentB": {
					newAccessListMember(t, "parentB", "child", accesslist.MembershipKindList),
				},
				"child": {
					newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// User should be a nested member of both parents and get an assignment for each.
			expectedScopedRoleAssignment("tester", "parentA", []*scopedaccessv1.Assignment{{
				Role:  "roleA",
				Scope: "/aa",
			}}),
			expectedScopedRoleAssignment("tester", "parentB", []*scopedaccessv1.Assignment{{
				Role:  "roleB",
				Scope: "/bb",
			}}),
		},
	})
}

func TestMaterializerDirectOwner(t *testing.T) {
	t.Parallel()
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.Collection{
			AccessListsByName: map[string]*accesslist.AccessList{
				"testlist": newAccessList(t, "testlist", withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "testrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "tester",
					MembershipKind: accesslist.MembershipKindUser,
				}})),
			},
			MembersByAccessList: map[string][]*accesslist.AccessListMember{
				"testlist": {},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// The user is simply a direct owner of the access list and an
			// assignment is expected.
			expectedScopedRoleAssignment("tester", "testlist", []*scopedaccessv1.Assignment{{
				Role:  "testrole",
				Scope: "/aa",
			}}),
		},
	})
}

func TestMaterializerUserMemberPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"granted": newAccessList(t, "granted",
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					"child": newAccessList(t, "child"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"granted": {
						newAccessListMember(t, "granted", "child", accesslist.MembershipKindList),
					},
					"child": {},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
					Role:  "ownerrole",
					Scope: "/owner",
				}}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// tester is now both a direct owner of granted and a nested member via child,
					// so the materialized assignment must include both grants.
					expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
						Role:  "memberrole",
						Scope: "/member",
					}, {
						Role:  "ownerrole",
						Scope: "/owner",
					}}),
				},
			}},
		})
	})
}

func TestMaterializerListMemberPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"granted": newAccessList(t, "granted",
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					"parent": newAccessList(t, "parent"),
					"child":  newAccessList(t, "child"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"granted": {
						newAccessListMember(t, "granted", "parent", accesslist.MembershipKindList),
					},
					"parent": {},
					"child": {
						newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
					Role:  "ownerrole",
					Scope: "/owner",
				}}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, "parent", "child", accesslist.MembershipKindList))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// tester is now both a direct owner of granted and a nested member via
					// parent->child, so the materialized assignment must include both grants.
					expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
						Role:  "memberrole",
						Scope: "/member",
					}, {
						Role:  "ownerrole",
						Scope: "/owner",
					}}),
				},
			}},
		})
	})
}

func TestMaterializerAccessListPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"granted": newAccessList(t, "granted",
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					// child list starts with membership requirements, so any
					// memberships are initially invalid.
					"child": newAccessList(t, "child", withMembershipRequires()),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"granted": {
						newAccessListMember(t, "granted", "child", accesslist.MembershipKindList),
					},
					"child": {
						newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// tests is initially ownly an owner of the granted list.
				expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
					Role:  "ownerrole",
					Scope: "/owner",
				}}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					// remove the membership requirements so that tester
					// becomes a legitimate member of child and granted lists.
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, "child"))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// as tests is now a member and owner of granted, make sure
					// the materialiazed assignment includes both grants.
					expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
						Role:  "memberrole",
						Scope: "/member",
					}, {
						Role:  "ownerrole",
						Scope: "/owner",
					}}),
				},
			}},
		})
	})
}

func TestMaterializerOwnerRequirements(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					// Ownership requirements in the owner list should not block
					// _members_ of the owner list from receiving owner grants
					// present in the granted list.
					"granted": newAccessList(t, "granted", withOwnerGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "ownerrole",
					}}), withOwners([]accesslist.Owner{{
						Name:           "owners",
						MembershipKind: accesslist.MembershipKindList,
					}})),
					"owners": newAccessList(t, "owners", withOwnershipRequires()),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"granted": {},
					"owners": {
						newAccessListMember(t, "owners", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// tester is a valid member of list "owners". All valid members of
				// list "owners" are valid _owners_ of list "granted", so the
				// assignment should be materialized.
				expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
					Role:  "ownerrole",
					Scope: "/aa",
				}}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					// Add a nested list as a member of the owners list.
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, "nested"))
					require.NoError(t, err)
					_, err = state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, "owners", "nested", accesslist.MembershipKindList))
					require.NoError(t, err)
					// Add a user as a member of the nested list.
					_, err = state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, "nested", "nested_tester", accesslist.MembershipKindUser))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// The nested user is also a valid member of the owners list so
					// should also have an assignment materialized.
					expectedScopedRoleAssignment("tester", "granted", []*scopedaccessv1.Assignment{{
						Role:  "ownerrole",
						Scope: "/aa",
					}}),
					expectedScopedRoleAssignment("nested_tester", "granted", []*scopedaccessv1.Assignment{{
						Role:  "ownerrole",
						Scope: "/aa",
					}}),
				},
			}},
		})
	})
}

func TestMaterializerOwnerChain(t *testing.T) {
	t.Parallel()
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.Collection{
			AccessListsByName: map[string]*accesslist.AccessList{
				"grandparent": newAccessList(t, "grandparent", withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb/cc",
					Role:  "grandparentownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "parent",
					MembershipKind: accesslist.MembershipKindList,
				}})),
				"parent": newAccessList(t, "parent", withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb",
					Role:  "parentownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "base",
					MembershipKind: accesslist.MembershipKindList,
				}})),
				"base": newAccessList(t, "base", withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "baseownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "baseowner",
					MembershipKind: accesslist.MembershipKindUser,
				}})),
			},
			MembersByAccessList: map[string][]*accesslist.AccessListMember{
				"grandparent": {
					newAccessListMember(t, "grandparent", "parent", accesslist.MembershipKindList),
					newAccessListMember(t, "grandparent", "grandparentmember", accesslist.MembershipKindUser),
				},
				"parent": {
					newAccessListMember(t, "parent", "base", accesslist.MembershipKindList),
					newAccessListMember(t, "parent", "parentmember", accesslist.MembershipKindUser),
				},
				"base": {
					newAccessListMember(t, "base", "basemember", accesslist.MembershipKindUser),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// baseowner is a direct owner of base list. Ownership is not inherited.
			expectedScopedRoleAssignment("baseowner", "base", []*scopedaccessv1.Assignment{{
				Role:  "baseownerrole",
				Scope: "/aa",
			}}),
			// basemember is a member of base list, base list is an owner of parent list.
			expectedScopedRoleAssignment("basemember", "parent", []*scopedaccessv1.Assignment{{
				Role:  "parentownerrole",
				Scope: "/aa/bb",
			}}),
			// basemember is a nested member of parent list, parent list is an owner of grandparent list.
			expectedScopedRoleAssignment("basemember", "grandparent", []*scopedaccessv1.Assignment{{
				Role:  "grandparentownerrole",
				Scope: "/aa/bb/cc",
			}}),
			// parentmember is a member of parent list, parent list is an owner of grandparent list.
			expectedScopedRoleAssignment("parentmember", "grandparent", []*scopedaccessv1.Assignment{{
				Role:  "grandparentownerrole",
				Scope: "/aa/bb/cc",
			}}),
		},
	})
}

func TestMaterializerAccessListPutWithMembershipRequirements(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"top": newAccessList(t, "top", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "toprole",
					}})),
					"middle": newAccessList(t, "middle"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"top": {
						newAccessListMember(t, "top", "middle", accesslist.MembershipKindList),
					},
					"middle": {
						newAccessListMember(t, "middle", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
					Role:  "toprole",
					Scope: "/aa",
				}}),
			},
			steps: []materializerTestcaseStep{{
				// Add membership requires to the middle list, the materialized
				// assignment for the top list should be deleted as the
				// membership path is broken.
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, "middle", withMembershipRequires()))
					require.NoError(t, err)
				},
				expectedAssignments: nil,
			}},
		})
	})
}

func TestMaterializerAccessListMemberKindUpdateInvalidatesPriorKind(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"parent": newAccessList(t, "parent", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "parentrole",
					}})),
					"child": newAccessList(t, "child"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"parent": {
						newAccessListMember(t, "parent", "child", accesslist.MembershipKindList),
					},
					"child": {
						newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// Initially tester is a member of list child which is a member
				// of list parent, resulting in a materialized assignment for
				// tester in parent.
				expectedScopedRoleAssignment("tester", "parent", []*scopedaccessv1.Assignment{{
					Role:  "parentrole",
					Scope: "/aa",
				}}),
			},
			steps: []materializerTestcaseStep{
				{
					// Update the member resource in place to change the membership kind from list to user.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						member, err := state.aclService.GetAccessListMember(t.Context(), "parent", "child")
						require.NoError(t, err)

						member.Spec.MembershipKind = accesslist.MembershipKindUser
						updatedMember, err := state.aclService.UpdateAccessListMember(t.Context(), member)
						require.NoError(t, err)
						require.Equal(t, accesslist.MembershipKindUser, updatedMember.Spec.MembershipKind)
					},
					// Now there should be a "user" named child that is a
					// member of the parent list, and tester is no longer a
					// member because the path through the list named child is
					// now broken.
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("child", "parent", []*scopedaccessv1.Assignment{{
							Role:  "parentrole",
							Scope: "/aa",
						}}),
					},
				},
				{
					// Update the member resource in place again to change it
					// back to a list, and the original assignment should be
					// restored.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						member, err := state.aclService.GetAccessListMember(t.Context(), "parent", "child")
						require.NoError(t, err)

						member.Spec.MembershipKind = accesslist.MembershipKindList
						updatedMember, err := state.aclService.UpdateAccessListMember(t.Context(), member)
						require.NoError(t, err)
						require.Equal(t, accesslist.MembershipKindList, updatedMember.Spec.MembershipKind)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "parent", []*scopedaccessv1.Assignment{{
							Role:  "parentrole",
							Scope: "/aa",
						}}),
					},
				},
			},
		})
	})
}

func TestMaterializerDiamond(t *testing.T) {
	// The initial condition will look like this, we'll then break the
	// membership path through left, then break the membership path through
	// right, then restore the membership path through left.
	//
	//       top
	//       / \
	//      /   \
	//     /     \
	//   left   right
	//     \     /
	//      \   /
	//       \ /
	//      bottom
	//        |
	//        v
	//      tester
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"top": newAccessList(t, "top", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "toprole",
					}})),
					"left":   newAccessList(t, "left"),
					"right":  newAccessList(t, "right"),
					"bottom": newAccessList(t, "bottom"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"top": {
						newAccessListMember(t, "top", "left", accesslist.MembershipKindList),
						newAccessListMember(t, "top", "right", accesslist.MembershipKindList),
					},
					"left": {
						newAccessListMember(t, "left", "bottom", accesslist.MembershipKindList),
					},
					"right": {
						newAccessListMember(t, "right", "bottom", accesslist.MembershipKindList),
					},
					"bottom": {
						newAccessListMember(t, "bottom", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			// Initially the user is a valid member of the top list by 2 paths.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
					Role:  "toprole",
					Scope: "/aa",
				}}),
			},
			steps: []materializerTestcaseStep{
				{
					// Add membership requires to the left list, the assignment
					// should stay via the right list.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Upsert the access list to remove the membership requires.
						_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, "left", withMembershipRequires()))
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
							Role:  "toprole",
							Scope: "/aa",
						}}),
					},
				},
				{
					// Delete the membership path on the right, the assignment
					// should be deleted as there is no more path.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						err := state.aclService.DeleteAccessListMember(t.Context(), "top", "right")
						require.NoError(t, err)
					},
					expectedAssignments: nil,
				},
				{
					// Remove membership requires from the left list, the assignment
					// should come back.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Upsert the access list to remove the membership requires.
						_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, "left"))
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
							Role:  "toprole",
							Scope: "/aa",
						}}),
					},
				},
			},
		})
	})
}

func TestMaterializerCascadingMemberExpiries(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {

		testStart := time.Now()

		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"testlist": newAccessList(t, "testlist", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test",
						Role:  "testrole",
					}})),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"testlist": {
						newAccessListMember(t, "testlist", "alice", accesslist.MembershipKindUser, withExpires(testStart.Add(time.Minute))),
						newAccessListMember(t, "testlist", "bob", accesslist.MembershipKindUser, withExpires(testStart.Add(2*time.Minute))),
						newAccessListMember(t, "testlist", "charlie", accesslist.MembershipKindUser, withExpires(testStart.Add(3*time.Minute))),
					},
				},
			},
			// Initially all users are valid members of testlist.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("alice", "testlist", []*scopedaccessv1.Assignment{{
					Role:  "testrole",
					Scope: "/test",
				}}),
				expectedScopedRoleAssignment("bob", "testlist", []*scopedaccessv1.Assignment{{
					Role:  "testrole",
					Scope: "/test",
				}}),
				expectedScopedRoleAssignment("charlie", "testlist", []*scopedaccessv1.Assignment{{
					Role:  "testrole",
					Scope: "/test",
				}}),
			},
			steps: []materializerTestcaseStep{
				{
					// Sleep until alice's membership expires.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("bob", "testlist", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test",
						}}),
						expectedScopedRoleAssignment("charlie", "testlist", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test",
						}}),
					},
				},
				{
					// Sleep until bob's membership expires.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("charlie", "testlist", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test",
						}}),
					},
				},
				{
					// Sleep until charlie's membership expires.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{},
				},
			},
		})
	})
}

func TestMaterializerRepairFailedReads(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"parent": newAccessList(t, "parent", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test/parent",
						Role:  "testrole",
					}})),
					"child": newAccessList(t, "child", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test/child",
						Role:  "testrole",
					}})),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"parent": {},
					"child":  {},
				},
			},
			// Initially there are no members and therefore no assignments.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{},
			steps: []materializerTestcaseStep{
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Add tester as a user member of the child list, but
						// with the access list reader broken, the assignment
						// cannot be validated and should not be materialized.
						state.breakableACLReader.Break()
						state.aclService.UpsertAccessListMember(t.Context(), newAccessListMember(t, "child", "tester", accesslist.MembershipKindUser))
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Fix the access list reader and the assignment should be materialized.
						synctest.Wait()
						state.breakableACLReader.Fix()
						time.Sleep(access.MissedMembersRepairBackoff)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "child", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test/child",
						}}),
					},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Break the access list reader again and add the child
						// list as a member of parent list.
						state.breakableACLReader.Break()
						state.aclService.UpsertAccessListMember(t.Context(), newAccessListMember(t, "parent", "child", accesslist.MembershipKindList))
					},
					// The access list service will update the child list with
					// a Status.MemberOf reference. While handling the access
					// list put event, the materializer will revalidate all
					// materialized assignments for members of the child list.
					// Because the reader is broken, it fails to validate the
					// existing assignment and deletes it.
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Fix the access list reader again and both
						// assignments should be materialized.
						synctest.Wait()
						state.breakableACLReader.Fix()
						time.Sleep(access.MissedMembersRepairBackoff)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "child", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test/child",
						}}),
						expectedScopedRoleAssignment("tester", "parent", []*scopedaccessv1.Assignment{{
							Role:  "testrole",
							Scope: "/test/parent",
						}}),
					},
				},
			},
		})
	})
}

func TestMaterializerDiamondExpiry(t *testing.T) {
	// The initial condition will look like this, we'll then break the
	// membership path through left, then break the membership path through
	// right, then restore the membership path through left.
	//
	//       top
	//       / \
	//      /   \
	//     /     \
	//   left   right
	//     \     /
	//      \   /
	//       \ /
	//      bottom
	//        |
	//        v
	//      tester
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {

		testStart := time.Now()
		leftExpires := testStart.Add(time.Minute)
		rightExpires := testStart.Add(3 * time.Hour)

		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.Collection{
				AccessListsByName: map[string]*accesslist.AccessList{
					"top": newAccessList(t, "top", withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "toprole",
					}})),
					"left":   newAccessList(t, "left"),
					"right":  newAccessList(t, "right"),
					"bottom": newAccessList(t, "bottom"),
				},
				MembersByAccessList: map[string][]*accesslist.AccessListMember{
					"top": {
						newAccessListMember(t, "top", "left", accesslist.MembershipKindList, withExpires(leftExpires)),
						newAccessListMember(t, "top", "right", accesslist.MembershipKindList, withExpires(rightExpires)),
					},
					"left": {
						newAccessListMember(t, "left", "bottom", accesslist.MembershipKindList),
					},
					"right": {
						newAccessListMember(t, "right", "bottom", accesslist.MembershipKindList),
					},
					"bottom": {
						newAccessListMember(t, "bottom", "tester", accesslist.MembershipKindUser),
					},
				},
			},
			// Initially the user is a valid member of the top list by 2 paths.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
					Role:  "toprole",
					Scope: "/aa",
				}}),
			},
			steps: []materializerTestcaseStep{
				{
					// Sleep until the left membership path expires, the
					// assignment should stay via the right list.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
							Role:  "toprole",
							Scope: "/aa",
						}}),
					},
				},
				{
					// Sleep until the right membership path expires, the
					// assignment should be deleted as there is no more path.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(3 * time.Hour)
					},
					expectedAssignments: nil,
				},
				{
					// Upsert the left membership with a future expiry, the assignment should come back.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						member := newAccessListMember(t, "top", "left", accesslist.MembershipKindList, withExpires(time.Now().Add(time.Minute)))
						_, err := state.aclService.UpsertAccessListMember(t.Context(), member)
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", "top", []*scopedaccessv1.Assignment{{
							Role:  "toprole",
							Scope: "/aa",
						}}),
					},
				},
			},
		})
	})
}

func BenchmarkMaterializerInit(b *testing.B) {
	for _, tc := range []struct {
		listCount      int
		membersPerList int
		nestingDepth   int
	}{
		{
			listCount:      1000,
			membersPerList: 100,
		},
		{
			listCount:      100,
			membersPerList: 1000,
		},
		{
			listCount:      100,
			membersPerList: 1000,
			// Each list will have 1 parent lists, meaning each of 100k users
			// are members of 2 lists, resulting in 200k materialized assignments.
			nestingDepth: 1,
		},
		{
			listCount:      100,
			membersPerList: 1000,
			// Each list will have 2 parent lists, meaning each of 100k users
			// are members of 3 lists, resulting in 300k materialized assignments.
			nestingDepth: 2,
		},
		// These cases are theoretically realistic for very large clusters, but pretty slow to run in CI.
		// {
		//  // benchmarked this case at 6.2 seconds for a materializer init,
		//  // vs 3.4 seconds for the baseline case that just copies the
		//  // assignments with no logic. It materializes 2 million assignments.
		// 	listCount:      100,
		// 	membersPerList: 20000,
		// },
		// {
		//  // benchmarked this case at 26.6 seconds for a materializer init,
		//  // vs 14.7 seconds for the baseline case that just copies the
		//  // assignments with no logic. It materializes 5 million assignments.
		// 	listCount:      25000,
		// 	membersPerList: 200,
		// },
	} {
		b.Run(fmt.Sprintf("lists=%d,members=%d,depth=%d", tc.listCount, tc.membersPerList, tc.nestingDepth), func(b *testing.B) {
			// Setup prequisites for the cache
			backend, err := memory.New(memory.Config{
				Context: b.Context(),
			})
			require.NoError(b, err)
			defer backend.Close()

			aclService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
				Backend: backend,
				Modules: modulestest.EnterpriseModules(),
			})
			require.NoError(b, err)

			t0 := time.Now()

			collection := createBenchmarkCollection(b, tc.listCount, tc.membersPerList, tc.nestingDepth)

			t1 := time.Now()

			// Insert the access lists and members into the backend.
			require.NoError(b, aclService.InsertAccessListCollection(b.Context(), collection))

			t2 := time.Now()

			// Create and init the access lists cache.
			aclCache, err := cachepkg.New(cachepkg.Config{
				Context: b.Context(),
				Events:  local.NewEventsService(backend),
				Watches: []types.WatchKind{
					{Kind: types.KindAccessList},
					{Kind: types.KindAccessListMember},
				},
				AccessLists: aclService,
			})
			require.NoError(b, err)
			defer aclCache.Close()
			<-aclCache.FirstInit()

			t3 := time.Now()

			b.Logf(`Setup timing:
creating the input:     %v
inserting to backend:   %v
initializing acl cache: %v`, t1.Sub(t0), t2.Sub(t1), t3.Sub(t2))

			var assignmentCache *assignments.AssignmentCache

			b.Run("init", func(b *testing.B) {
				// Initialize a new materializer with a new assignment state
				// for each benchmark loop.
				for b.Loop() {
					assignmentCache = assignments.NewAssignmentCache(assignments.AssignmentCacheConfig{})
					materializer := access.NewMaterializer(assignmentCache, aclCache, tracing.NoopTracer("test"))
					require.NoError(b, materializer.Init(b.Context()))
				}
			})

			b.Logf("Materialized %d assignments", assignmentCache.Len())

			// How long does it take to literally just copy every already
			// materialized assignment into a new assignment cache. This is a
			// nice baseline for how fast it's physically possible to go.
			b.Run("init baseline", func(b *testing.B) {
				for b.Loop() {
					clonedCache := assignments.NewAssignmentCache(assignments.AssignmentCacheConfig{})
					for assignment, err := range utils.RangeScopedRoleAssignments(b.Context(), assignmentCache, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
						require.NoError(b, err)
						clonedCache.Put(assignment)
					}
				}
			})

			// Assert there's an assignment for every direct member. This
			// doesn't cover nested members, it's mostly a smoke test to make
			// sure the benchmark is actually doing something.
			for listName, members := range collection.MembersByAccessList {
				for _, member := range members {
					if !member.IsUser() {
						continue
					}
					key := access.MaterializedAssignmentKey{
						List: listName,
						User: member.GetName(),
					}
					_, err := assignmentCache.GetScopedRoleAssignment(b.Context(), &scopedaccessv1.GetScopedRoleAssignmentRequest{
						Name:    key.AssignmentName(),
						SubKind: scopedaccess.SubKindMaterialized,
					})
					assert.NoError(b, err)
				}
			}
		})
	}
}

func createBenchmarkCollection(b require.TestingT, listCount, membersPerList, nestingDepth int) *accesslists.Collection {
	var collection accesslists.Collection

	grants := []accesslist.ScopedRoleGrant{{
		Role:  "testrole",
		Scope: "/aa",
	}}
	listIndex := 0
	memberIndex := 0
	for range listCount {
		listName := fmt.Sprintf("list-%d", listIndex)
		listIndex++

		list := newAccessList(b, listName, withMemberGrants(grants))

		members := make([]*accesslist.AccessListMember, membersPerList)
		for i := range membersPerList {
			memberName := fmt.Sprintf("user-%d", listIndex*membersPerList+memberIndex)
			memberIndex++

			members[i] = newAccessListMember(b, listName, memberName, accesslist.MembershipKindUser)
		}

		require.NoError(b, collection.AddAccessList(list, members))
	}

	// Each layer of nesting has listCount/2 lists with 2 list members from the previous layer.
	ancestorListCount := listCount
	childListIndex := 0
	for range nestingDepth {
		ancestorListCount = ancestorListCount / 2

		for range ancestorListCount {
			listName := fmt.Sprintf("list-%d", listIndex)
			listIndex++

			list := newAccessList(b, listName, withMemberGrants(grants))

			members := make([]*accesslist.AccessListMember, 2)
			for i := range 2 {
				childListName := fmt.Sprintf("list-%d", childListIndex)
				childListIndex++

				members[i] = newAccessListMember(b, listName, childListName, accesslist.MembershipKindList)
			}

			require.NoError(b, collection.AddAccessList(list, members))
		}
	}
	return &collection
}

type aclOption func(*accesslist.AccessList)

func withMemberGrants(grants []accesslist.ScopedRoleGrant) aclOption {
	return func(list *accesslist.AccessList) {
		list.Spec.Grants.ScopedRoles = grants
	}
}

func withOwnerGrants(grants []accesslist.ScopedRoleGrant) aclOption {
	return func(list *accesslist.AccessList) {
		list.Spec.OwnerGrants.ScopedRoles = grants
	}
}

func withOwners(owners []accesslist.Owner) aclOption {
	return func(list *accesslist.AccessList) {
		list.Spec.Owners = owners
	}
}

func withMembershipRequires() aclOption {
	return func(list *accesslist.AccessList) {
		list.Spec.MembershipRequires.Roles = []string{"access"}
	}
}
func withOwnershipRequires() aclOption {
	return func(list *accesslist.AccessList) {
		list.Spec.OwnershipRequires.Roles = []string{"access"}
	}
}

func newAccessList(t require.TestingT, name string, opts ...aclOption) *accesslist.AccessList {
	list, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: name,
		Owners: []accesslist.Owner{{
			Name:           "testowner",
			MembershipKind: accesslist.MembershipKindUser,
		}},
	})
	require.NoError(t, err)
	for _, opt := range opts {
		opt(list)
	}
	return list
}

type memberOption func(*accesslist.AccessListMember)

func withExpires(expires time.Time) memberOption {
	return func(member *accesslist.AccessListMember) {
		member.Spec.Expires = expires
	}
}

func newAccessListMember(t require.TestingT, parent, member, membershipKind string, opts ...memberOption) *accesslist.AccessListMember {
	memberResource, err := accesslist.NewAccessListMember(header.Metadata{
		Name: member,
	}, accesslist.AccessListMemberSpec{
		AccessList:     parent,
		Name:           member,
		MembershipKind: membershipKind,
		Joined:         time.Now(),
		AddedBy:        "testowner",
	})
	require.NoError(t, err)
	for _, opt := range opts {
		opt(memberResource)
	}
	return memberResource
}

func expectedScopedRoleAssignment(userName, listName string, assignments []*scopedaccessv1.Assignment) *scopedaccessv1.ScopedRoleAssignment {
	key := access.MaterializedAssignmentKey{
		User: userName,
		List: listName,
	}
	return &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindMaterialized,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: key.AssignmentName(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User:        userName,
			Assignments: assignments,
		},
		Status: &scopedaccessv1.ScopedRoleAssignmentStatus{
			Origin: &scopedaccessv1.ScopedRoleAssignmentStatus_Origin{
				CreatorKind: scopedaccess.CreatorKindAccessList,
				CreatorName: listName,
			},
		},
	}
}

type breakableAccessListReader struct {
	access.AccessListReader

	broken atomic.Bool
}

func (b *breakableAccessListReader) Break() {
	b.broken.Store(true)
}

func (b *breakableAccessListReader) Fix() {
	b.broken.Store(false)
}

func (b *breakableAccessListReader) err() error {
	return fmt.Errorf("access list reader is broken")
}

func (b *breakableAccessListReader) ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
	if b.broken.Load() {
		return nil, "", b.err()
	}
	return b.AccessListReader.ListAccessLists(ctx, pageSize, pageToken)
}

func (b *breakableAccessListReader) GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error) {
	if b.broken.Load() {
		return nil, b.err()
	}
	return b.AccessListReader.GetAccessList(ctx, accessListName)
}

func (b *breakableAccessListReader) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	if b.broken.Load() {
		return nil, "", b.err()
	}
	return b.AccessListReader.ListAllAccessListMembers(ctx, pageSize, pageToken)
}

func (b *breakableAccessListReader) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	if b.broken.Load() {
		return nil, "", b.err()
	}
	return b.AccessListReader.ListAccessListMembers(ctx, accessListName, pageSize, pageToken)
}

func (b *breakableAccessListReader) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	if b.broken.Load() {
		return nil, b.err()
	}
	return b.AccessListReader.GetAccessListMember(ctx, accessList, memberName)
}
