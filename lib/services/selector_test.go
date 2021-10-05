/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// TestMatchResourceLabels tests matching a resource against a selector.
func TestMatchResourceLabels(t *testing.T) {
	tests := []struct {
		description    string
		selectors      []Selector
		databaseLabels map[string]string
		match          bool
	}{
		{
			description: "wildcard selector matches empty labels",
			selectors: []Selector{
				{MatchLabels: types.Labels{types.Wildcard: []string{types.Wildcard}}},
			},
			databaseLabels: nil,
			match:          true,
		},
		{
			description: "wildcard selector matches any label",
			selectors: []Selector{
				{MatchLabels: types.Labels{types.Wildcard: []string{types.Wildcard}}},
			},
			databaseLabels: map[string]string{
				uuid.New(): uuid.New(),
				uuid.New(): uuid.New(),
			},
			match: true,
		},
		{
			description: "selector doesn't match empty labels",
			selectors: []Selector{
				{MatchLabels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: nil,
			match:          false,
		},
		{
			description: "selector matches specific label",
			selectors: []Selector{
				{MatchLabels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: map[string]string{"env": "dev"},
			match:          true,
		},
		{
			description: "selector doesn't match label",
			selectors: []Selector{
				{MatchLabels: types.Labels{"env": []string{"dev"}}},
			},
			databaseLabels: map[string]string{"env": "prod"},
			match:          false,
		},
		{
			description: "selector matches label",
			selectors: []Selector{
				{MatchLabels: types.Labels{"env": []string{"dev", "prod"}}},
			},
			databaseLabels: map[string]string{"env": "prod"},
			match:          true,
		},
		{
			description: "selector doesn't match multiple labels",
			selectors: []Selector{
				{MatchLabels: types.Labels{
					"env":     []string{"dev"},
					"cluster": []string{"root"},
				}},
			},
			databaseLabels: map[string]string{"cluster": "root"},
			match:          false,
		},
		{
			description: "selector matches multiple labels",
			selectors: []Selector{
				{MatchLabels: types.Labels{
					"env":     []string{"dev"},
					"cluster": []string{"root"},
				}},
			},
			databaseLabels: map[string]string{"cluster": "root", "env": "dev"},
			match:          true,
		},
		{
			description: "one of multiple selectors matches",
			selectors: []Selector{
				{MatchLabels: types.Labels{"env": []string{"dev"}}},
				{MatchLabels: types.Labels{"cluster": []string{"root"}}},
			},
			databaseLabels: map[string]string{"cluster": "root"},
			match:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name:   "test",
				Labels: test.databaseLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			})
			require.NoError(t, err)

			require.Equal(t, test.match, MatchResourceLabels(test.selectors, database))
		})
	}
}
