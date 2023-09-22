/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var _ AccessLists = (*accesslistclient.Client)(nil)

// AccessListsGetter defines an interface for reading access lists.
type AccessListsGetter interface {
	AccessListMembersGetter

	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
	// ListAccessLists returns a paginated list of access lists.
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
}

// AccessLists defines an interface for managing AccessLists.
type AccessLists interface {
	AccessListsGetter
	AccessListMembers

	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	// DeleteAccessList removes the specified access list resource.
	DeleteAccessList(context.Context, string) error
	// DeleteAllAccessLists removes all access lists.
	DeleteAllAccessLists(context.Context) error

	// UpsertAccessListWithMembers creates or updates an access list resource and its members.
	UpsertAccessListWithMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)
}

// MarshalAccessList marshals the access list resource to JSON.
func MarshalAccessList(accessList *accesslist.AccessList, opts ...MarshalOption) ([]byte, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *accessList
		copy.SetResourceID(0)
		accessList = &copy
	}
	return utils.FastMarshal(accessList)
}

// UnmarshalAccessList unmarshals the access list resource from JSON.
func UnmarshalAccessList(data []byte, opts ...MarshalOption) (*accesslist.AccessList, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accessList *accesslist.AccessList
	if err := utils.FastUnmarshal(data, &accessList); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		accessList.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		accessList.SetExpiry(cfg.Expires)
	}
	return accessList, nil
}

// AccessListMembersGetter defines an interface for reading access list members.
type AccessListMembersGetter interface {
	// ListAccessListMembers returns a paginated list of all access list members.
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// GetAccessListMember returns the specified access list member resource.
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
}

// AccessListMembers defines an interface for managing AccessListMembers.
type AccessListMembers interface {
	AccessListMembersGetter

	// UpsertAccessListMember creates or updates an access list member resource.
	UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	// DeleteAccessListMember hard deletes the specified access list member resource.
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
	// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list.
	DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error
	// DeleteAllAccessListMembers hard deletes all access list members.
	DeleteAllAccessListMembers(ctx context.Context) error
}

// MarshalAccessListMember marshals the access list member resource to JSON.
func MarshalAccessListMember(member *accesslist.AccessListMember, opts ...MarshalOption) ([]byte, error) {
	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *member
		copy.SetResourceID(0)
		member = &copy
	}
	return utils.FastMarshal(member)
}

// UnmarshalAccessListMember unmarshals the access list member resource from JSON.
func UnmarshalAccessListMember(data []byte, opts ...MarshalOption) (*accesslist.AccessListMember, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list member data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var member *accesslist.AccessListMember
	if err := utils.FastUnmarshal(data, &member); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		member.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		member.SetExpiry(cfg.Expires)
	}
	return member, nil
}

// IsAccessListOwner will return true if the user is an owner for the current list.
func IsAccessListOwner(identity tlsca.Identity, accessList *accesslist.AccessList) error {
	isOwner := false
	for _, owner := range accessList.Spec.Owners {
		if owner.Name == identity.Username {
			isOwner = true
			break
		}
	}

	// An opaque access denied error.
	accessDenied := trace.AccessDenied("access denied")

	// User is not an owner, so we'll access denied.
	if !isOwner {
		return accessDenied
	}

	if !UserMeetsRequirements(identity, accessList.Spec.OwnershipRequires) {
		return accessDenied
	}

	// We've gotten through all the checks, so the user is an owner.
	return nil
}

// IsAccessListMember will return true if the user is a member for the current list.
func IsAccessListMember(ctx context.Context, identity tlsca.Identity, clock clockwork.Clock, accessList *accesslist.AccessList, memberGetter AccessListMembersGetter) error {
	username := identity.Username

	member, err := memberGetter.GetAccessListMember(ctx, accessList.GetName(), username)
	if trace.IsNotFound(err) {
		// The member has not been found, so we know they're not a member of this list.
		return trace.NotFound("user %s is not a member of the access list", username)
	} else if err != nil {
		// Some other error has occurred
		return trace.Wrap(err)
	}

	expires := member.Spec.Expires
	if expires.IsZero() {
		return nil
	}

	if !clock.Now().Before(expires) {
		return trace.AccessDenied("user %s's membership has expired in the access list", username)
	}

	if !UserMeetsRequirements(identity, accessList.Spec.MembershipRequires) {
		return trace.AccessDenied("user %s is a member, but does not have the roles or traits required to be a member of this list", username)
	}
	return nil
}

// UserMeetsRequirements will return true if the user meets the requirements for the access list.
func UserMeetsRequirements(identity tlsca.Identity, requires accesslist.Requires) bool {
	// Assemble the user's roles for easy look up.
	userRolesMap := map[string]struct{}{}
	for _, role := range identity.Groups {
		userRolesMap[role] = struct{}{}
	}

	// Check that the user meets the role requirements.
	for _, role := range requires.Roles {
		if _, ok := userRolesMap[role]; !ok {
			return false
		}
	}

	// Assemble traits for easy lookyp.
	userTraitsMap := map[string]map[string]struct{}{}
	for k, values := range identity.Traits {
		if _, ok := userTraitsMap[k]; !ok {
			userTraitsMap[k] = map[string]struct{}{}
		}

		for _, v := range values {
			userTraitsMap[k][v] = struct{}{}
		}
	}

	// Check that user meets trait requirements.
	for k, values := range requires.Traits {
		if _, ok := userTraitsMap[k]; !ok {
			return false
		}

		for _, v := range values {
			if _, ok := userTraitsMap[k][v]; !ok {
				return false
			}
		}
	}

	// The user meets all requirements.
	return true
}

// SelectNextAccessListReviewDate will select the next access list review date given the frequency, and a current review date.
//
// This function does the following:
//   - Determines the proper month of the next review by adding the frequency to the current review date with the day of month stripped off.
//     If the resulting date is still in the same year and month, one month will be added, so that the minimum next review date is 1 month
//     past the current review date.
//   - Attempts to maintain the same day of the month unless the day of the month is >= 28, in which case it assumes the
//     intended day of month is the last day of the month. e.g. January 10 + 6 months = July 6, February 28 + 6 months = August 31.
//   - If the frequency added to the current review date with the month stripped off yields a date that is more than halfway through the
//     month, the next month will be selected. e.g. January 1 + frequency = February 27 will be nudged to March. This is done to account for
//     drift due to leap years, leap seconds, etc. Tests indiciate this works up to 200 years.
func SelectNextAccessListReviewDate(frequency time.Duration, reviewDate time.Time) time.Time {
	reviewDateUTC := reviewDate.UTC()

	dayOfMonth := reviewDateUTC.Day()

	// Take the first day of the month of the review date, which we'll use for calculating the next review date.
	calcDate := time.Date(reviewDateUTC.Year(), reviewDateUTC.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Add the frequency to the calc date, which should put us near to the next date.
	targetDate := calcDate.Add(frequency)

	// If we're at the same month as the starting date, go to the next month
	if targetDate.Year() == calcDate.Year() && targetDate.Month() == calcDate.Month() {
		targetDate = targetDate.AddDate(0, 1, 0)
	}

	// Get the first day of the month of the target calc date.
	targetCalcDate := targetDate.AddDate(0, 0, -(targetDate.Day() - 1))

	// If we're over halfway into the month, skip to the next month.
	nextMonth := targetCalcDate.AddDate(0, 1, 0)
	totalDurationInMonth := nextMonth.Sub(targetCalcDate)
	durationBetweenTargetDateAndFirstOfMonth := targetDate.Sub(targetCalcDate)

	if float64(durationBetweenTargetDateAndFirstOfMonth)/float64(totalDurationInMonth) >= 0.5 {
		targetCalcDate = targetCalcDate.AddDate(0, 1, 0)
	}

	// If we're getting towards the end of the month
	if dayOfMonth >= 28 {
		// If we're in this range of days (28-31), we'll assume the user is trying to target the last
		// day of the month. We'll get this by selecting the 0th day of the next month, which will be the last
		// day of the current month. This should handle leap years as well.
		dayOfMonth = time.Date(targetCalcDate.Year(), targetCalcDate.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	}

	return time.Date(targetCalcDate.Year(), targetCalcDate.Month(), dayOfMonth, 0, 0, 0, 0, time.UTC)
}
