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

package componentfeatures

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNew(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
	)

	tests := []struct {
		name     string
		features []FeatureID
		want     []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name:     "no features yields empty set",
			features: []FeatureID{},
			want:     []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name:     "duplicates are ignored",
			features: []FeatureID{featA, featB, featA, featB},
			want:     []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := New(tt.features...)
			require.NotNil(t, got, "New(%v) returned nil", tt.features)
			require.ElementsMatch(t, tt.want, got.Features, "New(%v) = %v; want %v", tt.features, got.Features, tt.want)
		})
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
	)

	tests := []struct {
		name string
		sets []*componentfeaturesv1.ComponentFeatures
		want []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name: "no sets returns empty set",
			sets: nil,
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "single set returns same set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
		{
			name: "multiple sets combined",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA), New(featB), New(featA, featC)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto(), featC.ToProto()},
		},
		{
			name: "nil and empty sets ignored",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA), nil, New(), New(featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Join(tt.sets...)
			require.NotNil(t, got, "Join(%v) returned nil", tt.sets)
			require.ElementsMatch(t, tt.want, got.Features, "Join(%v) = %v; want %v", tt.sets, got.Features, tt.want)
		})
	}
}

func TestInAllSets(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
	)

	tests := []struct {
		name    string
		sets    []*componentfeaturesv1.ComponentFeatures
		feature FeatureID
		want    bool
	}{
		{
			name:    "no sets",
			sets:    nil,
			feature: featA,
			want:    false,
		},
		{
			name:    "single set contains feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA)},
			feature: featA,
			want:    true,
		},
		{
			name:    "single set missing feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featB)},
			feature: featA,
			want:    false,
		},
		{
			name:    "multiple sets all contain feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA, featB), New(featA), New(featA, featB, featC)},
			feature: featA,
			want:    true,
		},
		{
			name:    "one set missing feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA), New(featA, featB), New(featB, featC)},
			feature: featA,
			want:    false,
		},
		{
			name:    "nil set treated as unsupported",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featB, featC), nil, New(featA, featB)},
			feature: featB,
			want:    false,
		},
		{
			name:    "empty set treated as unsupported",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA), New(), New(featA, featB)},
			feature: featA,
			want:    false,
		},
		{
			name:    "duplicate feature in set",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA, featA, featB), New(featA, featC, featC)},
			feature: featA,
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := InAllSets(tt.feature, tt.sets...)
			require.Equal(t, tt.want, got, "FeatureInAllSets(%v, %v) = %v; want %v", tt.feature, tt.sets, got, tt.want)
		})
	}
}

func TestIntersect(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
		featD = FeatureID(202)
	)

	tests := []struct {
		name string
		sets []*componentfeaturesv1.ComponentFeatures
		want []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name: "no sets returns empty set",
			sets: nil,
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "nil set returns empty set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB), nil, New(featA, featC)},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "disjoint sets return empty set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB), New(featC, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "single set returns same set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
		{
			name: "sets with full intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featD), New(featD, featA, featB), New(featB, featD, featA)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto(), featD.ToProto()},
		},
		{
			name: "sets with partial intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featC), New(featB, featC, featD), New(featB, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{featB.ToProto()},
		},
		{
			name: "duplicate features do not affect intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featB), New(featA, featA, featB, featC), New(featA, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Intersect(tt.sets...)
			require.NotNil(t, got, "Intersect should not return nil")

			// Order not guaranteed, so compare as sets.
			require.ElementsMatch(t, tt.want, got.Features, "Intersect(%v) = %v; want %v", tt.sets, got.Features, tt.want)
		})
	}
}

func TestGetClusterAuthProxyServerFeatures(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
	)

	tests := []struct {
		name       string
		makeLister func(t *testing.T) *fakeAuthProxyServersLister
		want       []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name: "intersection of single feature across all servers",
			makeLister: func(t *testing.T) *fakeAuthProxyServersLister {
				t.Helper()
				return &fakeAuthProxyServersLister{
					proxies: []types.Server{
						mustServerWithFeatures(t, "proxy-1", New(featA, featB)),
						mustServerWithFeatures(t, "proxy-2", New(featA)),
					},
					auths: []types.Server{
						mustServerWithFeatures(t, "auth-1", New(featA, featB)),
						mustServerWithFeatures(t, "auth-2", New(featA, featC)),
					},
				}
			},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto()},
		},
		{
			name: "missing features on any server yields empty intersection",
			makeLister: func(t *testing.T) *fakeAuthProxyServersLister {
				t.Helper()
				return &fakeAuthProxyServersLister{
					proxies: []types.Server{
						mustServerWithFeatures(t, "proxy-1", New(featA, featC)),
						mustServerWithFeatures(t, "proxy-2", nil),
					},
					auths: []types.Server{mustServerWithFeatures(t, "auth-1", New(featA, featC))},
				}
			},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "collect error yields empty intersection",
			makeLister: func(t *testing.T) *fakeAuthProxyServersLister {
				t.Helper()

				f := &fakeAuthProxyServersLister{
					auths: []types.Server{mustServerWithFeatures(t, "auth-1", New(featA))},
				}

				f.listProxiesFn = func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
					return nil, "", trace.ConnectionProblem(nil, "connection problem")
				}
				f.getProxiesFn = func() ([]types.Server, error) {
					return nil, trace.ConnectionProblem(nil, "connection problem")
				}

				return f
			},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetClusterAuthProxyServerFeatures(t.Context(), tt.makeLister(t), discardLogger())
			require.NotNil(t, got)
			require.ElementsMatch(t, tt.want, got.GetFeatures())
		})
	}
}

type fakeAuthProxyServersLister struct {
	proxies       []types.Server
	auths         []types.Server
	listProxiesFn func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	listAuthFn    func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	getProxiesFn  func() ([]types.Server, error)
	getAuthFn     func() ([]types.Server, error)
}

func (f *fakeAuthProxyServersLister) GetProxies() ([]types.Server, error) {
	if f.getProxiesFn != nil {
		return f.getProxiesFn()
	}
	return f.proxies, nil
}

func (f *fakeAuthProxyServersLister) GetAuthServers() ([]types.Server, error) {
	if f.getAuthFn != nil {
		return f.getAuthFn()
	}
	return f.auths, nil
}

func (f *fakeAuthProxyServersLister) ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	if f.listProxiesFn != nil {
		return f.listProxiesFn(ctx, pageSize, pageToken)
	}
	if pageToken != "" {
		return nil, "", nil
	}
	return f.proxies, "", nil
}

func (f *fakeAuthProxyServersLister) ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	if f.listAuthFn != nil {
		return f.listAuthFn(ctx, pageSize, pageToken)
	}
	if pageToken != "" {
		return nil, "", nil
	}
	return f.auths, "", nil
}

func mustServerWithFeatures(t *testing.T, name string, feats *componentfeaturesv1.ComponentFeatures) types.Server {
	t.Helper()

	srv, err := types.NewServerWithLabels(
		name,
		types.KindNode,
		types.ServerSpecV2{
			Hostname: name,
			Addr:     "127.0.0.1:0",
		},
		nil,
	)
	require.NoError(t, err)

	if feats != nil {
		srv.SetComponentFeatures(feats)
	}
	return srv
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}
