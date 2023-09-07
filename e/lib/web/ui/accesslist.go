package ui

import "github.com/gravitational/teleport/api/types/accesslist"

// AccessList is a UI representation of an access list.
type AccessList struct {
	*accesslist.AccessList

	// Members is a list of users and roles that are allowed to access the cluster.
	Members []accesslist.AccessListMemberSpec `json:"members,omitempty"`
	// MembersCount is the number of members in the access list.
	MembersCount *int `json:"membersCount,omitempty"`
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
