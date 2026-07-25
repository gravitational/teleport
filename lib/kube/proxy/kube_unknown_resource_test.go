/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const (
	unknownKindPath = "/apis/argoproj.io/v1alpha1/namespaces/team-x/applications/prod-billing"
	knownKindPath   = "/api/v1/namespaces/default/pods/foo"
)

// A kind absent from the discovery cache is flagged unsupportedResource
// so the forwarder denies it. A known kind resolves normally.
func TestGetResourceFromRequest_UnknownKindIsDenied(t *testing.T) {
	details := &kubeDetails{kubeCodecs: &globalKubeCodecs, rbacSupportedTypes: getRBACSupportedTypes(t)}

	t.Run("known kind", func(t *testing.T) {
		got, err := getResourceFromRequest(&http.Request{Method: http.MethodGet, URL: &url.URL{Path: knownKindPath}}, details)
		require.NoError(t, err)
		require.NotNil(t, got.rbacResource())
		require.Equal(t, "pods", got.rbacResource().Kind)
		require.False(t, got.unsupportedResource)
	})

	t.Run("unknown kind", func(t *testing.T) {
		got, err := getResourceFromRequest(&http.Request{Method: http.MethodGet, URL: &url.URL{Path: unknownKindPath}}, details)
		require.NoError(t, err)
		require.Nil(t, got.rbacResource())
		require.True(t, got.unsupportedResource)
	})
}

// A miss discovers the kind's group-version (picking up a freshly-installed CRD),
// then serves it from cache without re-discovering.
func TestKubeDetails_ResolveResourceTargetedDiscoveryOnMiss(t *testing.T) {
	t.Parallel()
	client := &fakeDiscoveryClientSet{resources: []*metav1.APIResourceList{coreList()}}
	codecs, rbac, gvk, err := newClusterSchemaBuilder(logtest.NewLogger(), client)
	require.NoError(t, err)
	details := &kubeDetails{kubeCreds: &staticKubeCreds{kubeClient: client}, kubeCodecs: codecs, rbacSupportedTypes: rbac, gvkSupportedResources: gvk}

	_, found := details.resolveResource("argoproj.io", "v1alpha1", "applications")
	require.False(t, found)

	// Install the CRD. The next miss discovers and merges it.
	client.resources = []*metav1.APIResourceList{coreList(), argoList()}
	res, found := details.resolveResource("argoproj.io", "v1alpha1", "applications")
	require.True(t, found)
	require.Equal(t, "applications", res.Name)

	// Once merged, lookups are cache hits and trigger no further discovery.
	calls := client.calls
	_, found = details.resolveResource("argoproj.io", "v1alpha1", "applications")
	require.True(t, found)
	require.Equal(t, calls, client.calls)
}

// A SelfSubjectAccessReview (kubectl auth can-i) often omits the API version. A miss still
// discovers the kind by resolving the group's preferred version, matching the data path.
func TestKubeDetails_ResolveResourceWithoutVersion(t *testing.T) {
	t.Parallel()
	client := &fakeDiscoveryClientSet{resources: []*metav1.APIResourceList{coreList()}}
	codecs, rbac, gvk, err := newClusterSchemaBuilder(logtest.NewLogger(), client)
	require.NoError(t, err)
	details := &kubeDetails{kubeCreds: &staticKubeCreds{kubeClient: client}, kubeCodecs: codecs, rbacSupportedTypes: rbac, gvkSupportedResources: gvk}

	// Install the CRD after startup, then resolve it without a version.
	client.resources = []*metav1.APIResourceList{coreList(), argoList()}
	res, found := details.resolveResource("argoproj.io", "", "applications")
	require.True(t, found)
	require.Equal(t, "applications", res.Name)
}

// fakeDiscoveryClientSet serves a swappable set of API resources and counts
// discovery calls. Not safe for concurrent use.
type fakeDiscoveryClientSet struct {
	kubernetes.Interface
	discovery.DiscoveryInterface
	resources []*metav1.APIResourceList
	calls     int
}

func (c *fakeDiscoveryClientSet) Discovery() discovery.DiscoveryInterface { return c }

func (c *fakeDiscoveryClientSet) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	c.calls++
	return nil, c.resources, nil
}

func (c *fakeDiscoveryClientSet) ServerResourcesForGroupVersion(gv string) (*metav1.APIResourceList, error) {
	c.calls++
	for _, l := range c.resources {
		if l.GroupVersion == gv {
			return l, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: gv}, "")
}

func (c *fakeDiscoveryClientSet) ServerGroups() (*metav1.APIGroupList, error) {
	c.calls++
	out := &metav1.APIGroupList{}
	for _, l := range c.resources {
		group, version := getKubeAPIGroupAndVersion(l.GroupVersion)
		gvfd := metav1.GroupVersionForDiscovery{GroupVersion: l.GroupVersion, Version: version}
		out.Groups = append(out.Groups, metav1.APIGroup{Name: group, Versions: []metav1.GroupVersionForDiscovery{gvfd}, PreferredVersion: gvfd})
	}
	return out, nil
}

func coreList() *metav1.APIResourceList {
	return &metav1.APIResourceList{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod", Namespaced: true}}}
}

func argoList() *metav1.APIResourceList {
	return &metav1.APIResourceList{GroupVersion: "argoproj.io/v1alpha1", APIResources: []metav1.APIResource{{Name: "applications", Kind: "Application", Namespaced: true}}}
}
