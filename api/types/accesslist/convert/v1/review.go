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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	traitv1 "github.com/gravitational/teleport/api/types/trait/convert/v1"
)

// FromReviewProto converts a v1 access list review into an internal access list review object.
func FromReviewProto(msg *accesslistv1.Review) (*accesslist.Review, error) {
	if msg == nil {
		return nil, trace.BadParameter("access list review message is nil")
	}

	if msg.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	// Manually check for the presence of the time so that we can be sure that the review date is
	// zero if the proto message's review date is nil.
	var reviewDate time.Time
	if msg.Spec.ReviewDate != nil {
		reviewDate = msg.Spec.ReviewDate.AsTime()
	}

	var reviewChanges accesslist.ReviewChanges
	if msg.Spec.Changes != nil {
		if msg.Spec.Changes.MembershipRequirementsChanged != nil {
			reviewChanges.MembershipRequirementsChanged = &accesslist.Requires{
				Roles:  msg.Spec.Changes.MembershipRequirementsChanged.Roles,
				Traits: traitv1.FromProto(msg.Spec.Changes.MembershipRequirementsChanged.Traits),
			}
		}
		reviewChanges.RemovedMembers = msg.Spec.Changes.RemovedMembers
		reviewChanges.ReviewFrequencyChanged = accesslist.ReviewFrequency(msg.Spec.Changes.ReviewFrequencyChanged)
		reviewChanges.ReviewDayOfMonthChanged = accesslist.ReviewDayOfMonth(msg.Spec.Changes.ReviewDayOfMonthChanged)
	}

	member, err := accesslist.NewReview(headerv1.FromMetadataProto(msg.Header.Metadata), accesslist.ReviewSpec{
		AccessList: msg.Spec.AccessList,
		Reviewers:  msg.Spec.Reviewers,
		ReviewDate: reviewDate,
		Notes:      msg.Spec.Notes,
		Changes:    reviewChanges,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return member, nil
}

// ToReviewProto converts an internal access list review into a v1 access list review object.
func ToReviewProto(review *accesslist.Review) *accesslistv1.Review {
	var reviewChanges *accesslistv1.ReviewChanges
	if review.Spec.Changes.MembershipRequirementsChanged != nil {
		reviewChanges = &accesslistv1.ReviewChanges{}

		reviewChanges.MembershipRequirementsChanged = &accesslistv1.AccessListRequires{
			Roles:  review.Spec.Changes.MembershipRequirementsChanged.Roles,
			Traits: traitv1.ToProto(review.Spec.Changes.MembershipRequirementsChanged.Traits),
		}
	}
	if len(review.Spec.Changes.RemovedMembers) > 0 {
		if reviewChanges == nil {
			reviewChanges = &accesslistv1.ReviewChanges{}
		}

		reviewChanges.RemovedMembers = review.Spec.Changes.RemovedMembers
	}
	if review.Spec.Changes.ReviewFrequencyChanged > 0 {
		if reviewChanges == nil {
			reviewChanges = &accesslistv1.ReviewChanges{}
		}

		reviewChanges.ReviewFrequencyChanged = accesslistv1.ReviewFrequency(review.Spec.Changes.ReviewFrequencyChanged)
	}
	if review.Spec.Changes.ReviewDayOfMonthChanged > 0 {
		if reviewChanges == nil {
			reviewChanges = &accesslistv1.ReviewChanges{}
		}

		reviewChanges.ReviewDayOfMonthChanged = accesslistv1.ReviewDayOfMonth(review.Spec.Changes.ReviewDayOfMonthChanged)
	}

	return &accesslistv1.Review{
		Header: headerv1.ToResourceHeaderProto(review.ResourceHeader),
		Spec: &accesslistv1.ReviewSpec{
			AccessList: review.Spec.AccessList,
			Reviewers:  review.Spec.Reviewers,
			ReviewDate: timestamppb.New(review.Spec.ReviewDate),
			Notes:      review.Spec.Notes,
			Changes:    reviewChanges,
		},
	}
}
