/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
)

func newAccessListSpec(nextAudit time.Time) accesslist.Spec {
	return accesslist.Spec{
		Title:       "crane operation",
		Description: "Access list that Gru uses to allow the minions to operate the crane.",
		Owners:      []accesslist.Owner{{Name: "Gru", Description: "The super villain.", MembershipKind: accesslist.MembershipKindUser}},
		Audit: accesslist.Audit{
			Recurrence: accesslist.Recurrence{
				Frequency:  accesslist.SixMonths,
				DayOfMonth: accesslist.FirstDayOfMonth,
			},
			NextAuditDate: nextAudit,
		},
		MembershipRequires: accesslist.Requires{
			Roles:  []string{"minion"},
			Traits: trait.Traits{},
		},
		OwnershipRequires: accesslist.Requires{
			Roles:  []string{"supervillain"},
			Traits: trait.Traits{},
		},
		Grants: accesslist.Grants{
			Roles:  []string{"crane-operator"},
			Traits: trait.Traits{},
		},
	}
}

type accessListTestingPrimitives struct {
	setup *TestSetup
	reconcilers.ResourceWithLabelsAdapter[*accesslist.AccessList]
}

func (g *accessListTestingPrimitives) Init(setup *TestSetup) {
	g.setup = setup
}

func (g *accessListTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *accessListTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	metadata := header.Metadata{
		Name: name,
	}
	accessList, err := accesslist.NewAccessList(metadata, newAccessListSpec(time.Time{}))
	if err != nil {
		return trace.Wrap(err)
	}
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	accessList.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.AccessListClient().UpsertAccessList(ctx, accessList)
	return trace.Wrap(err)
}

func (g *accessListTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*accesslist.AccessList, error) {
	al, err := g.setup.TeleportClient.AccessListClient().GetAccessList(ctx, name)
	return al, trace.Wrap(err)
}

func (g *accessListTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.AccessListClient().DeleteAccessList(ctx, name))
}

func (g *accessListTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	accessList := &resourcesv1.TeleportAccessList{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportAccessListSpec(newAccessListSpec(time.Time{})),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, accessList))
}

func (g *accessListTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	accessList := &resourcesv1.TeleportAccessList{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, accessList))
}

func (g *accessListTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportAccessList, error) {
	accessList := &resourcesv1.TeleportAccessList{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, accessList)
	return accessList, trace.Wrap(err)
}

func (g *accessListTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	accessList, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	accessList.Spec.Grants.Roles = []string{"crane-operator", "forklift-operator"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, accessList))
}

func (g *accessListTestingPrimitives) CompareTeleportAndKubernetesResource(tResource *accesslist.AccessList, kubeResource *resourcesv1.TeleportAccessList) (bool, string) {
	opts := CompareOptions()
	// If the kubernetes resource does not specify an audit date, it will be computed server-side
	if kubeResource.Spec.Audit.NextAuditDate.IsZero() {
		opts = append(opts, cmpopts.IgnoreFields(accesslist.Audit{}, "Notifications", "NextAuditDate"))
	} else {
		opts = append(opts, cmpopts.IgnoreFields(accesslist.Audit{}, "Notifications"))
	}
	opts = append(opts, cmpopts.IgnoreFields(accesslist.AccessList{}, "Status"))

	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		opts...,
	)
	return diff == "", diff
}

func AccessListCreationTest(t *testing.T, clt *client.Client) {
	test := &accessListTestingPrimitives{}
	ResourceCreationTest[*accesslist.AccessList, *resourcesv1.TeleportAccessList](t, test, WithTeleportClient(clt))
}

func AccessListDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &accessListTestingPrimitives{}
	ResourceDeletionDriftTest[*accesslist.AccessList, *resourcesv1.TeleportAccessList](t, test, WithTeleportClient(clt))
}

func AccessListUpdateTest(t *testing.T, clt *client.Client) {
	test := &accessListTestingPrimitives{}
	ResourceUpdateTest[*accesslist.AccessList, *resourcesv1.TeleportAccessList](t, test, WithTeleportClient(clt))
}

// AccessListMutateExistingTest checks that the operator propagates the expiry
// from the existing access list to the new one it will upsert.
func AccessListMutateExistingTest(t *testing.T, clt *client.Client) {
	ctx := context.Background()
	setup := SetupTestEnv(t, WithTeleportClient(clt), StepByStep)

	// Test setup: create a new AccessList in Teleport with an existing expiry
	expiry := time.Now().Add(10 * time.Hour)
	name := ValidRandomResourceName("accesslist-")

	metadata := header.Metadata{
		Name: name,
	}
	spec := newAccessListSpec(expiry)
	accessList, err := accesslist.NewAccessList(metadata, spec)
	require.NoError(t, err)
	accessList.SetOrigin(types.OriginKubernetes)
	_, err = clt.AccessListClient().UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Test setup create a new AccessList in Kube wihtout specifying the next audit
	kubeAccessList := &resourcesv1.TeleportAccessList{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportAccessListSpec(newAccessListSpec(time.Time{})),
	}
	// We create and get the reosurce to mimick that it comes from kube
	require.NoError(t, setup.K8sClient.Create(ctx, kubeAccessList))
	key := kclient.ObjectKey{
		Name:      name,
		Namespace: setup.Namespace.Name,
	}
	require.NoError(t, setup.K8sClient.Get(ctx, key, kubeAccessList))

	reconciler, err := resources.NewAccessListReconciler(setup.K8sClient, clt)
	require.NoError(t, err)

	// TODO: remove this hack when the role controller uses the teleport reconciler
	// and we can simplify Do, UpsertExternal, Upsert and Reconcile
	r, ok := reconciler.(interface {
		Upsert(context.Context, kclient.Object) error
	})
	require.True(t, ok)

	// Also a hack: convert the structured object into an unstructured one
	// to accommodate the teleport reconciler that casts first as an
	// unstructured object before converting into the final struct.
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kubeAccessList)
	require.NoError(t, err)
	obj := &unstructured.Unstructured{Object: content}
	// Test execution: we trigger a single reconciliation
	require.NoError(t, r.Upsert(ctx, obj))

	// Then we check if the AccessList audit date has been preserved in teleport
	accessList, err = clt.AccessListClient().GetAccessList(ctx, name)
	require.NoError(t, err)
	require.WithinDuration(t, expiry, accessList.Spec.Audit.NextAuditDate, MaxTimeDiff)
}
