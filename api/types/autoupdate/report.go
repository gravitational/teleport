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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAutoUpdateAgentReport creates a new auto update version resource.
func NewAutoUpdateAgentReport(spec *autoupdate.AutoUpdateAgentReportSpec, authName string) (*autoupdate.AutoUpdateAgentReport, error) {
	rollout := &autoupdate.AutoUpdateAgentReport{
		Kind:    types.KindAutoUpdateAgentReport,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: authName,
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

	// TODO: see if we need more validation
	return nil
}
