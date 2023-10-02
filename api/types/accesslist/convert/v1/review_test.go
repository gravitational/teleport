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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	traitv1 "github.com/gravitational/teleport/api/types/trait/convert/v1"
)

func TestReviewRoundtrip(t *testing.T) {
	t.Parallel()

	review := newAccessListReview(t, "access-list-review")

	converted, err := FromReviewProto(ToReviewProto(review))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(review, converted))
}

// Make sure that we don't panic if any of the message fields are missing.
func TestReviewFromProtoNils(t *testing.T) {
	t.Parallel()

	// Message is nil
	_, err := FromReviewProto(nil)
	require.Error(t, err)

	// Spec is nil
	review := ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec = nil

	_, err = FromReviewProto(review)
	require.Error(t, err)

	// AccessList is empty
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.AccessList = ""

	_, err = FromReviewProto(review)
	require.Error(t, err)

	// Reviewers is empty
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Reviewers = nil

	_, err = FromReviewProto(review)
	require.Error(t, err)

	// ReviewDate is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.ReviewDate = nil

	_, err = FromReviewProto(review)
	require.Error(t, err)

	// Notes is empty
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Notes = ""

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	// Changes is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Changes = nil

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	// MembershipRequirementsChanged is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Changes.MembershipRequirementsChanged = nil

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	// RemovedMembers is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Changes.RemovedMembers = nil

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	// ReviewFrequencyChanged is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Changes.ReviewFrequencyChanged = 0

	_, err = FromReviewProto(review)
	require.NoError(t, err)

	// ReviewFrequencyDayOfMonth is nil
	review = ToReviewProto(newAccessListReview(t, "access-list-review"))
	review.Spec.Changes.ReviewDayOfMonthChanged = 0

	_, err = FromReviewProto(review)
	require.NoError(t, err)
}

func TestReviewToProtoChanges(t *testing.T) {
	t.Parallel()

	// No changes.
	review := newAccessListReview(t, "access-list-review")
	review.Spec.Changes.MembershipRequirementsChanged = nil
	review.Spec.Changes.RemovedMembers = nil
	review.Spec.Changes.ReviewFrequencyChanged = 0
	review.Spec.Changes.ReviewDayOfMonthChanged = 0

	msg := ToReviewProto(review)
	require.Nil(t, msg.Spec.Changes)

	// Only membership requires changes.
	review = newAccessListReview(t, "access-list-review")
	review.Spec.Changes.RemovedMembers = nil
	review.Spec.Changes.ReviewFrequencyChanged = 0
	review.Spec.Changes.ReviewDayOfMonthChanged = 0

	msg = ToReviewProto(review)
	require.Equal(t, review.Spec.Changes.MembershipRequirementsChanged.Roles, msg.Spec.Changes.MembershipRequirementsChanged.Roles)
	require.Equal(t, review.Spec.Changes.MembershipRequirementsChanged.Traits, traitv1.FromProto(msg.Spec.Changes.MembershipRequirementsChanged.Traits))
	require.Nil(t, msg.Spec.Changes.RemovedMembers)
	require.Zero(t, msg.Spec.Changes.ReviewFrequencyChanged)
	require.Zero(t, msg.Spec.Changes.ReviewDayOfMonthChanged)

	// Only removed members changes.
	review = newAccessListReview(t, "access-list-review")
	review.Spec.Changes.MembershipRequirementsChanged = nil
	review.Spec.Changes.ReviewFrequencyChanged = 0
	review.Spec.Changes.ReviewDayOfMonthChanged = 0

	msg = ToReviewProto(review)
	require.Nil(t, msg.Spec.Changes.MembershipRequirementsChanged)
	require.Equal(t, review.Spec.Changes.RemovedMembers, msg.Spec.Changes.RemovedMembers)
	require.Zero(t, msg.Spec.Changes.ReviewFrequencyChanged)
	require.Zero(t, msg.Spec.Changes.ReviewDayOfMonthChanged)

	// Only review frequency changes.
	review = newAccessListReview(t, "access-list-review")
	review.Spec.Changes.MembershipRequirementsChanged = nil
	review.Spec.Changes.RemovedMembers = nil
	review.Spec.Changes.ReviewDayOfMonthChanged = 0

	msg = ToReviewProto(review)
	require.Nil(t, msg.Spec.Changes.MembershipRequirementsChanged)
	require.Equal(t, review.Spec.Changes.RemovedMembers, msg.Spec.Changes.RemovedMembers)
	require.Equal(t, accesslistv1.ReviewFrequency_REVIEW_FREQUENCY_THREE_MONTHS, msg.Spec.Changes.ReviewFrequencyChanged)
	require.Zero(t, msg.Spec.Changes.ReviewDayOfMonthChanged)

	// Only review day of month changes.
	review = newAccessListReview(t, "access-list-review")
	review.Spec.Changes.MembershipRequirementsChanged = nil
	review.Spec.Changes.RemovedMembers = nil
	review.Spec.Changes.ReviewFrequencyChanged = 0

	msg = ToReviewProto(review)
	require.Nil(t, msg.Spec.Changes.MembershipRequirementsChanged)
	require.Equal(t, review.Spec.Changes.RemovedMembers, msg.Spec.Changes.RemovedMembers)
	require.Zero(t, msg.Spec.Changes.ReviewFrequencyChanged)
	require.Equal(t, accesslistv1.ReviewDayOfMonth_REVIEW_DAY_OF_MONTH_FIFTEENTH, msg.Spec.Changes.ReviewDayOfMonthChanged)
}

func newAccessListReview(t *testing.T, name string) *accesslist.Review {
	t.Helper()

	accessList, err := accesslist.NewReview(
		header.Metadata{
			Name: name,
		},
		accesslist.ReviewSpec{
			AccessList: "access-list",
			Reviewers: []string{
				"reviewer1",
				"reviewer2",
			},
			ReviewDate: time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
			Notes:      "some notes",
			Changes: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{"role1", "role2"},
					Traits: trait.Traits{
						"trait1": []string{"value1"},
						"trait2": []string{"value2"},
					},
				},
				RemovedMembers: []string{
					"removed1",
					"removed2",
					"removed3",
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
		},
	)
	require.NoError(t, err)
	return accessList
}
