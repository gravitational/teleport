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

package config

import "testing"

func TestKubernetesV2Output_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[KubernetesV2Output]{
		{
			name: "full",
			in: KubernetesV2Output{
				Destination:       dest,
				DisableExecPlugin: true,
				Selectors: []*KubernetesSelector{
					{
						Name: "foo",

						// Unfortunately we have to manually initialize every
						// map if we want tests to pass. Otherwise we'd need to
						// support CheckAndSetDefaults() in testYAML() which
						// breaks a ton of tests that didn't expect to have to
						// compare initialized structs.
						Labels: map[string]string{},
					},
					{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
		{
			name: "minimal",
			in: KubernetesV2Output{
				Destination: dest,
				Selectors: []*KubernetesSelector{
					{
						Name:   "foo",
						Labels: map[string]string{},
					},
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestKubernetesV2Output_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*KubernetesV2Output]{
		{
			name: "valid_name",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{Name: "foo", Labels: map[string]string{}},
					},
				}
			},
		},
		{
			name: "valid_label",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{
							"foo": "bar",
						}},
					},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: nil,
					Selectors: []*KubernetesSelector{
						{Name: "foo"},
					},
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing selectors",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "at least one selector must be provided",
		},
		{
			name: "empty selector",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{},
					},
				}
			},
			wantErr: "selectors: one of 'name' and 'labels' must be specified",
		},
		{
			name: "both name and label in selector",
			in: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{
							Name: "foo",
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				}
			},
			wantErr: "selectors: only one of 'name' and 'labels' may be specified",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
