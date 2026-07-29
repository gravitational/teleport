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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/defaults"
)

var botSpec = machineidv1.BotSpec_builder{
	Roles: []string{"roleA", "roleB"},
	Traits: []*machineidv1.Trait{
		machineidv1.Trait_builder{
			Name:   "traitA",
			Values: []string{"valueA", "valueB"},
		}.Build(),
		machineidv1.Trait_builder{
			Name:   "traitB",
			Values: []string{"valueC", "valueD"},
		}.Build(),
	},
	// Note: The server-side resource will have the default value filled in, so
	// we need to explicitly set it to ensure comparisons succeed.
	MaxSessionTtl: durationpb.New(defaults.DefaultBotMaxSessionTTL),
}.Build()

var scopedBotSpec = &machineidv1.BotSpec{}

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
	var spec *machineidv1.BotSpec
	if scope := g.setup.OperatorMetadata().Scope; scope != "" {
		spec = scopedBotSpec
	} else {
		spec = botSpec
	}
	labels := map[string]string{
		types.OriginLabel: types.OriginKubernetes,
	}
	if g.setup.OperatorMetadata().Scope != "" {
		labels[reconcilers.OperatorIDLabel] = g.setup.OperatorMetadata().ID
	}
	bot := machineidv1.Bot_builder{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:   name,
			Labels: labels,
		}.Build(),
		Spec:  spec,
		Scope: g.setup.OperatorMetadata().Scope,
	}.Build()
	_, err := g.setup.TeleportClient.
		BotServiceClient().
		CreateBot(ctx, machineidv1.CreateBotRequest_builder{Bot: bot}.Build())
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*machineidv1.Bot, error) {
	resp, err := g.setup.TeleportClient.
		BotServiceClient().
		GetBot(ctx, machineidv1.GetBotRequest_builder{
			BotName: name,
			Scope:   g.setup.OperatorMetadata().Scope,
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *botTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.
		BotServiceClient().
		DeleteBot(ctx, machineidv1.DeleteBotRequest_builder{
			BotName: name,
			Scope:   g.setup.OperatorMetadata().Scope,
		}.Build())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (g *botTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	var spec *machineidv1.BotSpec
	if scope := g.setup.OperatorMetadata().Scope; scope != "" {
		spec = scopedBotSpec
	} else {
		spec = botSpec
	}
	bot := &resourcesv1.TeleportBotV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec:  (*resourcesv1.TeleportBotV1Spec)(spec),
		Scope: g.setup.OperatorMetadata().Scope,
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, bot))
}

func (g *botTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	bot := &resourcesv1.TeleportBotV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, bot))
}

func (g *botTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportBotV1, error) {
	bot := &resourcesv1.TeleportBotV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, bot)
	return bot, trace.Wrap(err)
}

func (g *botTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	// Note: scoped bots hold no data, there's nothing to modify.
	bot, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	bot.Spec.Roles = []string{"changed"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, bot))
}

func (g *botTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *machineidv1.Bot, kubeResource *resourcesv1.TeleportBotV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions(
			protocmp.IgnoreFields(&machineidv1.Bot{}, "status"),
			protocmp.SortRepeated(func(a, b *machineidv1.Trait) bool {
				return strings.Compare(a.GetName(), b.GetName()) == -1
			}),
		)...,
	)
	return diff == "", diff
}

func TestBotCreation(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test)
}

func TestBotDeletion(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test)
}

func TestBotDeletionDrift(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test)
}

func TestBotUpdate(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test)
}

func TestScopedBotCreation(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test, testlib.WithScope(testScope))
}

func TestScopedBotDeletion(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test, testlib.WithScope(testScope))
}

func TestScopedBotDeletionDrift(t *testing.T) {
	test := &botTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](t, resources.NewBotV1Reconciler, test, testlib.WithScope(testScope))
}

// Note: scoped bots hold no data, there's nothing to update, hence no update tests.
