// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type oktaImportRuleIndex string

const oktaImportRuleNameIndex oktaImportRuleIndex = "name"

func newOktaImportRuleCollection(upstream services.Okta, w types.WatchKind) (*collection[types.OktaImportRule, oktaImportRuleIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Okta")
	}

	return &collection[types.OktaImportRule, oktaImportRuleIndex]{
		store: newStore(
			types.OktaImportRule.Clone,
			map[oktaImportRuleIndex]func(types.OktaImportRule) string{
				oktaImportRuleNameIndex: types.OktaImportRule.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.OktaImportRule, error) {
			var startKey string
			var resources []types.OktaImportRule
			for {
				var importRules []types.OktaImportRule
				var err error
				importRules, startKey, err = upstream.ListOktaImportRules(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				resources = append(resources, importRules...)

				if startKey == "" {
					break
				}
			}

			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.OktaImportRule {
			return &types.OktaImportRuleV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (c *Cache) ListOktaImportRules(ctx context.Context, pageSize int, pageToken string) ([]types.OktaImportRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListOktaImportRules")
	defer span.End()

	lister := genericLister[types.OktaImportRule, oktaImportRuleIndex]{
		cache:        c,
		collection:   c.collections.oktaImportRules,
		index:        oktaImportRuleNameIndex,
		upstreamList: c.Config.Okta.ListOktaImportRules,
		nextToken: func(t types.OktaImportRule) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (c *Cache) GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetOktaImportRule")
	defer span.End()

	getter := genericGetter[types.OktaImportRule, oktaImportRuleIndex]{
		cache:       c,
		collection:  c.collections.oktaImportRules,
		index:       oktaImportRuleNameIndex,
		upstreamGet: c.Config.Okta.GetOktaImportRule,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

type oktaAssignmentIndex string

const oktaAssignmentNameIndex oktaAssignmentIndex = "name"

func newOktaImportAssignmentCollection(upstream services.Okta, w types.WatchKind) (*collection[types.OktaAssignment, oktaAssignmentIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Okta")
	}

	return &collection[types.OktaAssignment, oktaAssignmentIndex]{
		store: newStore(
			types.OktaAssignment.Copy,
			map[oktaAssignmentIndex]func(types.OktaAssignment) string{
				oktaAssignmentNameIndex: types.OktaAssignment.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.OktaAssignment, error) {
			var startKey string
			var resources []types.OktaAssignment
			for {
				var importRules []types.OktaAssignment
				var err error
				importRules, startKey, err = upstream.ListOktaAssignments(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				resources = append(resources, importRules...)

				if startKey == "" {
					break
				}
			}

			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.OktaAssignment {
			return &types.OktaAssignmentV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (c *Cache) ListOktaAssignments(ctx context.Context, pageSize int, pageToken string) ([]types.OktaAssignment, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListOktaAssignments")
	defer span.End()

	lister := genericLister[types.OktaAssignment, oktaAssignmentIndex]{
		cache:        c,
		collection:   c.collections.oktaAssignments,
		index:        oktaAssignmentNameIndex,
		upstreamList: c.Config.Okta.ListOktaAssignments,
		nextToken: func(t types.OktaAssignment) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetOktaAssignment returns the specified Okta assignment resources.
func (c *Cache) GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetOktaAssignment")
	defer span.End()

	getter := genericGetter[types.OktaAssignment, oktaAssignmentIndex]{
		cache:       c,
		collection:  c.collections.oktaAssignments,
		index:       oktaAssignmentNameIndex,
		upstreamGet: c.Config.Okta.GetOktaAssignment,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
