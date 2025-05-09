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
	"google.golang.org/protobuf/proto"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type accessMonitoringRuleIndex string

const accessMonitoringRuleNameIndex accessMonitoringRuleIndex = "name"

func newAccessMonitoringRuleCollection(upstream services.AccessMonitoringRules, w types.WatchKind) (*collection[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Integrations")
	}

	return &collection[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex]{
		store: newStore(
			proto.CloneOf[*accessmonitoringrulesv1.AccessMonitoringRule],
			map[accessMonitoringRuleIndex]func(*accessmonitoringrulesv1.AccessMonitoringRule) string{
				accessMonitoringRuleNameIndex: func(r *accessmonitoringrulesv1.AccessMonitoringRule) string {
					return r.GetMetadata().Name
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
			var resources []*accessmonitoringrulesv1.AccessMonitoringRule
			var nextToken string
			for {
				var page []*accessmonitoringrulesv1.AccessMonitoringRule
				var err error
				page, nextToken, err = upstream.ListAccessMonitoringRules(ctx, 0 /* page size */, nextToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				resources = append(resources, page...)

				if nextToken == "" {
					break
				}
			}
			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *accessmonitoringrulesv1.AccessMonitoringRule {
			return &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// ListAccessMonitoringRules returns a paginated list of access monitoring rules.
func (c *Cache) ListAccessMonitoringRules(ctx context.Context, pageSize int, pageToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessMonitoringRules")
	defer span.End()

	lister := genericLister[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex]{
		cache:        c,
		collection:   c.collections.accessMonitoringRules,
		index:        accessMonitoringRuleNameIndex,
		upstreamList: c.Config.AccessMonitoringRules.ListAccessMonitoringRules,
		nextToken: func(t *accessmonitoringrulesv1.AccessMonitoringRule) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListAccessMonitoringRulesWithFilter returns a paginated list of access monitoring rules.
func (c *Cache) ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessMonitoringRules")
	defer span.End()

	lister := genericLister[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex]{
		cache:        c,
		collection:   c.collections.accessMonitoringRules,
		index:        accessMonitoringRuleNameIndex,
		upstreamList: c.Config.AccessMonitoringRules.ListAccessMonitoringRules,
		nextToken: func(t *accessmonitoringrulesv1.AccessMonitoringRule) string {
			return t.GetMetadata().Name
		},
		filter: func(rule *accessmonitoringrulesv1.AccessMonitoringRule) bool {
			return services.MatchAccessMonitoringRule(rule, req.GetSubjects(), req.GetNotificationName(), req.GetAutomaticReviewName())
		},
	}
	out, next, err := lister.list(ctx, int(req.GetPageSize()), req.GetPageToken())
	return out, next, trace.Wrap(err)
}

// GetAccessMonitoringRule returns the specified AccessMonitoringRule resources.
func (c *Cache) GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessMonitoringRule")
	defer span.End()

	getter := genericGetter[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex]{
		cache:       c,
		collection:  c.collections.accessMonitoringRules,
		index:       accessMonitoringRuleNameIndex,
		upstreamGet: c.Config.AccessMonitoringRules.GetAccessMonitoringRule,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
