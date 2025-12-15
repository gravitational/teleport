/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var oktaImportRuleSpec = types.OktaImportRuleSpecV1{
	Priority: 100,
	Mappings: []*types.OktaImportRuleMappingV1{
		{
			Match: []*types.OktaImportRuleMatchV1{
				{
					AppIDs: []string{"1", "2", "3"},
				},
			},
			AddLabels: map[string]string{
				"label1": "value1",
			},
		},
		{
			Match: []*types.OktaImportRuleMatchV1{
				{
					GroupIDs: []string{"1", "2", "3"},
				},
			},
			AddLabels: map[string]string{
				"label2": "value2",
			},
		},
	},
}

type oktaImportRuleTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.OktaImportRule]
}

func (g *oktaImportRuleTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *oktaImportRuleTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *oktaImportRuleTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	importRule, err := types.NewOktaImportRule(types.Metadata{
		Name: name,
	}, oktaImportRuleSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	importRule.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.OktaClient().CreateOktaImportRule(ctx, importRule)
	return trace.Wrap(err)
}

func (g *oktaImportRuleTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.OktaImportRule, error) {
	return g.setup.TeleportClient.OktaClient().GetOktaImportRule(ctx, name)
}

func (g *oktaImportRuleTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.OktaClient().DeleteOktaImportRule(ctx, name))
}

func (g *oktaImportRuleTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	spec := resourcesv1.TeleportOktaImportRuleSpec{
		Priority: oktaImportRuleSpec.Priority,
		Mappings: make([]resourcesv1.TeleportOktaImportRuleMapping, len(oktaImportRuleSpec.Mappings)),
	}

	for i, mapping := range oktaImportRuleSpec.Mappings {
		matches := make([]resourcesv1.TeleportOktaImportRuleMatch, len(mapping.Match))
		for j, match := range mapping.Match {
			matches[j] = resourcesv1.TeleportOktaImportRuleMatch{
				AppIDs:           match.AppIDs,
				GroupIDs:         match.GroupIDs,
				AppNameRegexes:   match.AppNameRegexes,
				GroupNameRegexes: match.GroupNameRegexes,
			}
		}
		spec.Mappings[i] = resourcesv1.TeleportOktaImportRuleMapping{
			Match:     matches,
			AddLabels: maps.Clone(mapping.AddLabels),
		}
	}

	importRule := &resourcesv1.TeleportOktaImportRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: spec,
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, importRule))
}

func (g *oktaImportRuleTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	oidc := &resourcesv1.TeleportOktaImportRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, oidc))
}

func (g *oktaImportRuleTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportOktaImportRule, error) {
	importRule := &resourcesv1.TeleportOktaImportRule{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, importRule)
	return importRule, trace.Wrap(err)
}

func (g *oktaImportRuleTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	importRule, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	importRule.Spec.Priority = 50
	return g.setup.K8sClient.Update(ctx, importRule)
}

func (g *oktaImportRuleTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.OktaImportRule, kubeResource *resourcesv1.TeleportOktaImportRule) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestOktaImportRuleCreation(t *testing.T) {
	t.Skip("Skipping test since okta reconsider is not available in OSS")
	test := &oktaImportRuleTestingPrimitives{}
	testlib.ResourceCreationTest[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule](t, test)
}

func TestOktaImportRuleDeletionDrift(t *testing.T) {
	t.Skip("Skipping test since okta reconsider is not available in OSS")
	test := &oktaImportRuleTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule](t, test)
}

func TestOktaImportRuleUpdate(t *testing.T) {
	t.Skip("Skipping test since okta reconsider is not available in OSS")
	test := &oktaImportRuleTestingPrimitives{}
	testlib.ResourceUpdateTest[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule](t, test)
}
