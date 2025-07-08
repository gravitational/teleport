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

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/internal/testutils"
)

func TestKubernetesV2Output_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[KubernetesV2Output]{
		{
			Name: "full",
			In: KubernetesV2Output{
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
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: KubernetesV2Output{
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
	testutils.TestYAML(t, tests)
}

func TestKubernetesV2Output_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*KubernetesV2Output]{
		{
			Name: "valid_name",
			In: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{Name: "foo", Labels: map[string]string{}},
					},
				}
			},
		},
		{
			Name: "valid_label",
			In: func() *KubernetesV2Output {
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
			Name: "missing destination",
			In: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: nil,
					Selectors: []*KubernetesSelector{
						{Name: "foo"},
					},
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing selectors",
			In: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
				}
			},
			WantErr: "at least one selector must be provided",
		},
		{
			Name: "empty selector",
			In: func() *KubernetesV2Output {
				return &KubernetesV2Output{
					Destination: memoryDestForTest(),
					Selectors: []*KubernetesSelector{
						{},
					},
				}
			},
			WantErr: "selectors: one of 'name' and 'labels' must be specified",
		},
		{
			Name: "both name and label in selector",
			In: func() *KubernetesV2Output {
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
			WantErr: "selectors: only one of 'name' and 'labels' may be specified",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
