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
	"maps"
	"slices"
	"sync"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
)

const (
	// defaultAccessMonitoringRulePageSize is the default number of rules to retrieve per request
	defaultAccessMonitoringRulePageSize = 1000
)

// Cache is an access monitoring rules cache.
type Cache struct {
	sync.RWMutex
	// rules are the access monitoring rules being stored.
	rules map[string]*accessmonitoringrulesv1.AccessMonitoringRule
}

// NewCache returns a new access monitoring rules cache.
func NewCache() *Cache {
	return &Cache{
		rules: make(map[string]*accessmonitoringrulesv1.AccessMonitoringRule),
	}
}

// fetchRulesPage defines a function that fetches an access monitoring rules page.
type fetchRulesPage func(ctx context.Context, pageSize int64, pageToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)

// Initialize initializes the cache by fetching all the rules using the provided
// fetchRulesPage function and puts them in the cache.
func (cache *Cache) Initialize(ctx context.Context, fn fetchRulesPage) error {
	var page []*accessmonitoringrulesv1.AccessMonitoringRule
	var nextToken string
	var err error
	for {
		page, nextToken, err = fn(ctx, defaultAccessMonitoringRulePageSize, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, rule := range page {
			cache.Put(rule)
		}
		if nextToken == "" {
			return nil
		}
	}
}

// Get returns the entire set of access monitoring rules.
func (cache *Cache) Get() []*accessmonitoringrulesv1.AccessMonitoringRule {
	cache.RLock()
	defer cache.RUnlock()
	return slices.Collect(maps.Values(maps.Clone(cache.rules)))
}

// Put puts the access monitoring rule into the cache.
// Replaces existing access monitoring rule with the same name.
func (cache *Cache) Put(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
	cache.Lock()
	defer cache.Unlock()
	cache.rules[amr.GetMetadata().GetName()] = amr
}

// Delete deletes the access monitoring rule from the cache.
// No-op if the access monitoring rule does not exist in the cache.
func (cache *Cache) Delete(name string) {
	cache.Lock()
	defer cache.Unlock()
	delete(cache.rules, name)
}
