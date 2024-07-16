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

package workloadattest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mountpointSourceToContainerAndPodID(t *testing.T) {
	tests := []struct {
		name            string
		source          string
		wantPodID       string
		wantContainerID string
	}{
		{
			name:            "k3s-ubuntu-v1.28.6+k3s2",
			source:          "/../../kubepods-besteffort-podfecd2321_17b5_49b9_9f75_8c5be777fbfb.slice/cri-containerd-397529d07efebd566f15dbc7e8af9f3ef586033f5e753adfa96b2bf730102c64.scope",
			wantPodID:       "fecd2321-17b5-49b9-9f75-8c5be777fbfb",
			wantContainerID: "397529d07efebd566f15dbc7e8af9f3ef586033f5e753adfa96b2bf730102c64",
		},
		{
			name:            "orbstack-v1.6.4",
			source:          "/../../pod36827f77-691f-45aa-a470-0989cf3749c4/64dd9bf5199ff782835247cb072e4842dc3d0135ef02f6498cb6bb6f37a320d2",
			wantPodID:       "36827f77-691f-45aa-a470-0989cf3749c4",
			wantContainerID: "64dd9bf5199ff782835247cb072e4842dc3d0135ef02f6498cb6bb6f37a320d2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPodID, gotContainerID, err := mountpointSourceToContainerAndPodID(tt.source)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPodID, gotPodID)
			assert.Equal(t, tt.wantContainerID, gotContainerID)
		})
	}
}
