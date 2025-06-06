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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/utils"
)

// Review is an access list review resource.
type Review struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access list review.
	Spec ReviewSpec `json:"spec" yaml:"spec"`
}

// ReviewSpec describes the specification of a review of an access list.
type ReviewSpec struct {
	// AccessList is the name of the associated access list.
	AccessList string `json:"access_list" yaml:"access_list"`

	// Reviewers are the users who performed the review.
	Reviewers []string `json:"reviewers" yaml:"reviewers"`

	// ReviewDate is the date that this review was created.
	ReviewDate time.Time `json:"review_date" yaml:"review_date"`

	// Notes is an optional plaintext attached to the review that can be used by the review for arbitrary
	// note taking on the review.
	Notes string `json:"notes" yaml:"notes"`

	// Changes are the changes made as part of the review.
	Changes ReviewChanges `json:"changes" yaml:"changes"`
}

// ReviewChanges are the changes that were made as part of the review.
type ReviewChanges struct {
	// MembershipRequirementsChanged is populated if the requirements were changed as part of this review.
	MembershipRequirementsChanged *Requires `json:"membership_requirements_changed" yaml:"membership_requirements_changed"`

	// RemovedMembers contains the members that were removed as part of this review.
	RemovedMembers []string `json:"removed_members" yaml:"removed_members"`

	// ReviewFrequencyChanged is populated if the review frequency has changed.
	ReviewFrequencyChanged ReviewFrequency `json:"review_frequency_changed" yaml:"review_frequency_changed"`

	// ReviewDayOfMonthChanged is populated if the review day of month has changed.
	ReviewDayOfMonthChanged ReviewDayOfMonth `json:"review_day_of_month_changed" yaml:"review_day_of_month_changed"`
}

// NewReview will create a new access list review.
func NewReview(metadata header.Metadata, spec ReviewSpec) (*Review, error) {
	member := &Review{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return member, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (r *Review) CheckAndSetDefaults() error {
	r.SetKind(types.KindAccessListReview)
	r.SetVersion(types.V1)

	if err := r.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if r.Spec.AccessList == "" {
		return trace.BadParameter("access list is missing")
	}

	if len(r.Spec.Reviewers) == 0 {
		return trace.BadParameter("reviewers are missing")
	}

	if r.Spec.ReviewDate.IsZero() {
		return trace.BadParameter("review date is missing")
	}

	return nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (r *Review) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(r.Metadata)
}

// Clone returns a copy of the review.
func (a *Review) Clone() *Review {
	var copy *Review
	utils.StrictObjectToStruct(a, &copy)
	return copy
}

func (r *ReviewSpec) UnmarshalJSON(data []byte) error {
	type Alias ReviewSpec
	review := struct {
		ReviewDate string `json:"review_date"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &review); err != nil {
		return trace.Wrap(err)
	}

	var err error
	r.ReviewDate, err = time.Parse(time.RFC3339Nano, review.ReviewDate)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r ReviewSpec) MarshalJSON() ([]byte, error) {
	type Alias ReviewSpec
	return json.Marshal(&struct {
		ReviewDate string `json:"review_date"`
		Alias
	}{
		Alias:      (Alias)(r),
		ReviewDate: r.ReviewDate.Format(time.RFC3339Nano),
	})
}

func (r *ReviewChanges) UnmarshalJSON(data []byte) error {
	type Alias ReviewChanges
	review := struct {
		ReviewFrequencyChanged  string `json:"review_frequency_changed"`
		ReviewDayOfMonthChanged string `json:"review_day_of_month_changed"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &review); err != nil {
		return trace.Wrap(err)
	}
	r.ReviewFrequencyChanged = parseReviewFrequency(review.ReviewFrequencyChanged)
	r.ReviewDayOfMonthChanged = parseReviewDayOfMonth(review.ReviewDayOfMonthChanged)
	return nil
}

func (r ReviewChanges) MarshalJSON() ([]byte, error) {
	type Alias ReviewChanges
	return json.Marshal(&struct {
		ReviewFrequencyChanged  string `json:"review_frequency_changed"`
		ReviewDayOfMonthChanged string `json:"review_day_of_month_changed"`
		Alias
	}{
		Alias:                   (Alias)(r),
		ReviewFrequencyChanged:  r.ReviewFrequencyChanged.String(),
		ReviewDayOfMonthChanged: r.ReviewDayOfMonthChanged.String(),
	})
}
