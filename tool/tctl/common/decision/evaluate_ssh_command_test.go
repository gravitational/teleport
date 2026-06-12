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

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

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
						Logins:                []string{"llama", "beast"},
						ForwardAgent:          true,
						MaxSessionTtl:         durationpb.New(10),
						PortForwarding:        false,
						ClientIdleTimeout:     1000,
						DisconnectExpiredCert: true,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := decision.EvaluateSSHCommand{}

			var output bytes.Buffer
			cmd.Initialize(kingpin.New("tctl", "test").Command("decision", ""), &output)

			clt := fakeDecisionServiceClient{
				sshResponse: test.response,
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
