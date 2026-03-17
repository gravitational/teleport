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

func (s *TerraformSuiteEnterprise) TestInferenceModel() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.SummarizerClient().GetInferenceModel(ctx, "bedrock-model")
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}

		_, err = s.client.SummarizerClient().GetInferenceModel(ctx, "openai-model")
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}

		return nil
	}

	bedrockName := "teleport_inference_model.bedrock"
	openAIName := "teleport_inference_model.openai"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("inference_model_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(bedrockName, "kind", types.KindInferenceModel),
					resource.TestCheckResourceAttr(bedrockName, "spec.bedrock.region", "us-west-2"),
					resource.TestCheckResourceAttr(bedrockName, "spec.bedrock.bedrock_model_id", "us.amazon.nova-lite-v1:0"),
					resource.TestCheckResourceAttr(bedrockName, "spec.max_session_length_bytes", "100000"),

					resource.TestCheckResourceAttr(openAIName, "kind", types.KindInferenceModel),
					resource.TestCheckResourceAttr(openAIName, "spec.openai.base_url", "http://localhost:4000/"),
					resource.TestCheckResourceAttr(openAIName, "spec.openai.openai_model_id", "gpt5"),
				),
			},
			{
				Config:   s.getFixture("inference_model_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("inference_model_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(bedrockName, "spec.bedrock.region", "us-east-1"),
					resource.TestCheckResourceAttr(bedrockName, "spec.max_session_length_bytes", "200000"),

					resource.TestCheckResourceAttr(openAIName, "spec.openai.base_url", "http://localhost:8080/"),
				),
			},
			{
				Config:   s.getFixture("inference_model_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportInferenceModel() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_inference_model"
	id := "test_import"
	name := r + "." + id

	model := summarizer.NewInferenceModel(id, &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "gpt5",
			},
		},
	})

	model, err := s.client.SummarizerClient().CreateInferenceModel(ctx, model)
	s.Require().NoError(err)

	s.Require().Eventually(func() bool {
		_, err := s.client.SummarizerClient().GetInferenceModel(ctx, model.GetMetadata().GetName())
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
					s.Require().Equal(types.KindInferenceModel, state[0].Attributes["kind"])
					s.Require().Equal("gpt5", state[0].Attributes["spec.openai.openai_model_id"])
					return nil
				},
			},
		},
	})
}
