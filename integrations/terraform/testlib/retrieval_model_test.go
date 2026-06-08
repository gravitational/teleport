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
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
)

func (s *TerraformSuiteEnterprise) TestRetrievalModel() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.SummarizerClient().GetRetrievalModel(ctx)
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}
		return nil
	}

	name := "teleport_retrieval_model.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("retrieval_model_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindRetrievalModel),
					resource.TestCheckResourceAttr(name, "spec.bedrock.region", "us-west-2"),
					resource.TestCheckResourceAttr(name, "spec.bedrock.bedrock_model_id", "amazon.titan-embed-text-v2:0"),
					resource.TestCheckResourceAttr(name, "spec.inference_model_name", "bedrock-model"),
				),
			},
			{
				Config:   s.getFixture("retrieval_model_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("retrieval_model_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.bedrock.region", "us-east-1"),
					resource.TestCheckResourceAttr(name, "spec.inference_model_name", "bedrock-model"),
				),
			},
			{
				Config:   s.getFixture("retrieval_model_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportRetrievalModel() {
	t := s.T()
	ctx := t.Context()

	s.createInferenceModel(ctx, "bedrock-model")

	r := "teleport_retrieval_model"
	id := "test_import"
	name := r + "." + id

	model := summarizer.NewRetrievalModel(&summarizerv1.RetrievalModelSpec{
		EmbeddingsProvider: &summarizerv1.RetrievalModelSpec_Bedrock{
			Bedrock: &summarizerv1.BedrockProvider{
				Region:         "us-west-2",
				BedrockModelId: "amazon.titan-embed-text-v2:0",
			},
		},
		InferenceModelName: "bedrock-model",
	})

	_, err := s.client.SummarizerClient().CreateRetrievalModel(ctx, model)
	s.Require().NoError(err)
	t.Cleanup(func() {
		_ = s.client.SummarizerClient().DeleteRetrievalModel(context.Background())
	})

	s.Require().Eventually(func() bool {
		_, err := s.client.SummarizerClient().GetRetrievalModel(ctx)
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
					s.Require().Equal(types.KindRetrievalModel, state[0].Attributes["kind"])
					s.Require().Equal("us-west-2", state[0].Attributes["spec.bedrock.region"])
					s.Require().Equal("bedrock-model", state[0].Attributes["spec.inference_model_name"])
					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) createInferenceModel(ctx context.Context, name string) {
	s.T().Helper()
	model := summarizer.NewInferenceModel(name, &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "gpt-4",
			},
		},
	})
	_, err := s.client.SummarizerClient().UpsertInferenceModel(ctx, model)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		// Use a fresh context: t.Context() is canceled before cleanups run.
		_ = s.client.SummarizerClient().DeleteInferenceModel(context.Background(), name)
	})
}
