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
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var autoUpdateVersionSpec = &autoupdatev1pb.AutoUpdateVersionSpec{
	Tools: nil,
	Agents: &autoupdatev1pb.AutoUpdateVersionSpecAgents{
		StartVersion:  "1.2.3",
		TargetVersion: "1.2.4",
		Schedule:      autoupdate.AgentsScheduleRegular,
		Mode:          autoupdate.AgentsUpdateModeEnabled,
	},
}

type autoUpdateVersionTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*autoupdatev1pb.AutoUpdateVersion]
}

func (g *autoUpdateVersionTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *autoUpdateVersionTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *autoUpdateVersionTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	autoUpdateVersion := &autoupdatev1pb.AutoUpdateVersion{
		Kind:    types.KindAutoUpdateVersion,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateVersion,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: autoUpdateVersionSpec,
	}
	_, err := g.setup.TeleportClient.
		CreateAutoUpdateVersion(ctx, autoUpdateVersion)
	return trace.Wrap(err)
}

func (g *autoUpdateVersionTestingPrimitives) GetTeleportResource(ctx context.Context, _ string) (*autoupdatev1pb.AutoUpdateVersion, error) {
	resp, err := g.setup.TeleportClient.
		GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *autoUpdateVersionTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteAutoUpdateVersion(ctx))
}

func (g *autoUpdateVersionTestingPrimitives) CreateKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateVersion := &resourcesv1.TeleportAutoupdateVersionV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.MetaNameAutoUpdateVersion,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportAutoupdateVersionV1Spec)(autoUpdateVersionSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, autoUpdateVersion))
}

func (g *autoUpdateVersionTestingPrimitives) DeleteKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateVersion := &resourcesv1.TeleportAutoupdateVersionV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.MetaNameAutoUpdateVersion,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, autoUpdateVersion))
}

func (g *autoUpdateVersionTestingPrimitives) GetKubernetesResource(ctx context.Context, _ string) (*resourcesv1.TeleportAutoupdateVersionV1, error) {
	autoUpdateVersion := &resourcesv1.TeleportAutoupdateVersionV1{}
	obj := kclient.ObjectKey{
		Name:      types.MetaNameAutoUpdateVersion,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, autoUpdateVersion)
	return autoUpdateVersion, trace.Wrap(err)
}

func (g *autoUpdateVersionTestingPrimitives) ModifyKubernetesResource(ctx context.Context, _ string) error {
	autoUpdateVersion, err := g.GetKubernetesResource(ctx, types.MetaNameAutoUpdateVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	autoUpdateVersion.Spec.Agents.Mode = autoupdate.AgentsUpdateModeSuspended
	autoUpdateVersion.Spec.Tools = &autoupdatev1pb.AutoUpdateVersionSpecTools{
		TargetVersion: "1.2.4",
	}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, autoUpdateVersion))
}

func (g *autoUpdateVersionTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *autoupdatev1pb.AutoUpdateVersion, kubeResource *resourcesv1.TeleportAutoupdateVersionV1) (bool, string) {
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

func TestAutoUpdateVersionCreation(t *testing.T) {
	test := &autoUpdateVersionTestingPrimitives{}
	testlib.ResourceCreationTest[*autoupdatev1pb.AutoUpdateVersion, *resourcesv1.TeleportAutoupdateVersionV1](t, test)
}

func TestAutoUpdateVersionDeletionDrift(t *testing.T) {
	test := &autoUpdateVersionTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[*autoupdatev1pb.AutoUpdateVersion, *resourcesv1.TeleportAutoupdateVersionV1](t, test)
}

func TestAutoUpdateVersionUpdate(t *testing.T) {
	test := &autoUpdateVersionTestingPrimitives{}
	testlib.ResourceUpdateTest[*autoupdatev1pb.AutoUpdateVersion, *resourcesv1.TeleportAutoupdateVersionV1](t, test)
}
