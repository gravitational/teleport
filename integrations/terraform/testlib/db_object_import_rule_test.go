/*
Copyright 2015-2026 Gravitational, Inc.

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

package testlib

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestDatabaseObjectImportRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.DatabaseObjectImportRuleClient().GetDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{Name: "my-custom-rule"})
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_db_object_import_rule.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("db_object_import_rule_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindDatabaseObjectImportRule),
					resource.TestCheckResourceAttr(name, "spec.priority", "123"),
					resource.TestCheckResourceAttr(name, "spec.database_labels.0.name", "env"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.add_labels.database", "{{obj.database}}"),
				),
			},
			{
				Config:   s.getFixture("db_object_import_rule_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("db_object_import_rule_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindDatabaseObjectImportRule),
					resource.TestCheckResourceAttr(name, "spec.priority", "124"),
					resource.TestCheckResourceAttr(name, "spec.database_labels.0.name", "env"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.add_labels.database", "{{obj.database}}"),
				),
			},
			{
				Config:   s.getFixture("db_object_import_rule_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportDatabaseObjectImportRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_db_object_import_rule"
	id := "test_import"
	name := r + "." + id

	importRule := &dbobjectimportrulev1.DatabaseObjectImportRule{
		Kind:    types.KindDatabaseObjectImportRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
			Priority: 123,
			DatabaseLabels: []*labelv1.Label{
				{
					Name:   "label_a",
					Values: []string{"value_a1", "value_a2"},
				},
				{
					Name:   "label_b",
					Values: []string{"value_b1", "value_b2"},
				},
			},
			Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{
				{
					AddLabels: map[string]string{
						"database":              "{{obj.database}}",
						"object_kind":           "{{obj.object_kind}}",
						"name":                  "{{obj.name}}",
						"protocol":              "{{obj.protocol}}",
						"schema":                "{{obj.schema}}",
						"database_service_name": "{{obj.database_service_name}}",
						"fixed":                 "const_value",
						"template":              "foo-{{obj.name}}",
					},
					Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
						TableNames: []string{
							"fixed_table_name",
							"partial_wildcard_*",
						},
					},
					Scope: &dbobjectimportrulev1.DatabaseObjectImportScope{
						DatabaseNames: []string{"Widget*"},
						SchemaNames:   []string{"public", "secret"},
					},
				},
				{
					AddLabels: map[string]string{
						"confidential": "true",
					},
					Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
						TableNames: []string{"*"},
					},
					Scope: &dbobjectimportrulev1.DatabaseObjectImportScope{
						SchemaNames: []string{"secret"},
					},
				},
			},
		},
	}

	var err error
	importRule, err = s.client.DatabaseObjectImportRuleClient().
		CreateDatabaseObjectImportRule(ctx,
			&dbobjectimportrulev1.CreateDatabaseObjectImportRuleRequest{Rule: importRule},
		)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		rule, err := s.client.DatabaseObjectImportRuleClient().
			GetDatabaseObjectImportRule(ctx,
				&dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{Name: importRule.GetMetadata().GetName()},
			)
		fmt.Println(rule)
		if trace.IsNotFound(err) {
			return false
		}
		assert.NoError(s.T(), err)
		return true
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" {` + "\n spec={}\n}",
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), types.KindDatabaseObjectImportRule, state[0].Attributes["kind"])
					require.Equal(s.T(), "123", state[0].Attributes["spec.priority"])
					require.Equal(s.T(), "label_a", state[0].Attributes["spec.database_labels.0.name"])
					require.Equal(s.T(), "2", state[0].Attributes["spec.database_labels.0.values.#"])
					require.Equal(s.T(), "value_a2", state[0].Attributes["spec.database_labels.0.values.1"])

					return nil
				},
			},
		},
	})
}
