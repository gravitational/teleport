// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package decision_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/tool/tctl/common/decision"
)

func TestEvaluateDB(t *testing.T) {
	tests := []struct {
		name     string
		response *decisionpb.EvaluateDatabaseAccessResponse
	}{
		{
			name: "denied",
			response: &decisionpb.EvaluateDatabaseAccessResponse{
				Result: &decisionpb.EvaluateDatabaseAccessResponse_Denial{
					Denial: &decisionpb.DatabaseAccessDenial{
						Metadata: &decisionpb.DenialMetadata{
							PdpVersion:  teleport.Version,
							UserMessage: "denial",
						},
					},
				},
			},
		},
		{
			name: "permitted",
			response: &decisionpb.EvaluateDatabaseAccessResponse{
				Result: &decisionpb.EvaluateDatabaseAccessResponse_Permit{
					Permit: &decisionpb.DatabaseAccessPermit{
						Metadata: &decisionpb.PermitMetadata{
							PdpVersion: teleport.Version,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			cmd := decision.EvaluateDatabaseCommand{
				Output:     &output,
				DatabaseID: "database",
			}

			clt := fakeClient{
				clusterName: "cluster",
				decisionClient: fakeDecisionServiceClient{
					databaseResponse: test.response,
				},
			}

			err := cmd.Run(context.Background(), clt)
			require.NoError(t, err, "evaluating database access failed")

			var expected bytes.Buffer
			err = decision.WriteProtoJSON(&expected, test.response)
			require.NoError(t, err, "marshaling expected output failed")
			require.Equal(t, output.String(), expected.String(), "output did not match")
		})
	}
}
