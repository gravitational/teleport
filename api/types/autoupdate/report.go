/*
Copyright 2025 Gravitational, Inc.

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

package autoupdate

import (
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	autoUpdateAgentReportTTL = time.Hour
	maxGroups                = 50
	maxVersions              = 20
)

// NewAutoUpdateAgentReport creates a new auto update version resource.
func NewAutoUpdateAgentReport(spec *autoupdate.AutoUpdateAgentReportSpec, authName string) (*autoupdate.AutoUpdateAgentReport, error) {
	rollout := &autoupdate.AutoUpdateAgentReport{
		Kind:    types.KindAutoUpdateAgentReport,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: authName,
			// Validate will fail later if timestamp is zero
			Expires: timestamppb.New(spec.GetTimestamp().AsTime().Add(autoUpdateAgentReportTTL)),
		},
		Spec: spec,
	}
	if err := ValidateAutoUpdateAgentReport(rollout); err != nil {
		return nil, trace.Wrap(err)
	}

	return rollout, nil
}

// ValidateAutoUpdateAgentReport checks that required parameters are set
// for the specified AutoUpdateAgentReport.
func ValidateAutoUpdateAgentReport(v *autoupdate.AutoUpdateAgentReport) error {
	if v.GetMetadata().GetName() == "" {
		return trace.BadParameter("Metadata.Name is empty")
	}
	if v.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if ts := v.GetSpec().GetTimestamp(); ts.GetSeconds() == 0 && ts.GetNanos() == 0 {
		return trace.BadParameter("Spec.Timestamp is empty or zero")
	}

	if numGroups := len(v.GetSpec().GetGroups()); numGroups > maxGroups {
		return trace.BadParameter("Spec.Groups is too large (%d while the max is %d)", numGroups, maxGroups)
	}

	for groupName, group := range v.GetSpec().GetGroups() {
		if numVersions := len(group.GetVersions()); numVersions > maxVersions {
			return trace.BadParameter("group %q has too many versions (%d while the max is %d)", groupName, numVersions, maxVersions)
		}
	}

	return nil
}
