package common

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
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

// WithKind sets the kind.
func WithKind(kind string) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Kind = kind
	}
}

// WithAccessListType sets the access list type.
func WithAccessListType(t accesslist.Type) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.Type = t
	}
}

// WithAuditDate sets a custom audit date.
func WithAuditDate(date time.Time) AccessListOption {
	return func(cfg *AccessListConfig) {
		cfg.AuditDate = date
	}
}

// CreateAccessList creates an access list with members using flexible options.
func CreateAccessList(t *testing.T, sut *SUT, opts ...AccessListOption) *accesslist.AccessList {
	t.Helper()

	cfg := AccessListConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.AuditDate.IsZero() {
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

	var accessListMembers []*accesslist.AccessListMember
	for _, member := range cfg.Members {
		accessListMembers = append(accessListMembers, MustCreateMember(t, accessList.GetName(), member, accesslist.MembershipKindUser))
	}

	_, _, err = sut.Teleport.Process.GetAuthServer().AccessLists.UpsertAccessListWithMembers(t.Context(), accessList, accessListMembers)
	require.NoError(t, err)

	return accessList
}

func MustCreateMember(t *testing.T, aclName, memberName string, memberType string) *accesslist.AccessListMember {
	t.Helper()

	clock := clockwork.NewRealClock()
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     aclName,
			Name:           memberName,
			Joined:         clock.Now(),
			AddedBy:        "added by",
			Expires:        clock.Now().Add(24 * time.Hour).UTC(),
			MembershipKind: memberType,
		},
	)
	require.NoError(t, err)
	return member
}
