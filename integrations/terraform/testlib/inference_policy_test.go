// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testlib

import (
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
)

func (s *TerraformSuiteEnterprise) TestInferencePolicy() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.SummarizerClient().GetInferencePolicy(ctx, "test-policy")
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}

		return nil
	}

	name := "teleport_inference_policy.test-policy"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("inference_policy_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindInferencePolicy),
					resource.TestCheckResourceAttr(name, "spec.kinds.0", "ssh"),
					resource.TestCheckResourceAttr(name, "spec.kinds.1", "k8s"),
					resource.TestCheckResourceAttr(name, "spec.model", "dummy-model"),
					resource.TestCheckResourceAttr(name, "spec.filter", "equals(resource.metadata.labels[\"env\"], \"prod\")"),
				),
			},
			{
				Config:   s.getFixture("inference_policy_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("inference_policy_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.kinds.1", "db"),
					resource.TestCheckResourceAttr(name, "spec.model", "another-dummy-model"),
					resource.TestCheckNoResourceAttr(name, "spec.filter"),
				),
			},
			{
				Config:   s.getFixture("inference_policy_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportInferencePolicy() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_inference_policy"
	id := "test_import"
	name := r + "." + id

	policy := summarizer.NewInferencePolicy(id, &summarizerv1.InferencePolicySpec{
		Kinds: []string{"db"},
		Model: "some-model",
	})

	policy, err := s.client.SummarizerClient().CreateInferencePolicy(ctx, policy)
	s.Require().NoError(err)

	s.Require().Eventually(func() bool {
		_, err := s.client.SummarizerClient().GetInferencePolicy(ctx, policy.GetMetadata().GetName())
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
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					s.Require().Equal(types.KindInferencePolicy, state[0].Attributes["kind"])
					s.Require().Equal("db", state[0].Attributes["spec.kinds.0"])
					s.Require().Equal("some-model", state[0].Attributes["spec.model"])
					return nil
				},
			},
		},
	})
}
