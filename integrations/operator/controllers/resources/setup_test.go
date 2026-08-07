/*
Copyright 2026 Gravitational, Inc.

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

package resources

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestRequireSessionSummaries(t *testing.T) {
	t.Parallel()

	require.True(t, controllers.RequireSessionSummaries(&proto.Features{
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.Policy): {Enabled: true},
		},
	}))
	require.False(t, controllers.RequireSessionSummaries(&proto.Features{
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.Policy):           {Enabled: true},
			string(entitlements.SessionSummaries): {Enabled: false},
		},
	}))
	require.True(t, controllers.RequireSessionSummaries(&proto.Features{
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.SessionSummaries): {Enabled: true},
		},
	}))
}

type fakeReconciler struct {
	reconcile.Reconciler
	gvk          schema.GroupVersionKind
	teleportKind string
	scoped       bool
	featureGate  controllers.CheckFeaturesFunc
}

func (f *fakeReconciler) SetupWithManager(mgr manager.Manager) error {
	return nil
}

func (f *fakeReconciler) GVK() schema.GroupVersionKind {
	return f.gvk
}

func (f *fakeReconciler) TeleportKind() string {
	return f.teleportKind
}

func (f *fakeReconciler) Scoped() bool {
	return f.scoped
}

func (f *fakeReconciler) CheckFeatures(features *proto.Features) bool {
	return f.featureGate(features)
}

// Factory is a ReconcilerFactory for the given fakeReconciler.
func (f *fakeReconciler) Factory(_ kclient.Client, _ *client.Client) (controllers.Reconciler, error) {
	return f, nil
}

type fakeDiscovery struct {
	discovery.DiscoveryInterface
	gvks []schema.GroupVersionKind
}

func (f *fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	var apiResources []metav1.APIResource
	for _, gvk := range f.gvks {
		if gvk.Group+"/"+gvk.Version == groupVersion {
			apiResources = append(apiResources, metav1.APIResource{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			})
		}
	}
	if len(apiResources) == 0 {
		return nil, &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	}
	return &metav1.APIResourceList{
		GroupVersion: groupVersion,
		APIResources: apiResources,
	}, nil
}

func TestFilterEnabledReconcilers(t *testing.T) {
	log := logr.FromSlogHandler(logtest.NewLogger().Handler())
	// Test setup: create CRD fixtures and a fake kubernetes client to look them up
	installedGVK := schema.GroupVersionKind{Group: "resources.teleport.dev", Version: "v1", Kind: "Installed"}
	otherGVK := schema.GroupVersionKind{Group: "resources.teleport.dev", Version: "v1", Kind: "AnotherInstalled"}
	missingGVK := schema.GroupVersionKind{Group: "resources.teleport.dev", Version: "v1", Kind: "NotInstalled"}
	missingGV := schema.GroupVersionKind{Group: "not.installed.teleport.dev", Version: "v1", Kind: "NotInstalled"}

	discoveryClient := fakeDiscovery{gvks: []schema.GroupVersionKind{otherGVK, installedGVK}}

	// Test setup: create a list will various kinds of controllers so we can check which ones get selected.
	unscopedReconciler := &fakeReconciler{
		gvk: installedGVK,
		// note: we use teleport_kind
		teleportKind: "unscoped_reconciler",
		scoped:       false,
		featureGate:  controllers.AlwaysEnabled,
	}

	scopedReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      true,
		featureGate: controllers.AlwaysEnabled,
	}

	missingReconciler := &fakeReconciler{
		gvk:         missingGVK,
		scoped:      false,
		featureGate: controllers.AlwaysEnabled,
	}

	missingGVReconciler := &fakeReconciler{
		gvk:         missingGV,
		scoped:      false,
		featureGate: controllers.AlwaysEnabled,
	}

	missingScopedReconciler := &fakeReconciler{
		gvk:         missingGVK,
		scoped:      true,
		featureGate: controllers.AlwaysEnabled,
	}

	unscopedEnterpriseReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      false,
		featureGate: controllers.RequireEnterprise,
	}

	scopedEnterpriseReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      true,
		featureGate: controllers.RequireEnterprise,
	}

	reconcilers := []ReconcilerFactory{
		unscopedReconciler.Factory,
		scopedReconciler.Factory,
		missingReconciler.Factory,
		missingGVReconciler.Factory,
		missingScopedReconciler.Factory,
		unscopedEnterpriseReconciler.Factory,
		scopedEnterpriseReconciler.Factory,
	}

	tests := []struct {
		name     string
		scoped   bool
		features *proto.Features
		expected []controllers.Reconciler
	}{
		{
			name:     "unscoped OSS",
			scoped:   false,
			features: &proto.Features{},
			expected: []controllers.Reconciler{
				unscopedReconciler,
				scopedReconciler,
			},
		},
		{
			name:     "scoped OSS",
			scoped:   true,
			features: &proto.Features{},
			expected: []controllers.Reconciler{
				scopedReconciler,
			},
		},
		{
			name:   "unscoped enterprise",
			scoped: false,
			features: &proto.Features{
				AdvancedAccessWorkflows: true,
			},
			expected: []controllers.Reconciler{
				unscopedReconciler,
				scopedReconciler,
				unscopedEnterpriseReconciler,
				scopedEnterpriseReconciler,
			},
		},
		{
			name:   "scoped enterprise",
			scoped: true,
			features: &proto.Features{
				AdvancedAccessWorkflows: true,
			},
			expected: []controllers.Reconciler{
				scopedReconciler,
				scopedEnterpriseReconciler,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test execution: filter reconciler and check that the picked ones are the ones we expect.
			result, err := filterEnabledReconcilers(
				Config{
					Log:      log,
					Scoped:   tt.scoped,
					Features: tt.features,
				},
				reconcilers, &discoveryClient)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
