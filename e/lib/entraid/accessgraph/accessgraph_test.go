package accessgraph

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestReconcile(t *testing.T) {
	tenantID := uuid.NewString()

	id1 := uuid.NewString()
	id2 := uuid.NewString()
	id3 := uuid.NewString()
	id4 := uuid.NewString()
	appID1 := uuid.NewString()
	appID2 := uuid.NewString()
	appID3 := uuid.NewString()
	appID4 := uuid.NewString()

	app1 := accessgraphv1alpha.EntraApplication_builder{
		Id:                  id1,
		AppId:               appID1,
		TenantId:            tenantID,
		DisplayName:         "App 1",
		SigningCertificates: []string{"cert1"},
	}.Build()

	app2 := accessgraphv1alpha.EntraApplication_builder{
		Id:                  id2,
		AppId:               appID2,
		TenantId:            tenantID,
		DisplayName:         "App 2",
		SigningCertificates: []string{"cert2"},
	}.Build()

	app2Updated := accessgraphv1alpha.EntraApplication_builder{
		Id:                  id2,
		AppId:               appID2,
		TenantId:            tenantID,
		DisplayName:         "App 2 updated",
		SigningCertificates: []string{"cert2"},
	}.Build()

	app3 := accessgraphv1alpha.EntraApplication_builder{
		Id:                  id3,
		AppId:               appID3,
		TenantId:            tenantID,
		DisplayName:         "App 3",
		SigningCertificates: []string{"cert3"},
	}.Build()

	app4 := accessgraphv1alpha.EntraApplication_builder{
		Id:                  id4,
		AppId:               appID4,
		TenantId:            tenantID,
		DisplayName:         "App 4",
		SigningCertificates: []string{"cert4"},
	}.Build()

	oldApps := []*accessgraphv1alpha.EntraApplication{
		app1,
		app2,
		app3,
	}

	newApps := []*accessgraphv1alpha.EntraApplication{
		app1,        // Not modified
		app2Updated, // Updated
		// app3 deleted
		app4, // Newly inserted
	}

	wantUpsert := accessgraphv1alpha.EntraResourceList_builder{
		Resources: []*accessgraphv1alpha.EntraResource{
			{Resource: &accessgraphv1alpha.EntraResource_Application{Application: app2Updated}},
			{Resource: &accessgraphv1alpha.EntraResource_Application{Application: app4}},
		},
	}.Build()

	wantDelete := accessgraphv1alpha.EntraResourceList_builder{
		Resources: []*accessgraphv1alpha.EntraResource{
			{Resource: &accessgraphv1alpha.EntraResource_Application{Application: app3}},
		},
	}.Build()

	upsert, delete := reconcileResults(
		&resources{Applications: oldApps},
		&resources{Applications: newApps},
	)
	require.Empty(t, cmp.Diff(
		wantUpsert.GetResources(), upsert.GetResources(),
		protocmp.Transform(),
	))
	require.Empty(t, cmp.Diff(
		wantDelete.GetResources(), delete.GetResources(),
		protocmp.Transform(),
	))
}
