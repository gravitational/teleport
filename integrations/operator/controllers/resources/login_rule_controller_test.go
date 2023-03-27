// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	v1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
)

type loginRuleTestingPrimitives struct {
	setup *testSetup
}

func (l *loginRuleTestingPrimitives) init(setup *testSetup) {
	l.setup = setup
}

func (l *loginRuleTestingPrimitives) setupTeleportFixtures(context.Context) error {
	return nil
}

func (l *loginRuleTestingPrimitives) createTeleportResource(ctx context.Context, name string) error {
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
	_, err := l.setup.tClient.LoginRuleClient().CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
		LoginRule: &rule,
	})
	return trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) getTeleportResource(ctx context.Context, name string) (*v1.LoginRuleResource, error) {
	lrClient := l.setup.tClient.LoginRuleClient()
	loginRule, err := lrClient.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
		Name: name,
	})
	return &resourcesv1.LoginRuleResource{LoginRule: loginRule}, trail.FromGRPC(err)
}

func (l *loginRuleTestingPrimitives) deleteTeleportResource(ctx context.Context, name string) error {
	lrClient := l.setup.tClient.LoginRuleClient()
	_, err := lrClient.DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) createKubernetesResource(ctx context.Context, name string) error {
	rule := v1.TeleportLoginRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: l.setup.namespace.Name,
		},
		Spec: v1.TeleportLoginRuleSpec{
			Priority: 1,
			TraitsMap: map[string][]string{
				"logins": {"external.logins"},
				"groups": {"external.groups"},
			},
		},
	}
	return trace.Wrap(l.setup.k8sClient.Create(ctx, &rule))
}

func (l *loginRuleTestingPrimitives) deleteKubernetesResource(ctx context.Context, name string) error {
	rule := v1.TeleportLoginRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: l.setup.namespace.Name,
		},
	}
	return trace.Wrap(l.setup.k8sClient.Delete(ctx, &rule))
}

func (l *loginRuleTestingPrimitives) getKubernetesResource(ctx context.Context, name string) (*v1.TeleportLoginRule, error) {
	rule := &v1.TeleportLoginRule{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: l.setup.namespace.Name,
	}
	err := l.setup.k8sClient.Get(ctx, obj, rule)
	return rule, trace.Wrap(err)
}

func (l *loginRuleTestingPrimitives) modifyKubernetesResource(ctx context.Context, name string) error {
	rule, err := l.getKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	rule.Spec.TraitsMap["logins"] = []string{`external.logins.add("test")`}
	return trace.Wrap(l.setup.k8sClient.Update(ctx, rule))
}

func (l *loginRuleTestingPrimitives) compareTeleportAndKubernetesResource(tResource *v1.LoginRuleResource, kubeResource *v1.TeleportLoginRule) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(),
		cmpopts.IgnoreUnexported(loginrulepb.LoginRule{}),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Labels"),
	)
	return diff == "", diff
}

func TestLoginRuleCreation(t *testing.T) {
	if os.Getenv("OPERATOR_TEST_TELEPORT_ADDR") == "" {
		t.Skip("test environment does not support login rules")
	}
	test := &loginRuleTestingPrimitives{}
	testResourceCreation[*v1.LoginRuleResource, *v1.TeleportLoginRule](t, test)
}

func TestLoginRuleDeletionDrift(t *testing.T) {
	if os.Getenv("OPERATOR_TEST_TELEPORT_ADDR") == "" {
		t.Skip("test environment does not support login rules")
	}
	test := &loginRuleTestingPrimitives{}
	testResourceDeletionDrift[*v1.LoginRuleResource, *resourcesv1.TeleportLoginRule](t, test)
}

func TestLoginRuleConnectorUpdate(t *testing.T) {
	if os.Getenv("OPERATOR_TEST_TELEPORT_ADDR") == "" {
		t.Skip("test environment does not support login rules")
	}
	test := &loginRuleTestingPrimitives{}
	testResourceUpdate[*v1.LoginRuleResource, *resourcesv1.TeleportLoginRule](t, test)
}
