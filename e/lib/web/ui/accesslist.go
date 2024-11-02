package ui

import (
	"time"

	"github.com/gravitational/teleport/api/types/accesslist"
)

// AccessList is a UI representation of an access list.
type AccessList struct {
	*accesslist.AccessList

	// Members is a list of users and roles that are allowed to access the cluster.
	Members []accesslist.AccessListMemberSpec `json:"members,omitempty"`
	// MembersCount is the number of members in the access list.
	// A `nil` membersCount means the caller did not have access
	// to list members (eg: a user can read access list that
	// they are a member of but not other members).
	MembersCount *uint32 `json:"membersCount"`
	// MemberListCount is the number of members of list type in the access list.
	MemberListCount *uint32 `json:"memberListCount"`

	// InheritedMemberGrants is a list of inherited member grants from nested access lists.
	InheritedMemberGrants accesslist.Grants `json:"inherited_member_grants"`
}

// AccessListResponse is a UI representation of an access list response.
type AccessListResponse struct {
	AccessList *AccessList `json:"accessList,omitempty"`
}

// AccessListsResponse is a UI representation of access lists response.
type AccessListsResponse struct {
	AccessLists []*AccessList `json:"accessLists,omitempty"`
}

// UpsertAccessListRequest is a UI representation of an upsert access list request.
type UpsertAccessListRequest struct {
	accesslist.Spec
	// Members is a list of users and roles that are allowed to access the cluster.
	Members []accesslist.AccessListMemberSpec `json:"members,omitempty"`
}

// AddAccessListMemberRequest is a UI representation of an add access list member request.
type AddAccessListMemberRequest struct {
	Members []accesslist.AccessListMemberSpec `json:"members,omitempty"`
}

// AddAccessListMemberResponse is a UI representation of an add access list member response.
type AddAccessListMemberResponse struct {
	Members []accesslist.AccessListMemberSpec `json:"members,omitempty"`
}

// ReviewAccessListRequest is a UI representation for reviewing an access list request.
type ReviewAccessListRequest struct {
	accesslist.ReviewSpec
}

// ReviewAccessList is a UI representation for reviewing an access list request.
type ReviewAccessListResponse struct {
	NextAuditDate time.Time `json:"nextAuditDate,omitempty"`
}
