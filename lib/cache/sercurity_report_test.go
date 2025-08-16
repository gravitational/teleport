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
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestAuditQuery tests that CRUD operations on access list rule resources are
// replicated from the backend to the cache.
func TestAuditQuery(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	t.Run("GetSecurityAuditQueries", func(t *testing.T) {
		testResources(t, p, testFuncs[*secreports.AuditQuery]{
			newResource: func(name string) (*secreports.AuditQuery, error) {
				return newAuditQuery(t, name), nil
			},
			create: func(ctx context.Context, item *secreports.AuditQuery) error {
				err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
				return trace.Wrap(err)
			},
			list:     p.secReports.GetSecurityAuditQueries,
			cacheGet: p.cache.GetSecurityAuditQuery,
			cacheList: func(ctx context.Context, pageSize int) ([]*secreports.AuditQuery, error) {
				return p.cache.GetSecurityAuditQueries(ctx)
			},
			update: func(ctx context.Context, item *secreports.AuditQuery) error {
				err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
				return trace.Wrap(err)
			},
			deleteAll: p.secReports.DeleteAllSecurityAuditQueries,
		})
	})

	t.Run("ListSecurityAuditQueries", func(t *testing.T) {
		testResources(t, p, testFuncs[*secreports.AuditQuery]{
			newResource: func(name string) (*secreports.AuditQuery, error) {
				return newAuditQuery(t, name), nil
			},
			create: func(ctx context.Context, item *secreports.AuditQuery) error {
				err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
				return trace.Wrap(err)
			},
			list:     p.secReports.GetSecurityAuditQueries,
			cacheGet: p.cache.GetSecurityAuditQuery,
			cacheList: func(ctx context.Context, pageSize int) ([]*secreports.AuditQuery, error) {
				return stream.Collect(clientutils.ResourcesWithPageSize(ctx, p.cache.ListSecurityAuditQueries, pageSize))
			},
			update: func(ctx context.Context, item *secreports.AuditQuery) error {
				err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
				return trace.Wrap(err)
			},
			deleteAll: p.secReports.DeleteAllSecurityAuditQueries,
		})
	})

}

// TestSecurityReportState tests that CRUD operations on security report state resources are
// replicated from the backend to the cache.
func TestSecurityReports(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	t.Run("GetSecurityReports", func(t *testing.T) {
		testResources(t, p, testFuncs[*secreports.Report]{
			newResource: func(name string) (*secreports.Report, error) {
				return newSecurityReport(t, name), nil
			},
			create: func(ctx context.Context, item *secreports.Report) error {
				err := p.secReports.UpsertSecurityReport(ctx, item)
				return trace.Wrap(err)
			},
			list:     p.secReports.GetSecurityReports,
			cacheGet: p.cache.GetSecurityReport,
			cacheList: func(ctx context.Context, pageSize int) ([]*secreports.Report, error) {
				return p.cache.GetSecurityReports(ctx)
			},
			update: func(ctx context.Context, item *secreports.Report) error {
				err := p.secReports.UpsertSecurityReport(ctx, item)
				return trace.Wrap(err)
			},
			deleteAll: p.secReports.DeleteAllSecurityReports,
		})
	})
	t.Run("ListSecurityReports", func(t *testing.T) {
		testResources(t, p, testFuncs[*secreports.Report]{
			newResource: func(name string) (*secreports.Report, error) {
				return newSecurityReport(t, name), nil
			},
			create: func(ctx context.Context, item *secreports.Report) error {
				err := p.secReports.UpsertSecurityReport(ctx, item)
				return trace.Wrap(err)
			},
			list:     p.secReports.GetSecurityReports,
			cacheGet: p.cache.GetSecurityReport,
			cacheList: func(ctx context.Context, pageSize int) ([]*secreports.Report, error) {
				return stream.Collect(clientutils.ResourcesWithPageSize(ctx, p.cache.ListSecurityReports, pageSize))
			},
			update: func(ctx context.Context, item *secreports.Report) error {
				err := p.secReports.UpsertSecurityReport(ctx, item)
				return trace.Wrap(err)
			},
			deleteAll: p.secReports.DeleteAllSecurityReports,
		})
	})

}

// TestSecurityReportState tests that CRUD operations on security report state resources are
// replicated from the backend to the cache.
func TestSecurityReportState(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*secreports.ReportState]{
		newResource: func(name string) (*secreports.ReportState, error) {
			return newSecurityReportState(t, name), nil
		},
		create: func(ctx context.Context, item *secreports.ReportState) error {
			err := p.secReports.UpsertSecurityReportsState(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*secreports.ReportState, error) {
			return stream.Collect(clientutils.Resources(ctx, p.secReports.ListSecurityReportsStates))
		},
		cacheGet: p.cache.GetSecurityReportState,
		cacheList: func(ctx context.Context, pageSize int) ([]*secreports.ReportState, error) {
			return stream.Collect(clientutils.ResourcesWithPageSize(ctx, p.cache.ListSecurityReportsStates, pageSize))
		},
		update: func(ctx context.Context, item *secreports.ReportState) error {
			err := p.secReports.UpsertSecurityReportsState(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.secReports.DeleteAllSecurityReportsStates,
	})
}
