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
	"regexp"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
)

func (s *TerraformSuiteEnterprise) TestInferenceSecret() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.SummarizerClient().GetInferenceSecret(ctx, "test-secret")
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}

		return nil
	}

	name := "teleport_inference_secret.test-secret"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("inference_secret_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindInferenceSecret),
					resource.TestCheckResourceAttr(name, "spec.value", "secret-api-key"),
				),
				// Plan will have to be non-empty since the inference secret spec is
				// never returned from gRPC, so Terraform always assumes it needs to be
				// updated.
				ExpectNonEmptyPlan: true,
			},
			{
				Config: s.getFixture("inference_secret_0_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindInferenceSecret),
					resource.TestCheckResourceAttr(name, "spec.value", "updated-api-key"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportInferenceSecret() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_inference_secret"
	id := "test_import"
	name := r + "." + id

	secret := summarizer.NewInferenceSecret(id, &summarizerv1.InferenceSecretSpec{
		Value: "top secret",
	})

	secret, err := s.client.SummarizerClient().CreateInferenceSecret(ctx, secret)
	s.Require().NoError(err)

	s.Require().Eventually(func() bool {
		_, err := s.client.SummarizerClient().GetInferenceSecret(ctx, secret.GetMetadata().GetName())
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
				// The gRPC server doesn't return the secret value, therefore importing
				// should not be allowed.
				ExpectError: regexp.MustCompile("Resource Import Not Implemented"),
			},
		},
	})
}
