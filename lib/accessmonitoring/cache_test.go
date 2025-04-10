/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestCache(t *testing.T) {
	cache := NewCache()

	// Ensure rules are cached
	rules := []*accessmonitoringrulesv1.AccessMonitoringRule{
		{Metadata: &headerv1.Metadata{Name: "test-rule-1"}},
		{Metadata: &headerv1.Metadata{Name: "test-rule-2"}},
	}
	cache.Put(rules)
	require.Len(t, cache.Get(), 2)
	require.ElementsMatch(t, rules, cache.Get())

	// Ensure rules are updated
	updatedRules := []*accessmonitoringrulesv1.AccessMonitoringRule{
		{Metadata: &headerv1.Metadata{
			Name:        "test-rule-1",
			Description: "updated-rule-1",
		}},
		{Metadata: &headerv1.Metadata{
			Name:        "test-rule-2",
			Description: "updated-rule-2",
		}},
	}
	cache.Put(updatedRules)
	require.Len(t, cache.Get(), 2)
	require.ElementsMatch(t, updatedRules, cache.Get())

	// Ensure rules are deleted
	for _, rule := range rules {
		cache.Delete(rule.GetMetadata().GetName())
	}
	require.Empty(t, cache.Get())
}

func TestInitializeCache(t *testing.T) {
	cache := NewCache()

	rulesPageOne := []*accessmonitoringrulesv1.AccessMonitoringRule{
		{Metadata: &headerv1.Metadata{Name: "test-rule-1"}},
		{Metadata: &headerv1.Metadata{Name: "test-rule-2"}},
	}
	rulesPageTwo := []*accessmonitoringrulesv1.AccessMonitoringRule{
		{Metadata: &headerv1.Metadata{Name: "test-rule-3"}},
		{Metadata: &headerv1.Metadata{Name: "test-rule-4"}},
	}
	mockFetchPages := func(ctx context.Context, pageSize int64, pageToken string) (
		[]*accessmonitoringrulesv1.AccessMonitoringRule,
		string,
		error,
	) {
		switch pageToken {
		default:
			return nil, "1", nil
		case "1":
			return rulesPageOne, "2", nil
		case "2":
			return rulesPageTwo, "", nil
		}
	}

	require.NoError(t, cache.Initialize(context.Background(), mockFetchPages))
	require.Len(t, cache.Get(), 4)
	require.ElementsMatch(t, append(rulesPageOne, rulesPageTwo...), cache.Get())
}
