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

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type autoUpdateAgentReportIndex string

const autoUpdateAgentReportNameIndex autoUpdateAgentReportIndex = "name"

func newAutoUpdateAgentReportCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateAgentReports")
	}

	return &collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		store: newStore(
			types.KindAutoUpdateAgentReport,
			func(report *autoupdatev1.AutoUpdateAgentReport) *autoupdatev1.AutoUpdateAgentReport {
				return proto.Clone(report).(*autoupdatev1.AutoUpdateAgentReport)
			},
			map[autoUpdateAgentReportIndex]func(*autoupdatev1.AutoUpdateAgentReport) string{
				autoUpdateAgentReportNameIndex: func(r *autoupdatev1.AutoUpdateAgentReport) string {
					return r.GetMetadata().Name
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateAgentReport, error) {
			var discoveryConfigs []*autoupdatev1.AutoUpdateAgentReport
			var nextToken string
			for {
				var page []*autoupdatev1.AutoUpdateAgentReport
				var err error

				page, nextToken, err = upstream.ListAutoUpdateAgentReports(ctx, 0 /* default page size */, nextToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				discoveryConfigs = append(discoveryConfigs, page...)

				if nextToken == "" {
					break
				}
			}
			return discoveryConfigs, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateAgentReport {
			return &autoupdatev1.AutoUpdateAgentReport{
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

// GetAutoUpdateAgentReport gets the AutoUpdateAgentReport from the backend.
func (c *Cache) GetAutoUpdateAgentReport(ctx context.Context, name string) (*autoupdatev1.AutoUpdateAgentReport, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAutoUpdateAgentReport")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		cache:       c,
		collection:  c.collections.autoUpdateReports,
		index:       autoUpdateAgentReportNameIndex,
		upstreamGet: c.Config.AutoUpdateService.GetAutoUpdateAgentReport,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListAutoUpdateAgentReports lists autoupdate_agent_reports.
func (c *Cache) ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdatev1.AutoUpdateAgentReport, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAutoUpdateAgentReports")
	defer span.End()

	lister := genericLister[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		cache:        c,
		collection:   c.collections.autoUpdateReports,
		index:        autoUpdateAgentReportNameIndex,
		upstreamList: c.Config.AutoUpdateService.ListAutoUpdateAgentReports,
		nextToken: func(t *autoupdatev1.AutoUpdateAgentReport) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
