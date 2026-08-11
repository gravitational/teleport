// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testlib

import (
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestWorkloadIdentity() {
	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetWorkloadIdentity(s.T().Context(), "test")
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Errorf("expected not found, actual: %v", err)
	}

	name := "teleport_workload_identity.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("workload_identity_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "workload_identity"),
					resource.TestCheckResourceAttr(name, "spec.spiffe.id", "/test"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.attribute", "user.name"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.eq.value", "foo"),
				),
			},
			{
				Config:   s.getFixture("workload_identity_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("workload_identity_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "workload_identity"),
					resource.TestCheckResourceAttr(name, "spec.spiffe.id", "/test/updated"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.attribute", "user.name"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.eq.value", "foo"),
				),
			},
			{
				Config:   s.getFixture("workload_identity_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSSScopedResources) TestWorkloadIdentityWithScope() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.WorkloadIdentityResourceServiceClient().GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name:  "test-scoped",
			Scope: "/staging",
		})
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Errorf("expected not found, actual: %v", err)
	}

	name := "teleport_workload_identity.test_scoped"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("workload_identity_scoped_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "workload_identity"),
					resource.TestCheckResourceAttr(name, "scope", "/staging"),
					resource.TestCheckResourceAttr(name, "spec.spiffe.id", "/staging/_/test"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.attribute", "user.name"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.eq.value", "foo"),
				),
			},
			{
				Config:   s.getFixture("workload_identity_scoped_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("workload_identity_scoped_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "workload_identity"),
					resource.TestCheckResourceAttr(name, "scope", "/staging"),
					resource.TestCheckResourceAttr(name, "spec.spiffe.id", "/staging/_/test/updated"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.attribute", "user.name"),
					resource.TestCheckResourceAttr(name, "spec.rules.allow.0.conditions.0.eq.value", "foo"),
				),
			},
			{
				Config:   s.getFixture("workload_identity_scoped_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportWorkloadIdentity() {
	r := "teleport_workload_identity"
	id := "test_import"
	name := r + "." + id

	identity := &workloadidentityv1pb.WorkloadIdentity{
		Metadata: &v1.Metadata{
			Name: id,
		},
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "user.name",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "foo",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/test",
			},
		},
	}
	identity, err := s.client.CreateWorkloadIdentity(s.T().Context(), identity)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetWorkloadIdentity(s.T().Context(), identity.GetMetadata().Name)
		return err == nil
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        fmt.Sprintf("%s\nresource %q %q { }", s.terraformConfig, r, id),
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), types.KindWorkloadIdentity, state[0].Attributes["kind"])
					require.Equal(s.T(), "/test", state[0].Attributes["spec.spiffe.id"])
					require.Equal(s.T(), "user.name", state[0].Attributes["spec.rules.allow.0.conditions.0.attribute"])
					require.Equal(s.T(), "foo", state[0].Attributes["spec.rules.allow.0.conditions.0.eq.value"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSScopedResources) TestImportWorkloadIdentityWithScope() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_workload_identity"
	id := "test_import_scoped"
	name := r + "." + id

	identity := &workloadidentityv1pb.WorkloadIdentity{
		Metadata: &v1.Metadata{
			Name: id,
		},
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Scope:   "/staging",
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "user.name",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "foo",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/staging/_/test_import_scoped",
			},
		},
	}
	identity, err := s.client.CreateWorkloadIdentity(ctx, identity)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := s.client.WorkloadIdentityResourceServiceClient().GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name:  identity.GetMetadata().GetName(),
			Scope: identity.GetScope(),
		})
		return err == nil
	}, 5*time.Second, time.Second)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        fmt.Sprintf("%s\nresource %q %q { }", s.terraformConfig, r, id),
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: "/staging::" + id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(t, types.KindWorkloadIdentity, state[0].Attributes["kind"])
					require.Equal(t, "/staging", state[0].Attributes["scope"])
					require.Equal(t, "/staging/_/test_import_scoped", state[0].Attributes["spec.spiffe.id"])
					require.Equal(t, "user.name", state[0].Attributes["spec.rules.allow.0.conditions.0.attribute"])
					require.Equal(t, "foo", state[0].Attributes["spec.rules.allow.0.conditions.0.eq.value"])

					return nil
				},
			},
		},
	})
}
