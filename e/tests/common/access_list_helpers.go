package common

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// AccessListConfig holds the configurable fields for creating an AccessList.
type AccessListConfig struct {
	Name      string
	Title     string
	Owners    []string
	Grants    accesslist.Grants
	Members   []string
	Kind      string
	SubKind   string
	AuditDate time.Time
	Type      accesslist.Type
	Cleanup   bool
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
