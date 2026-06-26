// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package usagereporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestBeamsInferenceRequestEventAnonymize(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer(utils.AnonymizationKeyString("anon-key-or-cluster-id"))
	require.NoError(t, err)

	for _, tt := range []struct {
		desc     string
		event    *BeamsInferenceRequestEvent
		expected *prehogv1a.SubmitEventRequest
	}{
		{
			desc: "success anonymizes beam_id and passes through provider/model/tokens",
			event: &BeamsInferenceRequestEvent{
				BeamId:           "beam-uuid-1234",
				Provider:         "anthropic",
				Model:            "claude-opus-4-5",
				InputTokenCount:  1000,
				OutputTokenCount: 500,
				Success:          true,
			},
			expected: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_BeamsInferenceRequest{
					BeamsInferenceRequest: &prehogv1a.BeamsInferenceRequestEvent{
						BeamId:           anonymizer.AnonymizeString("beam-uuid-1234"),
						Provider:         "anthropic",
						Model:            "claude-opus-4-5",
						InputTokenCount:  1000,
						OutputTokenCount: 500,
						Success:          true,
					},
				},
			},
		},
		{
			desc: "failure has zero output tokens and success=false",
			event: &BeamsInferenceRequestEvent{
				BeamId:          "beam-uuid-1234",
				Provider:        "anthropic",
				Model:           "claude-opus-4-5",
				InputTokenCount: 1000,
				Success:         false,
			},
			expected: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_BeamsInferenceRequest{
					BeamsInferenceRequest: &prehogv1a.BeamsInferenceRequestEvent{
						BeamId:          anonymizer.AnonymizeString("beam-uuid-1234"),
						Provider:        "anthropic",
						Model:           "claude-opus-4-5",
						InputTokenCount: 1000,
						Success:         false,
					},
				},
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.event.Anonymize(anonymizer))
		})
	}
}
