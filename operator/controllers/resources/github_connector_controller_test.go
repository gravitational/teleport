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

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/operator/apis/resources/v3"
	"github.com/gravitational/teleport/operator/controllers/resources/testlib"
)

var githubSpec = types.GithubConnectorSpecV3{
	ClientID:      "client id",
	ClientSecret:  "client secret",
	RedirectURL:   "https://redirect",
	TeamsToLogins: nil,
	Display:       "",
	TeamsToRoles: []types.TeamRolesMapping{{
		Organization: "test",
		Team:         "test",
		Roles:        []string{"test"},
	}},
}

type githubTestingPrimitives struct {
	setup *testSetup
}

func (g *githubTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *githubTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *githubTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	github, err := types.NewGithubConnector(name, githubSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	github.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.UpsertGithubConnector(ctx, github))
}

func (g *githubTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.GithubConnector, error) {
	return g.setup.TeleportClient.GetGithubConnector(ctx, name, true)
}

func (g *githubTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteGithubConnector(ctx, name))
}

func (g *githubTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv3.TeleportGithubConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv3.TeleportGithubConnectorSpec(githubSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, github))
}

func (g *githubTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv3.TeleportGithubConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, github))
}

func (g *githubTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv3.TeleportGithubConnector, error) {
	github := &resourcesv3.TeleportGithubConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, github)
	return github, trace.Wrap(err)
}

func (g *githubTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	github, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	github.Spec.TeamsToRoles[0].Roles = []string{"foo", "bar"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, github))
}

func (g *githubTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.GithubConnector, kubeResource *resourcesv3.TeleportGithubConnector) (bool, string) {
	teleportMap, _ := teleportResourceToMap(tResource)
	kubernetesMap, _ := teleportResourceToMap(kubeResource.ToTeleport())

	equal := cmp.Equal(teleportMap["spec"], kubernetesMap["spec"])
	if !equal {
		return equal, cmp.Diff(teleportMap["spec"], kubernetesMap["spec"])
	}

	return equal, ""
}

func TestGithubConnectorCreation(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceCreationTest[types.GithubConnector, *resourcesv3.TeleportGithubConnector](t, test)
}

func TestGithubConnectorDeletionDrift(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.GithubConnector, *resourcesv3.TeleportGithubConnector](t, test)
}

func TestGithubConnectorUpdate(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceUpdateTest[types.GithubConnector, *resourcesv3.TeleportGithubConnector](t, test)
}
