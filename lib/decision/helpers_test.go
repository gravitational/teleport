/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package decision

import (
	"testing"

	"github.com/stretchr/testify/require"

	decision "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

func TestUnwrap(t *testing.T) {

	rsp := &decision.EvaluateSSHAccessResponse{
		Decision: &decision.EvaluateSSHAccessResponse_Denial{
			Denial: &decision.SSHAccessDenial{
				Metadata: &decision.DenialMetadata{
					UserMessage: "explicit denial msg",
				},
			},
		},
	}

	permit, err := Unwrap(rsp)
	require.Nil(t, permit)
	require.Error(t, err)
	require.Contains(t, err.Error(), "explicit denial msg")

	rsp = &decision.EvaluateSSHAccessResponse{
		Decision: &decision.EvaluateSSHAccessResponse_Denial{
			Denial: &decision.SSHAccessDenial{},
		},
	}

	permit, err = Unwrap(rsp)
	require.Nil(t, permit)
	require.Error(t, err)
	require.Equal(t, err.Error(), "access denied")

	rsp = &decision.EvaluateSSHAccessResponse{
		Decision: &decision.EvaluateSSHAccessResponse_Permit{
			Permit: &decision.SSHAccessPermit{},
		},
	}

	permit, err = Unwrap(rsp)
	require.NoError(t, err)
	require.NotNil(t, permit)
}
