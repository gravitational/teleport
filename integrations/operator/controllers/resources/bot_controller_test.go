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

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var botSpec = &machineidv1.BotSpec{
	Roles: []string{"roleA", "roleB"},
	Traits: []*machineidv1.Trait{
		{
			Name:   "traitA",
			Values: []string{"valueA", "valueB"},
		},
		{
			Name:   "traitB",
			Values: []string{"valueC", "valueD"},
		},
	},
}

type botTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*machineidv1.Bot]
}

func (g *botTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *botTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *botTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: botSpec,
	}
	_, err := g.setup.TeleportClient.
		BotServiceClient().
		CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: bot})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*machineidv1.Bot, error) {
	resp, err := g.setup.TeleportClient.
		BotServiceClient().
		GetBot(ctx, &machineidv1.GetBotRequest{BotName: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *botTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.
		BotServiceClient().
		DeleteBot(ctx, &machineidv1.DeleteBotRequest{BotName: name})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (g *botTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv1.TeleportBot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportBotSpec)(botSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, github))
}

func (g *botTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv1.TeleportBot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, github))
}

func (g *botTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportBot, error) {
	github := &resourcesv1.TeleportBot{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, github)
	return github, trace.Wrap(err)
}

func (g *botTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	github, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, github))
}

func (g *botTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *machineidv1.Bot, kubeResource *resourcesv1.TeleportBot) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		protocmp.Transform(),
		protocmp.IgnoreFields(&machineidv1.Bot{}, "status"),
	)
	return diff == "", diff
}

func TestBotCreation(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceCreationTest[*machineidv1.Bot, *resourcesv1.TeleportBot](t, test)
}

func TestBotDeletionDrift(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[*machineidv1.Bot, *resourcesv1.TeleportBot](t, test)
}

func TestBotUpdate(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceUpdateTest[*machineidv1.Bot, *resourcesv1.TeleportBot](t, test)
}
