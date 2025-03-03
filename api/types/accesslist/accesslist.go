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

package accesslist

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/utils"
)

// ReviewFrequency is the review frequency in months.
type ReviewFrequency int

const (
	OneMonth    ReviewFrequency = 1
	ThreeMonths ReviewFrequency = 3
	SixMonths   ReviewFrequency = 6
	OneYear     ReviewFrequency = 12

	twoWeeks = 24 * time.Hour * 14
)

func (r ReviewFrequency) String() string {
	switch r {
	case OneMonth:
		return "1 month"
	case ThreeMonths:
		return "3 months"
	case SixMonths:
		return "6 months"
	case OneYear:
		return "1 year"
	}

	return ""
}

func parseReviewFrequency(input string) ReviewFrequency {
	lowerInput := strings.ReplaceAll(strings.ToLower(input), " ", "")
	switch lowerInput {
	case "1month", "1months", "1m", "1":
		return OneMonth
	case "3month", "3months", "3m", "3":
		return ThreeMonths
	case "6month", "6months", "6m", "6":
		return SixMonths
	case "12month", "12months", "12m", "12", "1years", "1year", "1y":
		return OneYear
	}

	// We won't return an error here and we'll just let CheckAndSetDefaults handle the rest.
	return 0
}

// MaxAllowedDepth is the maximum allowed depth for nested access lists.
const MaxAllowedDepth = 10

var (
	// MembershipKindUnspecified is the default membership kind (treated as 'user').
	MembershipKindUnspecified = accesslistv1.MembershipKind_MEMBERSHIP_KIND_UNSPECIFIED.String()

	// MembershipKindUser is the user membership kind.
	MembershipKindUser = accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String()

	// MembershipKindList is the list membership kind.
	MembershipKindList = accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String()
)

// ReviewDayOfMonth is the day of month the review should be repeated on.
type ReviewDayOfMonth int

const (
	FirstDayOfMonth     ReviewDayOfMonth = 1
	FifteenthDayOfMonth ReviewDayOfMonth = 15
	LastDayOfMonth      ReviewDayOfMonth = 31
)

func (r ReviewDayOfMonth) String() string {
	switch r {
	case FirstDayOfMonth:
		return "1"
	case FifteenthDayOfMonth:
		return "15"
	case LastDayOfMonth:
		return "last"
	}

	return ""
}

func parseReviewDayOfMonth(input string) ReviewDayOfMonth {
	lowerInput := strings.ReplaceAll(strings.ToLower(input), " ", "")
	switch lowerInput {
	case "1", "first":
		return FirstDayOfMonth
	case "15":
		return FifteenthDayOfMonth
	case "last":
		return LastDayOfMonth
	}

	// We won't return an error here and we'll just let CheckAndSetDefaults handle the rest.
	return 0
}

// AccessList describes the basic building block of access grants, which are
// similar to access requests but for longer lived permissions that need to be
// regularly audited.
type AccessList struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access list.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains dynamically calculated fields.
	Status Status `json:"status" yaml:"status"`
}

// Spec is the specification for an access list.
type Spec struct {
	// Title is a plaintext short description of the access list.
	Title string `json:"title" yaml:"title"`

	// Description is an optional plaintext description of the access list.
	Description string `json:"description" yaml:"description"`

	// Owners is a list of owners of the access list.
	Owners []Owner `json:"owners" yaml:"owners"`

	// Audit describes the frequency that this access list must be audited.
	Audit Audit `json:"audit" yaml:"audit"`

	// MembershipRequires describes the requirements for a user to be a member of the access list.
	// For a membership to an access list to be effective, the user must meet the requirements of
	// MembershipRequires and must be in the members list.
	MembershipRequires Requires `json:"membership_requires" yaml:"membership_requires"`

	// OwnershipRequires describes the requirements for a user to be an owner of the access list.
	// For ownership of an access list to be effective, the user must meet the requirements of
	// OwnershipRequires and must be in the owners list.
	OwnershipRequires Requires `json:"ownership_requires" yaml:"ownership_requires"`

	// Grants describes the access granted by membership to this access list.
	Grants Grants `json:"grants" yaml:"grants"`

	// OwnerGrants describes the access granted by ownership of this access list.
	OwnerGrants Grants `json:"owner_grants" yaml:"owner_grants"`
}

// Owner is an owner of an access list.
type Owner struct {
	// Name is the username of the owner.
	Name string `json:"name" yaml:"name"`

	// Description is the plaintext description of the owner and why they are an owner.
	Description string `json:"description" yaml:"description"`

	// IneligibleStatus describes the reason why this owner is not eligible.
	IneligibleStatus string `json:"ineligible_status" yaml:"ineligible_status"`

	// MembershipKind describes the kind of ownership,
	// either "MEMBERSHIP_KIND_USER" or "MEMBERSHIP_KIND_LIST".
	MembershipKind string `json:"membership_kind" yaml:"membership_kind"`
}

// Audit describes the audit configuration for an access list.
type Audit struct {
	// NextAuditDate is the date that the next audit should be performed.
	NextAuditDate time.Time `json:"next_audit_date" yaml:"next_audit_date"`

	// Recurrence is the recurrence definition for auditing. Valid values are
	// 1, first, 15, and last.
	Recurrence Recurrence `json:"recurrence" yaml:"recurrence"`

	// Notifications is the configuration for notifying users.
	Notifications Notifications `json:"notifications" yaml:"notifications"`
}

// Recurrence defines when access list reviews should occur.
type Recurrence struct {
	// Frequency is the frequency between access list reviews.
	Frequency ReviewFrequency `json:"frequency" yaml:"frequency"`

	// DayOfMonth is the day of month subsequent reviews will be scheduled on.
	DayOfMonth ReviewDayOfMonth `json:"day_of_month" yaml:"day_of_month"`
}

// Notifications contains the configuration for notifying users of a nearing next audit date.
type Notifications struct {
	// Start specifies when to start notifying users that the next audit date is coming up.
	Start time.Duration `json:"start" yaml:"start"`
}

// Requires describes a requirement section for an access list. A user must
// meet the following criteria to obtain the specific access to the list.
type Requires struct {
	// Roles are the user roles that must be present for the user to obtain access.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits that must be present for the user to obtain access.
	Traits trait.Traits `json:"traits" yaml:"traits"`
}

// IsEmpty returns true when no roles or traits are set
func (r *Requires) IsEmpty() bool {
	return len(r.Roles) == 0 && len(r.Traits) == 0
}

// Grants describes what access is granted by membership to the access list.
type Grants struct {
	// Roles are the roles that are granted to users who are members of the access list.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits that are granted to users who are members of the access list.
	Traits trait.Traits `json:"traits" yaml:"traits"`
}

// Status contains dynamic fields calculated during retrieval.
type Status struct {
	// MemberCount is the number of members in the access list.
	MemberCount *uint32 `json:"-" yaml:"-"`
	// MemberListCount is the number of members in the access list that are lists themselves.
	MemberListCount *uint32 `json:"-" yaml:"-"`

	// OwnerOf is a list of Access List UUIDs where this access list is an explicit owner.
	OwnerOf []string `json:"owner_of" yaml:"owner_of"`
	// MemberOf is a list of Access List UUIDs where this access list is an explicit member.
	MemberOf []string `json:"member_of" yaml:"member_of"`
}

// NewAccessList will create a new access list.
func NewAccessList(metadata header.Metadata, spec Spec) (*AccessList, error) {
	accessList := &AccessList{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return accessList, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *AccessList) CheckAndSetDefaults() error {
	a.SetKind(types.KindAccessList)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if a.Spec.Title == "" {
		return trace.BadParameter("access list title required")
	}

	if len(a.Spec.Owners) == 0 {
		return trace.BadParameter("owners are missing")
	}

	if a.Spec.Audit.Recurrence.Frequency == 0 {
		a.Spec.Audit.Recurrence.Frequency = SixMonths
	}

	switch a.Spec.Audit.Recurrence.Frequency {
	case OneMonth, ThreeMonths, SixMonths, OneYear:
	default:
		return trace.BadParameter("recurrence frequency is an invalid value")
	}

	if a.Spec.Audit.Recurrence.DayOfMonth == 0 {
		a.Spec.Audit.Recurrence.DayOfMonth = FirstDayOfMonth
	}

	switch a.Spec.Audit.Recurrence.DayOfMonth {
	case FirstDayOfMonth, FifteenthDayOfMonth, LastDayOfMonth:
	default:
		return trace.BadParameter("recurrence day of month is an invalid value")
	}

	if a.Spec.Audit.NextAuditDate.IsZero() {
		a.setInitialAuditDate(clockwork.NewRealClock())
	}

	if a.Spec.Audit.Notifications.Start == 0 {
		a.Spec.Audit.Notifications.Start = twoWeeks
	}

	// Deduplicate owners. The backend will currently prevent this, but it's possible that access lists
	// were created with duplicated owners before the backend checked for duplicate owners. In order to
	// ensure that these access lists are backwards compatible, we'll deduplicate them here.
	ownerMap := make(map[string]struct{}, len(a.Spec.Owners))
	deduplicatedOwners := []Owner{}
	for _, owner := range a.Spec.Owners {
		if owner.Name == "" {
			return trace.BadParameter("owner name is missing")
		}
		if owner.MembershipKind == "" {
			owner.MembershipKind = MembershipKindUser
		}

		if _, ok := ownerMap[owner.Name]; ok {
			continue
		}

		ownerMap[owner.Name] = struct{}{}
		deduplicatedOwners = append(deduplicatedOwners, owner)
	}
	a.Spec.Owners = deduplicatedOwners

	return nil
}

// GetOwners returns the list of owners from the access list.
func (a *AccessList) GetOwners() []Owner {
	return a.Spec.Owners
}

// SetOwners sets the owners of the access list.
func (a *AccessList) SetOwners(owners []Owner) {
	a.Spec.Owners = owners
}

// GetMembershipRequires returns the membership requires configuration from the access list.
func (a *AccessList) GetMembershipRequires() Requires {
	return a.Spec.MembershipRequires
}

// GetOwnershipRequires returns the ownership requires configuration from the access list.
func (a *AccessList) GetOwnershipRequires() Requires {
	return a.Spec.OwnershipRequires
}

// GetGrants returns the grants from the access list.
func (a *AccessList) GetGrants() Grants {
	return a.Spec.Grants
}

// GetOwnerGrants returns the owner grants from the access list.
func (a *AccessList) GetOwnerGrants() Grants {
	return a.Spec.OwnerGrants
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *AccessList) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (a *AccessList) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName())
	return types.MatchSearch(fieldVals, values, nil)
}

// CloneResource returns a copy of the resource as types.ResourceWithLabels.
func (a *AccessList) CloneResource() types.ResourceWithLabels {
	var copy *AccessList
	utils.StrictObjectToStruct(a, &copy)
	return copy
}

func (a *Audit) UnmarshalJSON(data []byte) error {
	type Alias Audit
	audit := struct {
		NextAuditDate string `json:"next_audit_date"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		return trace.Wrap(err)
	}

	if audit.NextAuditDate == "" {
		return nil
	}
	var err error
	a.NextAuditDate, err = time.Parse(time.RFC3339Nano, audit.NextAuditDate)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a Audit) MarshalJSON() ([]byte, error) {
	type Alias Audit
	return json.Marshal(&struct {
		NextAuditDate string `json:"next_audit_date"`
		Alias
	}{
		Alias:         (Alias)(a),
		NextAuditDate: a.NextAuditDate.Format(time.RFC3339Nano),
	})
}

func (r *Recurrence) UnmarshalJSON(data []byte) error {
	type Alias Recurrence
	recurrence := struct {
		Frequency  string `json:"frequency"`
		DayOfMonth string `json:"day_of_month"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &recurrence); err != nil {
		return trace.Wrap(err)
	}

	r.Frequency = parseReviewFrequency(recurrence.Frequency)
	r.DayOfMonth = parseReviewDayOfMonth(recurrence.DayOfMonth)
	return nil
}

func (r Recurrence) MarshalJSON() ([]byte, error) {
	type Alias Recurrence
	return json.Marshal(&struct {
		Frequency  string `json:"frequency"`
		DayOfMonth string `json:"day_of_month"`
		Alias
	}{
		Alias:      (Alias)(r),
		Frequency:  r.Frequency.String(),
		DayOfMonth: r.DayOfMonth.String(),
	})
}

func (n *Notifications) UnmarshalJSON(data []byte) error {
	type Alias Notifications
	notifications := struct {
		Start string `json:"start"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &notifications); err != nil {
		return trace.Wrap(err)
	}

	var err error
	n.Start, err = time.ParseDuration(notifications.Start)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (n Notifications) MarshalJSON() ([]byte, error) {
	type Alias Notifications
	return json.Marshal(&struct {
		Start string `json:"start"`
		Alias
	}{
		Alias: (Alias)(n),
		Start: n.Start.String(),
	})
}

// SelectNextReviewDate will select the next review date for the access list.
func (a *AccessList) SelectNextReviewDate() time.Time {
	numMonths := int(a.Spec.Audit.Recurrence.Frequency)
	dayOfMonth := int(a.Spec.Audit.Recurrence.DayOfMonth)

	// If the last day of the month has been specified, use the 0 day of the
	// next month, which will result in the last day of the target month.
	if dayOfMonth == int(LastDayOfMonth) {
		numMonths += 1
		dayOfMonth = 0
	}

	currentReviewDate := a.Spec.Audit.NextAuditDate
	nextDate := time.Date(currentReviewDate.Year(), currentReviewDate.Month()+time.Month(numMonths), dayOfMonth,
		0, 0, 0, 0, time.UTC)

	return nextDate
}

// setInitialAuditDate sets the NextAuditDate for a newly created AccessList.
// The function is extracted from CheckAndSetDefaults for the sake of testing
// (we need to pass a fake clock).
func (a *AccessList) setInitialAuditDate(clock clockwork.Clock) {
	// We act as if the AccessList just got reviewed (we just created it, so
	// we're pretty sure of what it does) and pick the next review date.
	a.Spec.Audit.NextAuditDate = clock.Now()
	a.Spec.Audit.NextAuditDate = a.SelectNextReviewDate()
}
