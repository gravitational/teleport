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

package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestCommandRecorder(t *testing.T) {
	tests := []struct {
		name        string
		sshCommand  string
		inputs      [][]byte
		wantActions []*apievents.GitCommandAction
	}{
		{
			name:        "fetch",
			sshCommand:  "git-upload-pack 'my-org/my-repo.git'",
			inputs:      [][]byte{pktline.FlushPkt},
			wantActions: nil,
		},
		{
			name:        "no-op push",
			sshCommand:  "git-receive-pack 'my-org/my-repo.git'",
			inputs:      [][]byte{pktline.FlushPkt},
			wantActions: nil,
		},
		{
			name:       "push with packfile",
			sshCommand: "git-receive-pack 'my-org/my-repo.git'",
			inputs: [][]byte{
				[]byte("00af8a43aa31be3cb1816c8d517d34d61795613300f5 75ad3a489c1537ed064caa874ee38076b5a126be refs/heads/STeve/test\x00 report-status-v2 side-band-64k object-format=sha1 agent=git/2.45.00000"),
				[]byte("PACK-FILE-SHOULD-BE-IGNORED"),
			},
			wantActions: []*apievents.GitCommandAction{{
				Action:    "update",
				Reference: "refs/heads/STeve/test",
				Old:       "8a43aa31be3cb1816c8d517d34d61795613300f5",
				New:       "75ad3a489c1537ed064caa874ee38076b5a126be",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := ParseSSHCommand(tt.sshCommand)
			require.NoError(t, err)

			recorder := NewCommandRecorder(context.Background(), *command)
			for _, input := range tt.inputs {
				n, err := recorder.Write(input)
				require.NoError(t, err)
				require.Equal(t, len(input), n)
			}
			actions, err := recorder.GetActions()
			require.NoError(t, err)
			assert.Equal(t, tt.wantActions, actions)
		})
	}
}
