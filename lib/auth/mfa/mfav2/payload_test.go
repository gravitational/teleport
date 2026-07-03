/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package mfav2

import (
	"testing"

	"github.com/stretchr/testify/require"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
)

func TestCheckPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		sip        *mfav2.SessionIdentifyingPayload
		wantErrMsg string
	}{
		{
			desc: "ssh session id accepted",
			sip: mfav2.SessionIdentifyingPayload_builder{
				SshSessionId: []byte("ssh-session-id"),
			}.Build(),
		},
		{
			desc: "kube client cert fingerprint accepted",
			sip: mfav2.SessionIdentifyingPayload_builder{
				KubeClientCertFingerprint: []byte("kube-cert-fingerprint"),
			}.Build(),
		},
		{
			desc:       "nil payload rejected",
			wantErrMsg: "missing SessionIdentifyingPayload",
		},
		{
			desc:       "unset payload value rejected",
			sip:        &mfav2.SessionIdentifyingPayload{},
			wantErrMsg: "missing payload value",
		},
		{
			desc: "empty ssh session id rejected",
			sip: mfav2.SessionIdentifyingPayload_builder{
				SshSessionId: []byte{},
			}.Build(),
			wantErrMsg: "empty SshSessionId",
		},
		{
			desc: "empty kube client cert fingerprint rejected",
			sip: mfav2.SessionIdentifyingPayload_builder{
				KubeClientCertFingerprint: []byte{},
			}.Build(),
			wantErrMsg: "empty KubeClientCertFingerprint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := checkPayload(tt.sip)
			if tt.wantErrMsg == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErrMsg)
		})
	}
}
