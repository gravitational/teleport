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

package testlib

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

type loginRuleTestingPrimitives struct {
	setup *TestSetup
	reconcilers.ResourceWithoutLabelsAdapter[*resourcesv1.LoginRuleResource]
}

func (l *loginRuleTestingPrimitives) Init(setup *TestSetup) {
	l.setup = setup
}

func (l *loginRuleTestingPrimitives) SetupTeleportFixtures(context.Context) error {
	return nil
}

func (l *loginRuleTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	rule := loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: name,
		},
		Version:  "v1",
		Priority: 1,
		TraitsMap: map[string]*wrappers.StringValues{
			"logins": &wrappers.StringValues{
				Values: []string{"external.logins"},
			},
			"groups": &wrappers.StringValues{
				Values: []string{"external.groups"},
			},
		},
	}
	rule.Metadata.SetOrigin(types.OriginKubernetes)
	_, err := l.setup.TeleportClient.LoginRuleClient().CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
		LoginRule: &rule,
	})
	return trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*resourcesv1.LoginRuleResource, error) {
	lrClient := l.setup.TeleportClient.LoginRuleClient()
	loginRule, err := lrClient.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
		Name: name,
	})
	return &resourcesv1.LoginRuleResource{LoginRule: loginRule}, trail.FromGRPC(err)
}

func (l *loginRuleTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	lrClient := l.setup.TeleportClient.LoginRuleClient()
	_, err := lrClient.DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	rule := resourcesv1.TeleportLoginRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: l.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportLoginRuleSpec{
			Priority: 1,
			TraitsMap: map[string][]string{
				"logins": {"external.logins"},
				"groups": {"external.groups"},
			},
		},
	}
	return trace.Wrap(l.setup.K8sClient.Create(ctx, &rule))
}

func (l *loginRuleTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	rule := resourcesv1.TeleportLoginRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: l.setup.Namespace.Name,
		},
	}
	return trace.Wrap(l.setup.K8sClient.Delete(ctx, &rule))
}

func (l *loginRuleTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportLoginRule, error) {
	rule := &resourcesv1.TeleportLoginRule{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: l.setup.Namespace.Name,
	}
	err := l.setup.K8sClient.Get(ctx, obj, rule)
	return rule, trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	rule, err := l.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	rule.Spec.TraitsMap["logins"] = []string{`external.logins.add("test")`}
	return trace.Wrap(l.setup.K8sClient.Update(ctx, rule))
}

func (l *loginRuleTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *resourcesv1.LoginRuleResource,
	kubeResource *resourcesv1.TeleportLoginRule) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(),
		CompareOptions(cmpopts.IgnoreUnexported(loginrulepb.LoginRule{}))...,
	)
	return diff == "", diff
}

func LoginRuleCreationTest(t *testing.T, clt *client.Client) {
	test := &loginRuleTestingPrimitives{}
	ResourceCreationTest[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule](t, test, WithTeleportClient(clt))
}

func LoginRuleDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &loginRuleTestingPrimitives{}
	ResourceDeletionDriftTest[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule](t, test, WithTeleportClient(clt))
}

func LoginRuleUpdateTest(t *testing.T, clt *client.Client) {
	test := &loginRuleTestingPrimitives{}
	ResourceUpdateTest[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule](t, test, WithTeleportClient(clt))
}
