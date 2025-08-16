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

func TestEvaluateSSH(t *testing.T) {
	tests := []struct {
		name     string
		response *decisionpb.EvaluateSSHAccessResponse
	}{
		{
			name: "denied",
			response: &decisionpb.EvaluateSSHAccessResponse{
				Decision: &decisionpb.EvaluateSSHAccessResponse_Denial{
					Denial: &decisionpb.SSHAccessDenial{
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
			response: &decisionpb.EvaluateSSHAccessResponse{
				Decision: &decisionpb.EvaluateSSHAccessResponse_Permit{
					Permit: &decisionpb.SSHAccessPermit{
						Metadata: &decisionpb.PermitMetadata{
							PdpVersion: teleport.Version,
						},
						ForwardAgent:    true,
						X11Forwarding:   true,
						PortForwardMode: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			cmd := decision.EvaluateSSHCommand{
				Output:   &output,
				Username: "alice",
				ServerID: "server",
				Login:    "root",
			}

			clt := fakeClient{
				clusterName: "cluster",
				decisionClient: fakeDecisionServiceClient{
					sshResponse: test.response,
				},
			}

			err := cmd.Run(context.Background(), clt)
			require.NoError(t, err, "evaluating SSH access failed")

			var expected bytes.Buffer
			err = decision.WriteProtoJSON(&expected, test.response)
			require.NoError(t, err, "marshaling expected output failed")
			require.Equal(t, output.String(), expected.String(), "output did not match")
		})
	}
}
