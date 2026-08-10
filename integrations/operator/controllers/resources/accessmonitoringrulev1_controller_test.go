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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var accessMonitoringRuleSpec = &accessmonitoringrulesv1pb.AccessMonitoringRuleSpec{
	Subjects:  []string{types.KindAccessRequest},
	Condition: "access_request.spec.roles.contains(\"your_role_name\")",
	Notification: &accessmonitoringrulesv1pb.Notification{
		Name:       "slack",
		Recipients: []string{"your-slack-channel"},
	},
}

type accessMonitoringRuleTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*accessmonitoringrulesv1pb.AccessMonitoringRule]
}

func (g *accessMonitoringRuleTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *accessMonitoringRuleTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *accessMonitoringRuleTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	accessMonitoringRule := &accessmonitoringrulesv1pb.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: accessMonitoringRuleSpec,
	}
	_, err := g.setup.TeleportClient.AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, accessMonitoringRule)
	return trace.Wrap(err)
}

func (g *accessMonitoringRuleTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*accessmonitoringrulesv1pb.AccessMonitoringRule, error) {
	resp, err := g.setup.TeleportClient.AccessMonitoringRulesClient().
		GetAccessMonitoringRule(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *accessMonitoringRuleTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, name))
}

func (g *accessMonitoringRuleTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	accessMonitoringRule := &resourcesv1.TeleportAccessMonitoringRuleV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportAccessMonitoringRuleV1Spec)(accessMonitoringRuleSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, accessMonitoringRule))
}

func (g *accessMonitoringRuleTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	accessMonitoringRule := &resourcesv1.TeleportAccessMonitoringRuleV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, accessMonitoringRule))
}

func (g *accessMonitoringRuleTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportAccessMonitoringRuleV1, error) {
	accessMonitoringRule := &resourcesv1.TeleportAccessMonitoringRuleV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, accessMonitoringRule)
	return accessMonitoringRule, trace.Wrap(err)
}

func (g *accessMonitoringRuleTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	accessMonitoringRule, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	accessMonitoringRule.Spec.Notification.Recipients = []string{"your-slack-channel", "your-other-slack-channel"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, accessMonitoringRule))
}

func (g *accessMonitoringRuleTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *accessmonitoringrulesv1pb.AccessMonitoringRule, kubeResource *resourcesv1.TeleportAccessMonitoringRuleV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func TestAccessMonitoringRuleCreation(t *testing.T) {
	test := &accessMonitoringRuleTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest[*accessmonitoringrulesv1pb.AccessMonitoringRule, *resourcesv1.TeleportAccessMonitoringRuleV1](
		t,
		resources.NewAccessMonitoringRuleV1Reconciler,
		test,
	)
}

func TestAccessMonitoringRuleDeletionDrift(t *testing.T) {
	test := &accessMonitoringRuleTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest[*accessmonitoringrulesv1pb.AccessMonitoringRule, *resourcesv1.TeleportAccessMonitoringRuleV1](
		t,
		resources.NewAccessMonitoringRuleV1Reconciler,
		test,
	)
}

func TestAccessMonitoringRuleUpdate(t *testing.T) {
	test := &accessMonitoringRuleTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous[*accessmonitoringrulesv1pb.AccessMonitoringRule, *resourcesv1.TeleportAccessMonitoringRuleV1](
		t,
		resources.NewAccessMonitoringRuleV1Reconciler,
		test,
	)
}
