/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestUpgradeEnrollPeriodic(t *testing.T) {
	tts := []struct {
		desc          string
		enrolled      map[string]int
		unenrolled    map[string]int
		promptVersion string
	}{
		{
			desc: "mixed-case",
			enrolled: map[string]int{
				"v2.3.4": 5,
				"v1.2.3": 2,
			},
			unenrolled: map[string]int{
				"v2.3.4": 1,
				"v1.2.3": 2,
				"v1.0.0": 1,
			},
			promptVersion: "v2.3.4",
		},
		{
			desc: "trivial-case",
			enrolled: map[string]int{
				"v1.2.3": 1,
			},
			unenrolled: map[string]int{
				"v1.2.2": 1,
			},
			promptVersion: "v1.2.3",
		},
		{
			desc: "insufficient-lag",
			enrolled: map[string]int{
				"v2.3.4": 2,
				"v1.2.3": 3,
			},
			unenrolled: map[string]int{
				"v1.2.3": 3,
				"v1.2.2": 1,
			},
		},
		{
			desc: "sparse-with-prompt",
			enrolled: map[string]int{
				"v1.2.5": 1,
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
			},
			unenrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
			promptVersion: "v1.2.4",
		},
		{
			desc: "sparse-no-prompt",
			enrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
			unenrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
		},
		{
			desc: "no-enrolled",
			unenrolled: map[string]int{
				"v1.2.3": 1,
			},
		},
		{
			desc: "all-enrolled",
			enrolled: map[string]int{
				"v2.3.4": 1,
			},
		},
		{
			desc: "no-instances",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newUpgradeEnrollPeriodic()

			for ver, count := range tt.enrolled {
				for i := 0; i < count; i++ {
					instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
						Version:          ver,
						ExternalUpgrader: "some-upgrader",
					})
					require.NoError(t, err)

					periodic.VisitInstance(instance)
				}
			}

			for ver, count := range tt.unenrolled {
				for i := 0; i < count; i++ {
					instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
						Version:          ver,
						ExternalUpgrader: "",
					})
					require.NoError(t, err)

					periodic.VisitInstance(instance)
				}
			}

			msg, ok := periodic.GenerateEnrollPrompt()

			if tt.promptVersion == "" {
				require.False(t, ok, "expected no prompt gen, but got %q, tt=%q", msg, tt.desc)
				return
			}

			require.True(t, ok, "expected prompt containing version %s, but prompt was not generated. tt=%q", tt.promptVersion, tt.desc)

			pattern := fmt.Sprintf("--older-than=%s", tt.promptVersion)
			require.Contains(t, msg, pattern, "tt=%q", tt.desc)
		})
	}
}
