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

package web

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestListWorkloadIdentities(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"workload-identity",
	)

	name := uuid.New().String()

	_, err := env.server.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				"label-1": "value-1",
				"label-2": "value-2",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id:   "/test/spiffe/id",
				Hint: "Lorem ipsum delor sit",
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

	var instances ListWorkloadIdentitiesResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

	assert.Len(t, instances.Items, 1)
	require.Empty(t, cmp.Diff(instances, ListWorkloadIdentitiesResponse{
		Items: []WorkloadIdentity{
			{
				Name:       name,
				SpiffeID:   "/test/spiffe/id",
				SpiffeHint: "Lorem ipsum delor sit",
				Labels: map[string]string{
					"label-1": "value-1",
					"label-2": "value-2",
				},
			},
		},
	}))
}

func TestListWorkloadIdentitiesPaging(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name         string
		numInstances int
		pageSize     int
	}{
		{
			name:         "zero results",
			numInstances: 0,
			pageSize:     1,
		},
		{
			name:         "smaller page size",
			numInstances: 5,
			pageSize:     2,
		},
		{
			name:         "larger page size",
			numInstances: 2,
			pageSize:     5,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			env := newWebPack(t, 1)
			proxy := env.proxies[0]
			pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
			clusterName := env.server.ClusterName()
			endpoint := pack.clt.Endpoint(
				"webapi",
				"sites",
				clusterName,
				"workload-identity",
			)

			for i := range tc.numInstances {
				_, err := env.server.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: uuid.New().String(),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/test/spiffe/" + uuid.New().String(),
						},
					},
				})
				require.NoError(t, err, "failed to create WorkloadIdentity index:%d", i)
			}

			response, err := pack.clt.Get(ctx, endpoint, url.Values{
				"page_token": []string{""}, // default to the start
				"page_size":  []string{strconv.Itoa(tc.pageSize)},
			})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

			var resp ListWorkloadIdentitiesResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

			assert.Len(t, resp.Items, int(math.Min(float64(tc.numInstances), float64(tc.pageSize))))
		})
	}
}

func TestListWorkloadIdentitiesSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1, withWebPackAuthCacheEnabled(true))
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"workload-identity",
	)

	for i := range 10 {
		_, err := env.server.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.New().String(),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/test/spiffe/" + uuid.New().String(),
				},
			},
		})
		require.NoError(t, err, "failed to create WorkloadIdentity index:%d", i)
	}

	response, err := pack.clt.Get(ctx, endpoint, url.Values{
		"page_token": []string{""}, // default to the start
		"page_size":  []string{"0"},
		"sort_field": []string{"spiffe_id"},
		"sort_dir":   []string{"DESC"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

	var resp ListWorkloadIdentitiesResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

	prevValue := "~"
	for _, r := range resp.Items {
		assert.Less(t, r.SpiffeID, prevValue)
		prevValue = r.SpiffeID
	}
}

func TestListWorkloadIdentitiesWithSearchTermFilter(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name       string
		searchTerm string
		metadata   *headerv1.Metadata
		spec       *workloadidentityv1pb.WorkloadIdentitySpec
	}{
		{
			name:       "match on name",
			searchTerm: "nick",
			metadata: &headerv1.Metadata{
				Name: "this-is-nicks-workload-identity",
			},
			spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/spiffe/id/99",
				},
			},
		},
		{
			name:       "match on spiffe id",
			searchTerm: "id/22",
			metadata: &headerv1.Metadata{
				Name: "this-is-nicks-workload-identity",
			},
			spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/spiffe/id/22",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			env := newWebPack(t, 1)
			proxy := env.proxies[0]
			pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
			clusterName := env.server.ClusterName()
			endpoint := pack.clt.Endpoint(
				"webapi",
				"sites",
				clusterName,
				"workload-identity",
			)

			_, err := env.server.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
				Kind:     types.KindWorkloadIdentity,
				Version:  types.V1,
				Metadata: tc.metadata,
				Spec:     tc.spec,
			})
			require.NoError(t, err)

			_, err = env.server.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "gone",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/test/spiffe/id",
					},
				},
			})
			require.NoError(t, err)

			response, err := pack.clt.Get(ctx, endpoint, url.Values{
				"search": []string{tc.searchTerm},
			})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

			var resp ListWorkloadIdentitiesResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

			assert.Len(t, resp.Items, 1)
			assert.Equal(t, "this-is-nicks-workload-identity", resp.Items[0].Name)
		})
	}
}
