/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by locklicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestLock() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetLock(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_lock.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("lock_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "lock"),
					resource.TestCheckResourceAttr(name, "spec.target.user", "john"),
				),
			},
			{
				Config:   s.getFixture("lock_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("lock_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "lock"),
					resource.TestCheckResourceAttr(name, "spec.target.user", "eve"),
				),
			},
			{
				Config:   s.getFixture("lock_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportLock() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_lock"
	id := "test_import"
	name := r + "." + id

	lock := &types.LockV2{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.LockSpecV2{
			Target: types.LockTarget{
				User: "john",
			},
		},
	}
	err := lock.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.UpsertLock(ctx, lock)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetLock(ctx, lock.GetName())
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(s.T(), err)
		return true
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "lock", state[0].Attributes["kind"])
					require.Equal(s.T(), "john", state[0].Attributes["spec.target.user"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSWithCache) TestLockWithCache() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetLock(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_lock.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("lock_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "lock"),
					resource.TestCheckResourceAttr(name, "spec.target.user", "john"),
				),
			},
			{
				Config:   s.getFixture("lock_0_create.tf"),
				PlanOnly: true,
			},
		},
	})
}
