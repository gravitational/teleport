package ui

import (
	"time"

	"github.com/gravitational/teleport/api/types"
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

	// CurrentUserAssignments describes the current user's ownership and membership status in the access list.
	CurrentUserAssignments *accesslist.CurrentUserAssignments `json:"current_user_assignments"`
	// UserAssignments describes the requested user's ownership and membership assignment types in the access list.
	UserAssignments *accesslist.UserAssignments `json:"user_assignments,omitempty"`
	// UserDisplays contains display values keyed by username.
	UserDisplays map[string]types.UserDisplay `json:"user_displays,omitempty"`
}

// AccessListResponse is a UI representation of an access list response.
type AccessListResponse struct {
	AccessList *AccessList `json:"accessList,omitempty"`
}

// AccessListReview is a UI representation of an Access List review.
type AccessListReview struct {
	*accesslist.Review
	ReviewersInfo []UserInfo `json:"reviewersInfo"`
}

// NewAccessListReview creates a UI Access List review.
func NewAccessListReview(review *accesslist.Review) AccessListReview {
	var displays map[string]types.UserDisplay
	if review.Status != nil {
		displays = review.Status.ReviewerDisplays
	}

	reviewersInfo := make([]UserInfo, 0, len(review.Spec.Reviewers))
	for _, reviewer := range review.Spec.Reviewers {
		reviewersInfo = append(reviewersInfo, newUserInfo(reviewer, displays))
	}

	return AccessListReview{
		Review:        review,
		ReviewersInfo: reviewersInfo,
	}
}

// AccessListReviewsResponse is a UI representation of a response for listing access list reviews.
type AccessListReviewsResponse struct {
	Reviews  []AccessListReview `json:"reviews"`
	StartKey string             `json:"startKey"`
}

// AccessListsResponse is a UI representation of access lists response.
type AccessListsResponse struct {
	AccessLists []*AccessList `json:"accessLists,omitempty"`
	StartKey    string        `json:"startKey"`
	TotalCount  int32         `json:"totalCount"`
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
	NextAuditDate time.Time `json:"nextAuditDate"`
}
