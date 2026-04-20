/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accesslist

import (
	"encoding/json"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// Review is an access list review resource.
type Review struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access list review.
	Spec ReviewSpec `json:"spec" yaml:"spec"`
}

const (
	// reviewNotesMaxSizeBytes is the maximum size in bytes of review notes.
	reviewNotesMaxSizeBytes = 200 * 1024 // 200 KB should be more than plenty
)

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
	review := &Review{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := review.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return review, nil
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

	if len(r.Spec.Notes) > reviewNotesMaxSizeBytes {
		r.Spec.Notes = r.Spec.Notes[:reviewNotesMaxSizeBytes]
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
	if a == nil {
		return nil
	}
	out := &Review{}
	deriveDeepCopyReview(out, a)
	return out
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
