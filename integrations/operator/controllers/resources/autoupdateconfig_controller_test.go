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

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var autoUpdateConfigSpec = autoupdatev1pb.AutoUpdateConfigSpec_builder{
	Tools: nil,
	Agents: autoupdatev1pb.AutoUpdateConfigSpecAgents_builder{
		Mode:     autoupdate.AgentsUpdateModeEnabled,
		Strategy: autoupdate.AgentsStrategyHaltOnError,
		Schedules: autoupdatev1pb.AgentAutoUpdateSchedules_builder{
			Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
				autoupdatev1pb.AgentAutoUpdateGroup_builder{
					Name:      "dev",
					Days:      []string{"*"},
					StartHour: 12,
					WaitHours: 0,
				}.Build(),
				autoupdatev1pb.AgentAutoUpdateGroup_builder{
					Name:      "stage",
					Days:      []string{"*"},
					StartHour: 12,
					WaitHours: 24,
				}.Build(),
				autoupdatev1pb.AgentAutoUpdateGroup_builder{
					Name:      "prod",
					Days:      []string{"Mon", "Tue", "Wed", "Thu"},
					StartHour: 12,
					WaitHours: 24,
				}.Build(),
			},
		}.Build(),
	}.Build(),
}.Build()

type autoUpdateConfigTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*autoupdatev1pb.AutoUpdateConfig]
}

func (g *autoUpdateConfigTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *autoUpdateConfigTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *autoUpdateConfigTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	autoUpdateConfig := autoupdatev1pb.AutoUpdateConfig_builder{
		Kind:    types.KindAutoUpdateConfig,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: types.MetaNameAutoUpdateConfig,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		}.Build(),
		Spec: autoUpdateConfigSpec,
	}.Build()
	_, err := g.setup.TeleportClient.
		CreateAutoUpdateConfig(ctx, autoUpdateConfig)
	return trace.Wrap(err)
}

func (g *autoUpdateConfigTestingPrimitives) GetTeleportResource(ctx context.Context, _ string) (*autoupdatev1pb.AutoUpdateConfig, error) {
	resp, err := g.setup.TeleportClient.
		GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *autoUpdateConfigTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteAutoUpdateConfig(ctx))
}

func (g *autoUpdateConfigTestingPrimitives) CreateKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateConfig := &resourcesv1.TeleportAutoupdateConfigV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.MetaNameAutoUpdateConfig,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportAutoupdateConfigV1Spec)(autoUpdateConfigSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, autoUpdateConfig))
}

func (g *autoUpdateConfigTestingPrimitives) DeleteKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateConfig := &resourcesv1.TeleportAutoupdateConfigV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.MetaNameAutoUpdateConfig,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, autoUpdateConfig))
}

func (g *autoUpdateConfigTestingPrimitives) GetKubernetesResource(ctx context.Context, _ string) (*resourcesv1.TeleportAutoupdateConfigV1, error) {
	autoUpdateConfig := &resourcesv1.TeleportAutoupdateConfigV1{}
	obj := kclient.ObjectKey{
		Name:      types.MetaNameAutoUpdateConfig,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, autoUpdateConfig)
	return autoUpdateConfig, trace.Wrap(err)
}

func (g *autoUpdateConfigTestingPrimitives) ModifyKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateConfig, err := g.GetKubernetesResource(ctx, types.MetaNameAutoUpdateConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	autoUpdateConfig.Spec.Agents.SetMode(autoupdate.AgentsUpdateModeSuspended)
	autoUpdateConfig.Spec.Tools = autoupdatev1pb.AutoUpdateConfigSpecTools_builder{
		Mode: autoupdate.ToolsUpdateModeEnabled,
	}.Build()
	return trace.Wrap(g.setup.K8sClient.Update(ctx, autoUpdateConfig))
}

func (g *autoUpdateConfigTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *autoupdatev1pb.AutoUpdateConfig, kubeResource *resourcesv1.TeleportAutoupdateConfigV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions(
			// Ignoring the name because the resource is a singleton
			protocmp.IgnoreFields(&headerv1.Metadata{}, "name"),
		)...,
	)
	return diff == "", diff
}

func TestAutoUpdateConfigCreation(t *testing.T) {
	test := &autoUpdateConfigTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(
		t,
		resources.NewAutoUpdateConfigV1Reconciler,
		test,
		testlib.WithResourceName(types.MetaNameAutoUpdateConfig),
	)
}

func TestAutoUpdateConfigDeletion(t *testing.T) {
	test := &autoUpdateConfigTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(
		t,
		resources.NewAutoUpdateConfigV1Reconciler,
		test,
		testlib.WithResourceName(types.MetaNameAutoUpdateConfig),
	)
}

func TestAutoUpdateConfigDeletionDrift(t *testing.T) {
	test := &autoUpdateConfigTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(
		t,
		resources.NewAutoUpdateConfigV1Reconciler,
		test,
		testlib.WithResourceName(types.MetaNameAutoUpdateConfig),
	)
}

func TestAutoUpdateConfigUpdate(t *testing.T) {
	test := &autoUpdateConfigTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(
		t,
		resources.NewAutoUpdateConfigV1Reconciler,
		test,
		testlib.WithResourceName(types.MetaNameAutoUpdateConfig),
	)
}
