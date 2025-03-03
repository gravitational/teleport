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

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// TestKubernetesV2Command tests that the KubernetesCommand properly parses its
// arguments and applies as expected onto a BotConfig.
func TestKubernetesV2Command(t *testing.T) {
	testStartConfigureCommand(t, NewKubernetesV2Command, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"kubernetes/v2",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--disable-exec-plugin",
				"--name-selector=a",
				"--name-selector=b",
				"--label-selector=c=\"foo bar\",d=\"baz qux\"",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				// It must configure a kubernetes output with a directory destination.
				svc := cfg.Services[0]
				k8s, ok := svc.(*config.KubernetesV2Output)
				require.True(t, ok)

				require.True(t, k8s.DisableExecPlugin)

				dir, ok := k8s.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)

				var foundA, foundB, foundLabelSelector bool
				for _, selector := range k8s.Selectors {
					switch selector.Name {
					case "a":
						foundA = true
					case "b":
						foundB = true
					case "":
						require.Equal(t, map[string]string{
							"c": "foo bar",
							"d": "baz qux",
						}, selector.Labels)
						foundLabelSelector = true
					default:
						require.Fail(t, "unexpected selector name %q", selector.Name)
					}
				}

				require.True(t, foundA, "name selector 'a' must exist")
				require.True(t, foundB, "name selector 'b' must exist")
				require.True(t, foundLabelSelector, "label selector must exist")
			},
		},
	})
}
