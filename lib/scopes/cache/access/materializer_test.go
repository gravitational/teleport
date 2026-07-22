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
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
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
	"github.com/gravitational/teleport/lib/scopes"
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
	collection accesslists.ScopedCollection
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
		ScopesFeatures: scopes.Features{
			Enabled: true,
		},
	})
	require.NoError(t, err)

	// Insert the access lists and members into the backend.
	err = aclService.InsertScopedAccessListCollection(t.Context(), &tc.collection)
	require.NoError(t, err, trace.DebugReport(err))

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
	t.Run("unscoped", func(t *testing.T) {
		grandparentName := accesslists.NormalizedSQN{Name: "grandparent"}
		parentName := accesslists.NormalizedSQN{Name: "parent"}
		baseName := accesslists.NormalizedSQN{Name: "base"}
		testMaterializerSimpleChain(t, grandparentName, parentName, baseName)
	})
	t.Run("scoped", func(t *testing.T) {
		grandparentName := accesslists.NormalizedSQN{
			Scope: "/aa/bb/cc",
			Name:  "grandparent",
		}
		parentName := accesslists.NormalizedSQN{
			Scope: "/aa/bb",
			Name:  "parent",
		}
		baseName := accesslists.NormalizedSQN{
			Scope: "/aa",
			Name:  "base",
		}
		testMaterializerSimpleChain(t, grandparentName, parentName, baseName)
	})
}

func testMaterializerSimpleChain(t *testing.T, grandparentName, parentName, baseName accesslists.NormalizedSQN) {
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					grandparentName: newAccessList(t, grandparentName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Role:  "/::grandparentrole",
						Scope: "/aa/bb/cc",
					}})),
					parentName: newAccessList(t, parentName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Role:  "/::parentrole",
						Scope: "/aa/bb",
					}})),
					baseName: newAccessList(t, baseName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Role:  "/::baserole",
						Scope: "/aa",
					}})),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					grandparentName: {
						newAccessListMember(t, grandparentName, parentName),
					},
					parentName: {
						newAccessListMember(t, parentName, baseName),
					},
					baseName: {
						newUserMember(t, baseName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", baseName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::baserole",
					Scope: "/aa",
				}.Build()}),
				expectedScopedRoleAssignment("tester", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::parentrole",
					Scope: "/aa/bb",
				}.Build()}),
				expectedScopedRoleAssignment("tester", grandparentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::grandparentrole",
					Scope: "/aa/bb/cc",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{
				// Delete each membership in the chain one by one, the materialized assignments should be deleted.
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						require.NoError(t, state.aclService.DeleteAccessListMemberV2(t.Context(), accesslistv1.DeleteAccessListMemberRequest_builder{
							AccessList:      grandparentName.Name,
							AccessListScope: grandparentName.Scope,
							MemberName:      parentName.Name,
							MemberScope:     parentName.Scope,
						}.Build()))
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", baseName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::baserole",
							Scope: "/aa",
						}.Build()}),
						expectedScopedRoleAssignment("tester", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::parentrole",
							Scope: "/aa/bb",
						}.Build()}),
					},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						require.NoError(t, state.aclService.DeleteAccessListMemberV2(t.Context(), accesslistv1.DeleteAccessListMemberRequest_builder{
							AccessList:      parentName.Name,
							AccessListScope: parentName.Scope,
							MemberName:      baseName.Name,
							MemberScope:     baseName.Scope,
						}.Build()))
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", baseName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::baserole",
							Scope: "/aa",
						}.Build()}),
					},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						require.NoError(t, state.aclService.DeleteAccessListMemberV2(t.Context(), accesslistv1.DeleteAccessListMemberRequest_builder{
							AccessList:      baseName.Name,
							AccessListScope: baseName.Scope,
							MemberName:      "tester",
						}.Build()))
					},
					expectedAssignments: nil,
				},
			},
		})
	})
}

func TestMaterializerDoubleListMembership(t *testing.T) {
	t.Parallel()
	parentName := accesslists.NormalizedSQN{
		Scope: "/parentscope",
		Name:  "parent",
	}
	memberListAName := accesslists.NormalizedSQN{
		Scope: "/parentscope",
		Name:  "member-list-a",
	}
	memberListBName := accesslists.NormalizedSQN{Name: "memberListB"}
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.ScopedCollection{
			AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
				parentName: newAccessList(t, parentName, withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/parentscope",
					Role:  "/::parentrole",
				}})),
				memberListAName: newAccessList(t, memberListAName),
				memberListBName: newAccessList(t, memberListBName),
			},
			MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
				parentName: {
					newAccessListMember(t, parentName, memberListAName),
					newAccessListMember(t, parentName, memberListBName),
					newUserMember(t, parentName, "testerD"),
				},
				memberListAName: {
					newUserMember(t, memberListAName, "testerA"),
					newUserMember(t, memberListAName, "testerC"),
				},
				memberListBName: {
					newUserMember(t, memberListBName, "testerB"),
					newUserMember(t, memberListBName, "testerC"),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// All users should be direct or nested members of the parent role,
			// expect exactly one materialized assignment each.
			expectedScopedRoleAssignment("testerA", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::parentrole",
				Scope: "/parentscope",
			}.Build()}),
			expectedScopedRoleAssignment("testerB", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::parentrole",
				Scope: "/parentscope",
			}.Build()}),
			expectedScopedRoleAssignment("testerC", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::parentrole",
				Scope: "/parentscope",
			}.Build()}),
			expectedScopedRoleAssignment("testerD", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::parentrole",
				Scope: "/parentscope",
			}.Build()}),
		},
	})
}

func TestMaterializerDoubleListParents(t *testing.T) {
	t.Parallel()
	parentAName := accesslists.NormalizedSQN{Scope: "/aa", Name: "parent-a"}
	parentBName := accesslists.NormalizedSQN{Scope: "/bb", Name: "parent-b"}
	childName := accesslists.NormalizedSQN{Scope: "/", Name: "child"}
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.ScopedCollection{
			AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
				parentAName: newAccessList(t, parentAName, withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "/::rolea",
				}})),
				parentBName: newAccessList(t, parentBName, withMemberGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/bb",
					Role:  "/::roleb",
				}})),
				childName: newAccessList(t, childName),
			},
			MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
				parentAName: {
					newAccessListMember(t, parentAName, childName),
				},
				parentBName: {
					newAccessListMember(t, parentBName, childName),
				},
				childName: {
					newUserMember(t, childName, "tester"),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// User should be a nested member of both parents and get an assignment for each.
			expectedScopedRoleAssignment("tester", parentAName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::rolea",
				Scope: "/aa",
			}.Build()}),
			expectedScopedRoleAssignment("tester", parentBName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::roleb",
				Scope: "/bb",
			}.Build()}),
		},
	})
}

func TestMaterializerDirectOwner(t *testing.T) {
	t.Parallel()
	listName := accesslists.NormalizedSQN{Scope: "/aa", Name: "testlist"}
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.ScopedCollection{
			AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
				listName: newAccessList(t, listName, withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "/::testrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "tester",
					MembershipKind: accesslist.MembershipKindUser,
				}})),
			},
			MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
				listName: {},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// The user is simply a direct owner of the access list and an
			// assignment is expected.
			expectedScopedRoleAssignment("tester", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::testrole",
				Scope: "/aa",
			}.Build()}),
		},
	})
}

func TestMaterializerUserMemberPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	grantedName := accesslists.NormalizedSQN{Scope: "/", Name: "granted"}
	childName := accesslists.NormalizedSQN{Scope: "/", Name: "child"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					grantedName: newAccessList(t, grantedName,
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "/::memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "/::ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					childName: newAccessList(t, childName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					grantedName: {
						newAccessListMember(t, grantedName, childName),
					},
					childName: {},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::ownerrole",
					Scope: "/owner",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessListMember(t.Context(), newUserMember(t, childName, "tester"))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// tester is now both a direct owner of granted and a nested member via child,
					// so the materialized assignment must include both grants.
					expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
						Role:  "/::memberrole",
						Scope: "/member",
					}.Build(), scopedaccessv1.Assignment_builder{
						Role:  "/::ownerrole",
						Scope: "/owner",
					}.Build()}),
				},
			}},
		})
	})
}

func TestMaterializerListMemberPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	grantedName := accesslists.NormalizedSQN{Scope: "/", Name: "granted"}
	parentName := accesslists.NormalizedSQN{Scope: "/", Name: "parent"}
	childName := accesslists.NormalizedSQN{Scope: "/", Name: "child"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					grantedName: newAccessList(t, grantedName,
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "/::memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "/::ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					parentName: newAccessList(t, parentName),
					childName:  newAccessList(t, childName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					grantedName: {
						newAccessListMember(t, grantedName, parentName),
					},
					parentName: {},
					childName: {
						newUserMember(t, childName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::ownerrole",
					Scope: "/owner",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, parentName, childName))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// tester is now both a direct owner of granted and a nested member via
					// parent->child, so the materialized assignment must include both grants.
					expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
						Role:  "/::memberrole",
						Scope: "/member",
					}.Build(), scopedaccessv1.Assignment_builder{
						Role:  "/::ownerrole",
						Scope: "/owner",
					}.Build()}),
				},
			}},
		})
	})
}

func TestMaterializerAccessListPutPreservesExistingOwnerGrant(t *testing.T) {
	t.Parallel()
	grantedName := accesslists.NormalizedSQN{Scope: "/", Name: "granted"}
	childName := accesslists.NormalizedSQN{Name: "child"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					grantedName: newAccessList(t, grantedName,
						withMemberGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/member",
							Role:  "/::memberrole",
						}}),
						withOwnerGrants([]accesslist.ScopedRoleGrant{{
							Scope: "/owner",
							Role:  "/::ownerrole",
						}}),
						withOwners([]accesslist.Owner{{
							Name:           "tester",
							MembershipKind: accesslist.MembershipKindUser,
						}}),
					),
					// child list starts with membership requirements, so any
					// memberships are initially invalid.
					childName: newAccessList(t, childName, withMembershipRequires()),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					grantedName: {
						newAccessListMember(t, grantedName, childName),
					},
					childName: {
						newUserMember(t, childName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// tests is initially ownly an owner of the granted list.
				expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::ownerrole",
					Scope: "/owner",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					// remove the membership requirements so that tester
					// becomes a legitimate member of child and granted lists.
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, childName))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// as tests is now a member and owner of granted, make sure
					// the materialiazed assignment includes both grants.
					expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
						Role:  "/::memberrole",
						Scope: "/member",
					}.Build(), scopedaccessv1.Assignment_builder{
						Role:  "/::ownerrole",
						Scope: "/owner",
					}.Build()}),
				},
			}},
		})
	})
}

func TestMaterializerOwnerRequirements(t *testing.T) {
	t.Parallel()
	grantedName := accesslists.NormalizedSQN{Scope: "/aa", Name: "granted"}
	ownersName := accesslists.NormalizedSQN{Name: "owners"}
	nestedName := accesslists.NormalizedSQN{Name: "nested"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					// Ownership requirements in the owner list should not block
					// _members_ of the owner list from receiving owner grants
					// present in the granted list.
					grantedName: newAccessList(t, grantedName, withOwnerGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "/::ownerrole",
					}}), withOwners([]accesslist.Owner{{
						Name:           "owners",
						MembershipKind: accesslist.MembershipKindList,
					}})),
					ownersName: newAccessList(t, ownersName, withOwnershipRequires()),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					grantedName: {},
					ownersName: {
						newUserMember(t, ownersName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// tester is a valid member of list "owners". All valid members of
				// list "owners" are valid _owners_ of list "granted", so the
				// assignment should be materialized.
				expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::ownerrole",
					Scope: "/aa",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{{
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					// Add a nested list as a member of the owners list.
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, nestedName))
					require.NoError(t, err)
					_, err = state.aclService.UpsertAccessListMember(t.Context(),
						newAccessListMember(t, ownersName, nestedName))
					require.NoError(t, err)
					// Add a user as a member of the nested list.
					_, err = state.aclService.UpsertAccessListMember(t.Context(),
						newUserMember(t, nestedName, "nested_tester"))
					require.NoError(t, err)
				},
				expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
					// The nested user is also a valid member of the owners list so
					// should also have an assignment materialized.
					expectedScopedRoleAssignment("tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
						Role:  "/::ownerrole",
						Scope: "/aa",
					}.Build()}),
					expectedScopedRoleAssignment("nested_tester", grantedName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
						Role:  "/::ownerrole",
						Scope: "/aa",
					}.Build()}),
				},
			}},
		})
	})
}

func TestMaterializerOwnerChain(t *testing.T) {
	t.Parallel()
	grandParentName := accesslists.NormalizedSQN{Scope: "/aa/bb/cc", Name: "grandparent"}
	parentName := accesslists.NormalizedSQN{Scope: "/aa/bb", Name: "parent"}
	baseName := accesslists.NormalizedSQN{Scope: "/aa", Name: "base"}
	runMaterializerTestcase(t, materializerTestcase{
		collection: accesslists.ScopedCollection{
			AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
				grandParentName: newAccessList(t, grandParentName, withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb/cc",
					Role:  "/::grandparentownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           parentName.String(),
					MembershipKind: accesslist.MembershipKindScopedList,
				}})),
				parentName: newAccessList(t, parentName, withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa/bb",
					Role:  "/::parentownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           baseName.String(),
					MembershipKind: accesslist.MembershipKindScopedList,
				}})),
				baseName: newAccessList(t, baseName, withOwnerGrants([]accesslist.ScopedRoleGrant{{
					Scope: "/aa",
					Role:  "/::baseownerrole",
				}}), withOwners([]accesslist.Owner{{
					Name:           "baseowner",
					MembershipKind: accesslist.MembershipKindUser,
				}})),
			},
			MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
				grandParentName: {
					newAccessListMember(t, grandParentName, parentName),
					newUserMember(t, grandParentName, "grandparentmember"),
				},
				parentName: {
					newAccessListMember(t, parentName, baseName),
					newUserMember(t, parentName, "parentmember"),
				},
				baseName: {
					newUserMember(t, baseName, "basemember"),
				},
			},
		},
		expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
			// baseowner is a direct owner of base list. Ownership is not inherited.
			expectedScopedRoleAssignment("baseowner", baseName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::baseownerrole",
				Scope: "/aa",
			}.Build()}),
			// basemember is a member of base list, base list is an owner of parent list.
			expectedScopedRoleAssignment("basemember", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::parentownerrole",
				Scope: "/aa/bb",
			}.Build()}),
			// basemember is a nested member of parent list, parent list is an owner of grandparent list.
			expectedScopedRoleAssignment("basemember", grandParentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::grandparentownerrole",
				Scope: "/aa/bb/cc",
			}.Build()}),
			// parentmember is a member of parent list, parent list is an owner of grandparent list.
			expectedScopedRoleAssignment("parentmember", grandParentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
				Role:  "/::grandparentownerrole",
				Scope: "/aa/bb/cc",
			}.Build()}),
		},
	})
}

func TestMaterializerAccessListPutWithMembershipRequirements(t *testing.T) {
	t.Parallel()
	topName := accesslists.NormalizedSQN{Scope: "/aa", Name: "top"}
	middleName := accesslists.NormalizedSQN{Name: "middle"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					topName: newAccessList(t, topName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "/::toprole",
					}})),
					middleName: newAccessList(t, middleName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					topName: {
						newAccessListMember(t, topName, middleName),
					},
					middleName: {
						newUserMember(t, middleName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::toprole",
					Scope: "/aa",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{{
				// Add membership requires to the middle list, the materialized
				// assignment for the top list should be deleted as the
				// membership path is broken.
				mutateState: func(t *testing.T, state *materializerTestcaseState) {
					_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, middleName, withMembershipRequires()))
					require.NoError(t, err)
				},
				expectedAssignments: nil,
			}},
		})
	})
}

func TestMaterializerAccessListMemberKindUpdateInvalidatesPriorKind(t *testing.T) {
	t.Parallel()
	parentName := accesslists.NormalizedSQN{Scope: "/aa", Name: "parent"}
	childName := accesslists.NormalizedSQN{Name: "child"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					parentName: newAccessList(t, parentName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "/::parentrole",
					}})),
					childName: newAccessList(t, childName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					parentName: {
						newAccessListMember(t, parentName, childName),
					},
					childName: {
						newUserMember(t, childName, "tester"),
					},
				},
			},
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				// Initially tester is a member of list child which is a member
				// of list parent, resulting in a materialized assignment for
				// tester in parent.
				expectedScopedRoleAssignment("tester", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::parentrole",
					Scope: "/aa",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{
				{
					// Update the member resource in place to change the membership kind from list to user.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						member, err := state.aclService.GetAccessListMemberV2(t.Context(), accesslistv1.GetAccessListMemberRequest_builder{
							AccessListScope: parentName.Scope,
							AccessList:      parentName.Name,
							MemberName:      childName.Name,
						}.Build())
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
						expectedScopedRoleAssignment("child", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::parentrole",
							Scope: "/aa",
						}.Build()}),
					},
				},
				{
					// Update the member resource in place again to change it
					// back to a list, and the original assignment should be
					// restored.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						member, err := state.aclService.GetAccessListMemberV2(t.Context(), accesslistv1.GetAccessListMemberRequest_builder{
							AccessListScope: parentName.Scope,
							AccessList:      parentName.Name,
							MemberName:      childName.Name,
						}.Build())
						require.NoError(t, err)

						member.Spec.MembershipKind = accesslist.MembershipKindList
						updatedMember, err := state.aclService.UpdateAccessListMember(t.Context(), member)
						require.NoError(t, err)
						require.Equal(t, accesslist.MembershipKindList, updatedMember.Spec.MembershipKind)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::parentrole",
							Scope: "/aa",
						}.Build()}),
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
	topName := accesslists.NormalizedSQN{Scope: "/aa", Name: "top"}
	leftName := accesslists.NormalizedSQN{Name: "left"}
	rightName := accesslists.NormalizedSQN{Scope: "/aa", Name: "right"}
	bottomName := accesslists.NormalizedSQN{Name: "bottom"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					topName: newAccessList(t, topName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "/::toprole",
					}})),
					leftName:   newAccessList(t, leftName),
					rightName:  newAccessList(t, rightName),
					bottomName: newAccessList(t, bottomName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					topName: {
						newAccessListMember(t, topName, leftName),
						newAccessListMember(t, topName, rightName),
					},
					leftName: {
						newAccessListMember(t, leftName, bottomName),
					},
					rightName: {
						newAccessListMember(t, rightName, bottomName),
					},
					bottomName: {
						newUserMember(t, bottomName, "tester"),
					},
				},
			},
			// Initially the user is a valid member of the top list by 2 paths.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::toprole",
					Scope: "/aa",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{
				{
					// Add membership requires to the left list, the assignment
					// should stay via the right list.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Upsert the access list to remove the membership requires.
						_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, leftName, withMembershipRequires()))
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::toprole",
							Scope: "/aa",
						}.Build()}),
					},
				},
				{
					// Delete the membership path on the right, the assignment
					// should be deleted as there is no more path.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						err := state.aclService.DeleteAccessListMemberV2(t.Context(), accesslistv1.DeleteAccessListMemberRequest_builder{
							AccessListScope: topName.Scope,
							AccessList:      topName.Name,
							MemberScope:     rightName.Scope,
							MemberName:      rightName.Name,
						}.Build())
						require.NoError(t, err)
					},
					expectedAssignments: nil,
				},
				{
					// Remove membership requires from the left list, the assignment
					// should come back.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Upsert the access list to remove the membership requires.
						_, err := state.aclService.UpsertAccessList(t.Context(), newAccessList(t, leftName))
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::toprole",
							Scope: "/aa",
						}.Build()}),
					},
				},
			},
		})
	})
}

func TestMaterializerCascadingMemberExpiries(t *testing.T) {
	t.Parallel()
	listName := accesslists.NormalizedSQN{Scope: "/test", Name: "testlist"}
	synctest.Test(t, func(t *testing.T) {

		testStart := time.Now()

		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					listName: newAccessList(t, listName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test",
						Role:  "/::testrole",
					}})),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					listName: {
						newUserMember(t, listName, "alice", withExpires(testStart.Add(time.Minute))),
						newUserMember(t, listName, "bob", withExpires(testStart.Add(2*time.Minute))),
						newUserMember(t, listName, "charlie", withExpires(testStart.Add(3*time.Minute))),
					},
				},
			},
			// Initially all users are valid members of testlist.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("alice", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::testrole",
					Scope: "/test",
				}.Build()}),
				expectedScopedRoleAssignment("bob", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::testrole",
					Scope: "/test",
				}.Build()}),
				expectedScopedRoleAssignment("charlie", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::testrole",
					Scope: "/test",
				}.Build()}),
			},
			steps: []materializerTestcaseStep{
				{
					// Sleep until alice's membership expires.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("bob", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test",
						}.Build()}),
						expectedScopedRoleAssignment("charlie", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test",
						}.Build()}),
					},
				},
				{
					// Sleep until bob's membership expires.
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						synctest.Wait()
						time.Sleep(time.Minute)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("charlie", listName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test",
						}.Build()}),
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
	parentName := accesslists.NormalizedSQN{Scope: "/test", Name: "parent"}
	childName := accesslists.NormalizedSQN{Scope: "/test", Name: "child"}
	synctest.Test(t, func(t *testing.T) {
		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					parentName: newAccessList(t, parentName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test/parent",
						Role:  "/::testrole",
					}})),
					childName: newAccessList(t, childName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/test/child",
						Role:  "/::testrole",
					}})),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					parentName: {},
					childName:  {},
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
						state.aclService.UpsertAccessListMember(t.Context(), newUserMember(t, childName, "tester"))
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
						expectedScopedRoleAssignment("tester", childName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test/child",
						}.Build()}),
					},
				},
				{
					mutateState: func(t *testing.T, state *materializerTestcaseState) {
						// Break the access list reader again and add the child
						// list as a member of parent list.
						state.breakableACLReader.Break()
						state.aclService.UpsertAccessListMember(t.Context(), newAccessListMember(t, parentName, childName))
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
						expectedScopedRoleAssignment("tester", childName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test/child",
						}.Build()}),
						expectedScopedRoleAssignment("tester", parentName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::testrole",
							Scope: "/test/parent",
						}.Build()}),
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
	topName := accesslists.NormalizedSQN{Scope: "/aa", Name: "top"}
	leftName := accesslists.NormalizedSQN{Scope: "/aa", Name: "left"}
	rightName := accesslists.NormalizedSQN{Scope: "/aa", Name: "right"}
	bottomName := accesslists.NormalizedSQN{Scope: "/aa", Name: "bottom"}
	synctest.Test(t, func(t *testing.T) {

		testStart := time.Now()
		leftExpires := testStart.Add(time.Minute)
		rightExpires := testStart.Add(3 * time.Hour)

		runMaterializerTestcase(t, materializerTestcase{
			collection: accesslists.ScopedCollection{
				AccessListsByName: map[accesslists.NormalizedSQN]*accesslist.AccessList{
					topName: newAccessList(t, topName, withMemberGrants([]accesslist.ScopedRoleGrant{{
						Scope: "/aa",
						Role:  "/::toprole",
					}})),
					leftName:   newAccessList(t, leftName),
					rightName:  newAccessList(t, rightName),
					bottomName: newAccessList(t, bottomName),
				},
				MembersByAccessList: map[accesslists.NormalizedSQN][]*accesslist.AccessListMember{
					topName: {
						newAccessListMember(t, topName, leftName, withExpires(leftExpires)),
						newAccessListMember(t, topName, rightName, withExpires(rightExpires)),
					},
					leftName: {
						newAccessListMember(t, leftName, bottomName),
					},
					rightName: {
						newAccessListMember(t, rightName, bottomName),
					},
					bottomName: {
						newUserMember(t, bottomName, "tester"),
					},
				},
			},
			// Initially the user is a valid member of the top list by 2 paths.
			expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
				expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "/::toprole",
					Scope: "/aa",
				}.Build()}),
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
						expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::toprole",
							Scope: "/aa",
						}.Build()}),
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
						member := newAccessListMember(t, topName, leftName, withExpires(time.Now().Add(time.Minute)))
						_, err := state.aclService.UpsertAccessListMember(t.Context(), member)
						require.NoError(t, err)
					},
					expectedAssignments: []*scopedaccessv1.ScopedRoleAssignment{
						expectedScopedRoleAssignment("tester", topName, []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
							Role:  "/::toprole",
							Scope: "/aa",
						}.Build()}),
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
		// 	// benchmarked this case at 6.2 seconds for a materializer init,
		// 	// vs 3.4 seconds for the baseline case that just copies the
		// 	// assignments with no logic. It materializes 2 million assignments.
		// 	listCount:      100,
		// 	membersPerList: 20000,
		// },
		// {
		// 	// benchmarked this case at 26.6 seconds for a materializer init,
		// 	// vs 14.7 seconds for the baseline case that just copies the
		// 	// assignments with no logic. It materializes 5 million assignments.
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
				ScopesFeatures: scopes.Features{
					Enabled: true,
				},
			})
			require.NoError(b, err)

			t0 := time.Now()

			collection := createBenchmarkCollection(b, tc.listCount, tc.membersPerList, tc.nestingDepth)

			t1 := time.Now()

			// Insert the access lists and members into the backend.
			require.NoError(b, aclService.InsertScopedAccessListCollection(b.Context(), collection))

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
						ListName:  listName.Name,
						ListScope: listName.Scope,
						User:      member.GetName(),
					}
					_, err := assignmentCache.GetScopedRoleAssignment(b.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
						Name:    key.AssignmentName(),
						SubKind: scopedaccess.SubKindMaterialized,
						Scope:   "/",
					}.Build())
					assert.NoError(b, err)
				}
			}
		})
	}
}

func createBenchmarkCollection(b require.TestingT, listCount, membersPerList, nestingDepth int) *accesslists.ScopedCollection {
	var collection accesslists.ScopedCollection

	grants := []accesslist.ScopedRoleGrant{{
		Role:  "/::testrole",
		Scope: "/aa",
	}}
	listIndex := 0
	memberIndex := 0
	for range listCount {
		listName := accesslists.NormalizedSQN{Name: fmt.Sprintf("list-%d", listIndex)}
		listIndex++

		list := newAccessList(b, listName, withMemberGrants(grants))

		members := make([]*accesslist.AccessListMember, membersPerList)
		for i := range membersPerList {
			memberName := fmt.Sprintf("user-%d", listIndex*membersPerList+memberIndex)
			memberIndex++

			members[i] = newUserMember(b, listName, memberName)
		}

		require.NoError(b, collection.AddAccessList(list, members))
	}

	// Each layer of nesting has listCount/2 lists with 2 list members from the previous layer.
	ancestorListCount := listCount
	childListIndex := 0
	for range nestingDepth {
		ancestorListCount = ancestorListCount / 2

		for range ancestorListCount {
			listName := accesslists.NormalizedSQN{Name: fmt.Sprintf("list-%d", listIndex)}
			listIndex++

			list := newAccessList(b, listName, withMemberGrants(grants))

			members := make([]*accesslist.AccessListMember, 2)
			for i := range 2 {
				childListName := accesslists.NormalizedSQN{Name: fmt.Sprintf("list-%d", childListIndex)}
				childListIndex++

				members[i] = newAccessListMember(b, listName, childListName)
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

func newAccessList(t require.TestingT, name accesslists.NormalizedSQN, opts ...aclOption) *accesslist.AccessList {
	list, err := accesslist.NewAccessListWithScope(header.Metadata{
		Name: name.Name,
	}, accesslist.Spec{
		Title: name.String(),
		Owners: []accesslist.Owner{{
			Name:           "testowner",
			MembershipKind: accesslist.MembershipKindUser,
		}},
	}, name.Scope)
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

func withMembershipKind(kind string) memberOption {
	return func(member *accesslist.AccessListMember) {
		member.Spec.MembershipKind = kind
	}
}

func newAccessListMember(t require.TestingT, parent, member accesslists.NormalizedSQN, opts ...memberOption) *accesslist.AccessListMember {
	membershipKind := accesslist.MembershipKindList
	if member.Scope != "" {
		membershipKind = accesslist.MembershipKindScopedList
	}
	memberResource, err := accesslist.NewAccessListMemberWithScope(header.Metadata{
		Name: member.String(),
	}, accesslist.AccessListMemberSpec{
		AccessList:     parent.String(),
		Name:           member.String(),
		MembershipKind: membershipKind,
		Joined:         time.Now(),
		AddedBy:        "testowner",
	}, parent.Scope)
	require.NoError(t, err)
	for _, opt := range opts {
		opt(memberResource)
	}
	return memberResource
}

func newUserMember(t require.TestingT, parent accesslists.NormalizedSQN, userName string, opts ...memberOption) *accesslist.AccessListMember {
	return newAccessListMember(t, parent, accesslists.NormalizedSQN{Name: userName}, append(opts, withMembershipKind(accesslist.MembershipKindUser))...)
}

func expectedScopedRoleAssignment(userName string, listName accesslists.NormalizedSQN, assignments []*scopedaccessv1.Assignment) *scopedaccessv1.ScopedRoleAssignment {
	key := access.MaterializedAssignmentKey{
		User:      userName,
		ListName:  listName.Name,
		ListScope: listName.Scope,
	}
	return scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindMaterialized,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: key.AssignmentName(),
		}.Build(),
		Scope: key.AssignmentScope(),
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User:        userName,
			Assignments: assignments,
		}.Build(),
		Status: scopedaccessv1.ScopedRoleAssignmentStatus_builder{
			Origin: scopedaccessv1.ScopedRoleAssignmentStatus_Origin_builder{
				CreatorKind: scopedaccess.CreatorKindAccessList,
				CreatorName: listName.String(),
			}.Build(),
		}.Build(),
	}.Build()
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

func (b *breakableAccessListReader) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	if b.broken.Load() {
		return nil, "", b.err()
	}
	return b.AccessListReader.ListAccessListsV2(ctx, req)
}

func (b *breakableAccessListReader) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	if b.broken.Load() {
		return nil, b.err()
	}
	return b.AccessListReader.GetAccessListV2(ctx, req)
}

func (b *breakableAccessListReader) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	if b.broken.Load() {
		return nil, "", b.err()
	}
	return b.AccessListReader.ListAccessListMembersV2(ctx, req)
}

func (b *breakableAccessListReader) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	if b.broken.Load() {
		return nil, b.err()
	}
	return b.AccessListReader.GetAccessListMemberV2(ctx, req)
}
