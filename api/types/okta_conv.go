// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	math "math"
	"time"

	"github.com/gravitational/teleport/api/constants"
)

// oktaAssignmentStatusToProto will convert the internal notion of an Okta status into the Okta status
// message understood by protobuf.
//
// Note: To convert OktaAssignmentStatus, use oktaAssignmentResourceStatusToProto.
func OktaAssignmentStatusToProto(status string) OktaAssignmentSpecV1_OktaAssignmentStatus {
	switch status {
	case constants.OktaAssignmentStatusPending:
		return OktaAssignmentSpecV1_PENDING
	case constants.OktaAssignmentStatusProcessing:
		return OktaAssignmentSpecV1_PROCESSING
	case constants.OktaAssignmentStatusSuccessful:
		return OktaAssignmentSpecV1_SUCCESSFUL
	case constants.OktaAssignmentStatusFailed:
		return OktaAssignmentSpecV1_FAILED
	default:
		return OktaAssignmentSpecV1_UNKNOWN
	}
}

// TODO(kopiczko) rename to oktaAssignmentStatusToProto when spec.status is removed.
func oktaAssignmentResourceStatusToProto(status OktaAssignmentStatus) OktaAssignmentStatusV1 {
	return OktaAssignmentStatusV1{
		Phase:       string(status.Phase),
		ProcessedAt: &status.ProcessedAt,
		Targets:     oktaAssignmentStatusTargetsToProto(status.Targets),
	}
}

func oktaAssignmentStatusTargetsToProto(targets OktaAssignmentStatusTargets) *OktaAssignmentStatusTargetsV1 {
	return &OktaAssignmentStatusTargetsV1{
		Stats:  oktaAssignmentStatusTargetsStatsToProto(targets.Stats),
		Status: oktaAssignmentStatusTargetStatusToProto(targets.Status),
	}
}

func oktaAssignmentStatusTargetsStatsToProto(stats OktaAssignmentStatusTargetsStats) *OktaAssignmentStatusTargetsStatsV1 {
	return &OktaAssignmentStatusTargetsStatsV1{
		Total:       int64(stats.Total),
		Provisioned: int64(stats.Provisioned),
		Failed:      int64(stats.Failed),
	}
}

func oktaAssignmentStatusTargetStatusToProto(status []OktaAssignmentStatusTargetStatus) []*OktaAssignmentStatusTargetStatusV1 {
	if status == nil {
		return nil
	}
	out := make([]*OktaAssignmentStatusTargetStatusV1, len(status))
	for i, s := range status {
		out[i] = &OktaAssignmentStatusTargetStatusV1{
			Type:           string(s.Type),
			Id:             s.ID,
			Phase:          string(s.Phase),
			ProcessedAt:    &s.ProcessedAt,
			FailedAttempts: s.FailedAttempts,
		}
	}
	return out
}

// TODO(kopiczko) rename to oktaAssignmentStatusFromProto when spec.status is removed.
func oktaAssignmentResourceStatusFromProto(status OktaAssignmentStatusV1) OktaAssignmentStatus {
	return OktaAssignmentStatus{
		Phase:       OktaAssignmentPhase(status.Phase),
		ProcessedAt: timeToVal(status.ProcessedAt),
		Targets:     oktaAssignmentStatusTargetsFromProto(status.Targets),
	}
}

func oktaAssignmentStatusTargetsFromProto(targets *OktaAssignmentStatusTargetsV1) OktaAssignmentStatusTargets {
	if targets == nil {
		return OktaAssignmentStatusTargets{}
	}
	return OktaAssignmentStatusTargets{
		Stats:  oktaAssignmentStatusTargetsStatsFromProto(targets.Stats),
		Status: oktaAssignmentStatusTargetStatusFromProto(targets.Status),
	}
}

func oktaAssignmentStatusTargetsStatsFromProto(stats *OktaAssignmentStatusTargetsStatsV1) OktaAssignmentStatusTargetsStats {
	if stats == nil {
		return OktaAssignmentStatusTargetsStats{}
	}
	clamp := func(i int64) int { return int(max(math.MinInt, min(math.MaxInt, i))) }
	return OktaAssignmentStatusTargetsStats{
		Total:       clamp(stats.Total),
		Provisioned: clamp(stats.Provisioned),
		Failed:      clamp(stats.Failed),
	}
}

func oktaAssignmentStatusTargetStatusFromProto(status []*OktaAssignmentStatusTargetStatusV1) []OktaAssignmentStatusTargetStatus {
	if status == nil {
		return nil
	}
	out := make([]OktaAssignmentStatusTargetStatus, len(status))
	for i, s := range status {
		if s == nil {
			out[i] = OktaAssignmentStatusTargetStatus{}
			continue
		}
		out[i] = OktaAssignmentStatusTargetStatus{
			Type:           OktaAssignmentTargetType(s.Type),
			ID:             s.Id,
			Phase:          OktaAssignmentTargetPhase(s.Phase),
			ProcessedAt:    timeToVal(s.ProcessedAt),
			FailedAttempts: s.FailedAttempts,
		}
	}
	return out
}

func timeToVal(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}
