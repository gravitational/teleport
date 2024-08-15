/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"google.golang.org/protobuf/testing/protocmp"
	"sort"

	"github.com/google/go-cmp/cmp"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestKubeProvisionCRUD tests backend operations with KubeProvision resources.
func TestKubeProvisionCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getKubeProvisionService(t)

	kubeProvision1 := newKubeProvision(t, "provision1", &kubeprovisionv1.KubeProvisionSpec{})
	kubeProvision2 := newKubeProvision(t, "provision2", &kubeprovisionv1.KubeProvisionSpec{})

	// Creating a new resource should succeed.
	created, err := service.CreateKubeProvision(ctx, cloneKubeProvision(kubeProvision1))
	require.NoError(t, err)
	require.Empty(t, compareKubeProvisions(kubeProvision1, created))

	// Second attempt to create should fail, kubeProvision already exists
	_, err = service.CreateKubeProvision(ctx, cloneKubeProvision(kubeProvision1))
	require.ErrorAs(t, err, new(*trace.AlreadyExistsError))

	// Test that we get correct kubeProvision from the service.
	got, err := service.GetKubeProvision(ctx, kubeProvision1.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, compareKubeProvisions(created, got))

	// Prepare kubeProvision for upsert test by changing something on it.
	got.Spec.ClusterRoles = []*kubeprovisionv1.ClusterRole{{
		Rules: []*kubeprovisionv1.PolicyRule{{
			Verbs:     []string{"get"},
			ApiGroups: []string{""},
			Resources: []string{"pod"},
		}},
	}}

	// Upsert attempt should succeed, it will update existing resource.
	upserted, err := service.UpsertKubeProvision(ctx, cloneKubeProvision(got))
	require.NoError(t, err)
	require.Empty(t, compareKubeProvisions(got, upserted))

	// Upsert a new resource, it will be created.
	upserted2, err := service.UpsertKubeProvision(ctx, cloneKubeProvision(kubeProvision2))
	require.NoError(t, err)
	require.Empty(t, compareKubeProvisions(kubeProvision2, upserted2))

	// List existing kubeProvisions, we should have 2 by now.
	listed, nextToken, err := service.ListKubeProvisions(ctx, 10, "")
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Empty(t, nextToken)

	// Sort results so we can predictably compare them.
	sort.Slice(listed, func(i, j int) bool {
		return listed[i].Metadata.Name < listed[j].Metadata.Name
	})
	require.Empty(t, compareKubeProvisions(upserted, listed[0]))
	require.Empty(t, compareKubeProvisions(kubeProvision2, listed[1]))

	// Check that page token is returned if page size is smaller than amount of available resources.
	listed, nextToken, err = service.ListKubeProvisions(ctx, 1, "")
	require.NoError(t, err)
	require.NotEmpty(t, nextToken)

	// Check that update fails if revision is missing
	_, err = service.UpdateKubeProvision(ctx, cloneKubeProvision(kubeProvision2))
	require.ErrorAs(t, err, new(*trace.CompareFailedError))

	// Getting non-existing resource returns an error.
	_, err = service.GetKubeProvision(ctx, "wrong_name")
	require.ErrorAs(t, err, new(*trace.NotFoundError), "expected not found error, got %v", err)

	// Delete kubeProvision.
	err = service.DeleteKubeProvision(ctx, kubeProvision1.Metadata.Name)
	require.NoError(t, err)

	// Deleting non-existing resource returns an error.
	err = service.DeleteKubeProvision(ctx, "wrong_name")
	require.ErrorAs(t, err, new(*trace.NotFoundError), "expected not found error, got %v", err)
}

// cloneKubeProvision clones KubeProvision resource. We need it because default service implementation returns
// pointer to the same struct it got as a parameter, so comparing them would always be equal.
func cloneKubeProvision(source *kubeprovisionv1.KubeProvision) *kubeprovisionv1.KubeProvision {
	return proto.Clone(source).(*kubeprovisionv1.KubeProvision)
}

func compareKubeProvisions(expected, actual *kubeprovisionv1.KubeProvision) string {
	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}
	return cmp.Diff(expected, actual, cmpOpts...)
}

func newKubeProvision(t *testing.T, name string, spec *kubeprovisionv1.KubeProvisionSpec) *kubeprovisionv1.KubeProvision {
	t.Helper()

	kubeProvision := kubeprovisionv1.KubeProvision{
		Kind:    types.KindKubeProvision,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}

	return &kubeProvision
}

func getKubeProvisionService(t *testing.T) services.KubeProvisions {
	t.Helper()
	memoryBackend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewKubeProvisionService(memoryBackend)
	require.NoError(t, err)
	return service
}
