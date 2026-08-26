/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package controller

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/gravitational/teleport/lib/autoupdate/agent"
)

func TestGenerateData(t *testing.T) {
	tests := []struct {
		name           string
		data           map[string]string
		version        string
		failed         bool
		updateGroup    string
		proxyAddress   string
		expectedConfig string
	}{
		{
			name:         "new configmap",
			version:      "1.0.0",
			updateGroup:  "test-group",
			proxyAddress: "proxy.example.com",
			expectedConfig: `version: v1
kind: update_config
spec:
    proxy: proxy.example.com
    path: ""
    group: test-group
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    active:
        version: 1.0.0
`,
		},
		{
			name: "success",
			data: map[string]string{
				updaterIDFile: "old",
				agent.UpdateConfigName: `version: v1
kind: update_config
spec:
    proxy: old-proxy.example.com
    path: ""
    group: test-group-old
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    active:
       version: 0.9.0
`,
			},
			version:      "1.0.0",
			updateGroup:  "test-group",
			proxyAddress: "proxy.example.com",
			expectedConfig: `version: v1
kind: update_config
spec:
    proxy: proxy.example.com
    path: ""
    group: test-group
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    last_update:
        success: true
        time: 1970-01-01T00:00:01Z
        target:
            version: ""
    active:
        version: 1.0.0
`,
		},
		{
			name: "failure",
			data: map[string]string{
				updaterIDFile: "old",
				agent.UpdateConfigName: `version: v1
kind: update_config
spec:
    proxy: proxy.example.com
    path: ""
    group: test-group
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    active:
       version: 1.0.0
`,
			},
			version:      "1.0.0",
			failed:       true,
			updateGroup:  "test-group",
			proxyAddress: "proxy.example.com",
			expectedConfig: `version: v1
kind: update_config
spec:
    proxy: proxy.example.com
    path: ""
    group: test-group
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    last_update:
        success: false
        time: 1970-01-01T00:00:01Z
        target:
            version: ""
    active:
        version: 1.0.0
`,
		},
		{
			name: "invalid defaults",
			data: map[string]string{
				agent.UpdateConfigName: `bad`,
			},
			version:      "1.0.0",
			updateGroup:  "test-group",
			proxyAddress: "proxy.example.com",
			expectedConfig: `version: v1
kind: update_config
spec:
    proxy: proxy.example.com
    path: ""
    group: test-group
    enabled: true
    pinned: false
status:
    id_file: /etc/updater-config/teleport-update.id
    active:
        version: 1.0.0
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sw := &StatusWriter{
				UpdateID:     uuid.New(),
				UpdateGroup:  tc.updateGroup,
				ProxyAddress: tc.proxyAddress,
			}
			cm := &corev1.ConfigMap{
				Data: tc.data,
			}
			updateTime := time.Unix(1, 0)
			err := sw.generateData(t.Context(), cm, tc.version, tc.failed, updateTime)
			require.NoError(t, err)
			require.Len(t, cm.Data, 2)
			require.Equal(t, tc.expectedConfig, cm.Data[agent.UpdateConfigName])
			require.Equal(t, sw.UpdateID.String(), cm.Data[updaterIDFile])
		})
	}
}
