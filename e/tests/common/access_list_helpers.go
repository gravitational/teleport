package common

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// AccessListConfig holds the configurable fields for creating an AccessList.
type AccessListConfig struct {
	Name   string
	Title  string
	Owners []string
	// ListOwners are owners that are themselves Access Lists (nested ownership),
	// i.e. members of these lists are inherited owners of this list.
	ListOwners []string
	Grants     accesslist.Grants
	Members    []string
	Kind       string
	SubKind    string
	AuditDate  time.Time
	Type       accesslist.Type
	Cleanup    bool
}

// AccessListOption configures an AccessListConfig.
type AccessListOption func(*AccessListConfig)

// WithName sets the AccessList name.
func WithName(name string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Name = name
	}
}

// WithTitle sets the title.
func WithTitle(title string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Title = title
	}
}

// WithOwners sets the owners.
func WithOwners(owners ...string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Owners = owners
	}
}

// WithListOwners sets nested (Access List) owners. Members of these lists are
// inherited owners of the created Access List.
func WithListOwners(owners ...string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.ListOwners = owners
	}
}

// WithGrants sets the grants.
func WithGrants(grants accesslist.Grants) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Grants = grants
	}
}

// WithMembers sets the members.
func WithMembers(members ...string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Members = members
	}
}

// WithAccessListType sets the access list type.
func WithAccessListType(t accesslist.Type) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Type = t
	}
}

// WithCleanup adds a cleanup hook to the current test that will automatically
// delete the Access List at the end of the test.
func WithCleanup(cfg *AccessListConfig) {
	cfg.Cleanup = true
}

// CreateAccessList creates an access list with members using flexible options.
func CreateAccessList(t *testing.T, sut *SUT, opts ...AccessListOption) *accesslist.AccessList {
	t.Helper()

	cfg := AccessListConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.Type.IsReviewable() && cfg.AuditDate.IsZero() {
		cfg.AuditDate = sut.Clock.Now()
	}

	if cfg.Title == "" {
		cfg.Title = cfg.Name
	}

	var accessListOwners []accesslist.Owner
	for _, owner := range cfg.Owners {
		accessListOwners = append(accessListOwners, accesslist.Owner{
			Name:             owner,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindUser,
		})
	}
	for _, owner := range cfg.ListOwners {
		accessListOwners = append(accessListOwners, accesslist.Owner{
			Name:             owner,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindList,
		})
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: cfg.Name,
	}, accesslist.Spec{
		Title:  cfg.Title,
		Owners: accessListOwners,
		Grants: cfg.Grants,
		Audit:  accesslist.Audit{NextAuditDate: cfg.AuditDate},
		Type:   cfg.Type,
	})
	require.NoError(t, err)

	accessList.Kind = cfg.Kind
	accessList.SubKind = cfg.SubKind

	accessListMembers := make([]*accesslist.AccessListMember, 0, len(cfg.Members))
	for _, member := range cfg.Members {
		accessListMembers = append(accessListMembers, NewAccessListMember(t, accessList.GetName(), member, accesslist.MembershipKindUser))
	}

	accessListsSvc := sut.Teleport.Process.GetAuthServer().AccessListsInternal
	createdAccessList, _, err := accessListsSvc.UpsertAccessListWithMembers(t.Context(), accessList, accessListMembers)
	require.NoError(t, err)

	if cfg.Cleanup {
		t.Cleanup(func() {
			require.NoError(t, accessListsSvc.DeleteAccessList(context.Background(), createdAccessList.GetName()))
		})
	}

	return createdAccessList
}

func NewAccessListMember(t *testing.T, aclName, memberName string, memberType string) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     aclName,
			Name:           memberName,
			Joined:         time.Now(),
			AddedBy:        "added by",
			Expires:        time.Now().Add(24 * time.Hour).UTC(),
			MembershipKind: memberType,
		},
	)
	require.NoError(t, err)
	return member
}

func CreateAccessListMember(t *testing.T, sut *SUT, aclName, memberName string, memberType string) *accesslist.AccessListMember {
	t.Helper()
	ctx := t.Context()

	member := NewAccessListMember(t, aclName, memberName, memberType)
	member, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.UpsertAccessListMember(ctx, member)
	require.NoError(t, err)
	return member
}

func GetAccessListMembers(t *testing.T, sut *SUT, aclName string) []*accesslist.AccessListMember {
	getPage := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
		page, nextToken, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.ListAccessListMembers(ctx, aclName, pageSize, pageToken)
		return page, nextToken, trace.Wrap(err)
	}
	results, err := stream.Collect(clientutils.Resources(t.Context(), getPage))
	require.NoError(t, err)
	return results
}

func GetAccessListMemberName(m *accesslist.AccessListMember) string {
	return m.GetName()
}

// AccessListAssertion describes the signature of a function to assert properties
// of an Access List.
type AccessListAssertion func(assert.TestingT, *accesslist.AccessList) bool

// HasTitle is an [AccessListAssertion] that check the Title (a.k.a Display Name)
// of an Access List.
func HasTitle(title string) AccessListAssertion {
	return func(t assert.TestingT, acl *accesslist.AccessList) bool {
		return assert.Equal(t, title, acl.Spec.Title)
	}
}

// AccessListSelector describes the signature of functions used with functions
// like [WaitForAccessLists] and [AssertAccessLists] to accept or reject access
// lists.
type AccessListSelector func(*accesslist.AccessList) bool

// AccessList constructs an [AccessListSelector] from a collection of
// [AccessListAssertion]s.
func AccessList(assertions ...AccessListAssertion) AccessListSelector {
	return func(acl *accesslist.AccessList) bool {
		var collector CollectT
		for _, assertion := range assertions {
			if !assertion(&collector, acl) {
				return false
			}
		}
		return true
	}
}

// WaitForAccessLists monitors the supplied [types.Watcher] for `put` events on
// Access Lists, until all of the supplied selectors are satisfied.
func WaitForAccessLists(t *testing.T, watcher types.Watcher, selectors ...func(*accesslist.AccessList) bool) []*accesslist.AccessList {
	return waitForEvents[*accesslist.AccessList](t, watcher, types.OpPut, selectors...)
}

// AccessListLister allows paged listing of Access List resources
type AccessListLister interface {
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
}

// AssertAccessLists iterates over the supplied selectors, making sure that
// each one is satisfied by an Access List returned by `lister`, and that all
// Access Lists are match. Each selector is only matched once; use multiple
// selectors to handle duplicate access lists.
func AssertAccessLists(t *testing.T, lister AccessListLister, selectors ...func(*accesslist.AccessList) bool) bool {
	t.Helper()

	acls, err := stream.Collect(clientutils.Resources(t.Context(), lister.ListAccessLists))
	if !assert.NoError(t, err, "Listing Access Lists") {
		return false
	}

	for sI, selector := range selectors {
		idx := slices.IndexFunc(acls, selector)
		if !assert.GreaterOrEqual(t, idx, 0, "No ACL matching selector %d", sI) {
			return false
		}
		acls = slices.Delete(acls, idx, idx+1)
	}

	return assert.Empty(t, acls, "Extra Access Lists were not selected")
}

// RequireAccessLists behaves similarly to [AssertAccessLists], but will will
// fail the test immediately if any of rge selectors are not satisfied, or not
// all Access Lists returned by `lister` are selected.
func RequireAccessLists(t *testing.T, lister AccessListLister, selectors ...func(*accesslist.AccessList) bool) {
	t.Helper()
	if AssertAccessLists(t, lister, selectors...) {
		return
	}
	t.FailNow()
}
