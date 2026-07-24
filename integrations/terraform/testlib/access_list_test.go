/*
Copyright 2015-2021 Gravitational, Inc.

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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
)

type nextAuditDateComparer struct {
	client        *client.Client
	nextAuditDate time.Time
}

func (c *nextAuditDateComparer) CaptureNextAuditDate(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		al, err := c.client.AccessListClient().GetAccessList(context.TODO(), name)
		if err != nil {
			return trace.Wrap(err)
		}
		c.nextAuditDate = al.Spec.Audit.NextAuditDate
		return nil
	}
}

func (c *nextAuditDateComparer) TestNextAuditDateUnchanged(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		al, err := c.client.AccessListClient().GetAccessList(context.TODO(), name)
		if err != nil {
			return trace.Wrap(err)
		}
		diff := cmp.Diff(c.nextAuditDate, al.Spec.Audit.NextAuditDate, cmpopts.EquateApproxTime(2*time.Millisecond))
		if diff != "" {
			return trace.CompareFailed("NextAuditDate should not have changed, was %s, is now %s", c.nextAuditDate, al.Spec.Audit.NextAuditDate)
		}
		return nil
	}
}

func (s *TerraformSuiteEnterprise) TestAccessList() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	checkAccessListDestroyed := func(state *terraform.State) error {
		_, err := s.client.AccessListClient().GetAccessList(context.TODO(), "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_access_list.test"
	auditDateChecker := nextAuditDateComparer{client: s.client}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkAccessListDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_list_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "header.kind", "access_list"),
					resource.TestCheckResourceAttr(name, "header.version", "v1"),
					resource.TestCheckResourceAttr(name, "header.metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "header.metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.description", "test description"),
					resource.TestCheckResourceAttr(name, "spec.owners.0.name", "gru"),
					resource.TestCheckResourceAttr(name, "spec.owners.0.description", "The supervillain."),
					resource.TestCheckResourceAttr(name, "spec.membership_requires.roles.0", "minion"),
					resource.TestCheckResourceAttr(name, "spec.ownership_requires.roles.0", "supervillain"),
					resource.TestCheckResourceAttr(name, "spec.grants.roles.0", "crane-operator"),
					resource.TestCheckResourceAttr(name, "spec.title", "Hello"),
					resource.TestCheckResourceAttr(name, "spec.audit.recurrence.frequency", "3"),
					auditDateChecker.CaptureNextAuditDate("test"),
				),
			},
			{
				Config:   s.getFixture("access_list_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("access_list_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "header.metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.audit.recurrence.day_of_month", "15"),
					resource.TestCheckResourceAttr(name, "spec.audit.recurrence.frequency", "3"),
					resource.TestCheckResourceAttr(name, "spec.description", "test description"),
					resource.TestCheckResourceAttr(name, "spec.grants.roles.0", "crane-operator"),
					resource.TestCheckResourceAttr(name, "spec.grants.traits.0.key", "allowed-machines"),
					resource.TestCheckResourceAttr(name, "spec.grants.traits.0.values.0", "crane"),
					resource.TestCheckResourceAttr(name, "spec.grants.traits.0.values.1", "forklift"),
					resource.TestCheckResourceAttr(name, "spec.membership_requires.roles.0", "minion"),
					resource.TestCheckResourceAttr(name, "spec.owners.0.description", "The supervillain."),
					resource.TestCheckResourceAttr(name, "spec.owners.0.name", "gru"),
					resource.TestCheckResourceAttr(name, "spec.ownership_requires.roles.0", "supervillain"),
					auditDateChecker.TestNextAuditDateUnchanged("test"),
				),
			},
			{
				Config:   s.getFixture("access_list_1_update.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("access_list_2_expiring.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "header.metadata.expires", "2038-01-01T00:00:00Z"),
					auditDateChecker.TestNextAuditDateUnchanged("test"),
				),
			},
			{
				Config:   s.getFixture("access_list_2_expiring.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestAccessListDefaults() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	checkAccessListDestroyed := func(state *terraform.State) error {
		_, err := s.client.AccessListClient().GetAccessList(context.TODO(), "defaults")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_access_list.defaults"
	auditDateChecker := nextAuditDateComparer{client: s.client}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkAccessListDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_list_defaults.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "header.kind", "access_list"),
					resource.TestCheckResourceAttr(name, "header.version", "v1"),
					resource.TestCheckResourceAttr(name, "header.metadata.name", "defaults"),
					resource.TestCheckResourceAttr(name, "spec.audit.notifications.start", "336h0m0s"),
					resource.TestCheckResourceAttr(name, "spec.audit.recurrence.day_of_month", "1"),
					resource.TestCheckResourceAttr(name, "spec.audit.recurrence.frequency", "6"),
					resource.TestCheckResourceAttr(name, "spec.owners.0.name", "gru"),
					resource.TestCheckResourceAttr(name, "spec.title", "Hello"),
					auditDateChecker.CaptureNextAuditDate("defaults"),
				),
			},
			{
				Config:   s.getFixture("access_list_defaults.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestAccessListMemberDefaults() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	checkAccessListMemberDestroyed := func(state *terraform.State) error {
		_, err := s.client.AccessListClient().GetAccessListMember(context.TODO(), "member_defaults", "member_defaults")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_access_list_member.member_defaults"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkAccessListMemberDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_list_member_defaults.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "header.kind", "access_list_member"),
					resource.TestCheckResourceAttr(name, "header.version", "v1"),
					resource.TestCheckResourceAttr(name, "header.metadata.name", "member_defaults"),

					resource.TestCheckResourceAttr(name, "spec.access_list", "member_defaults"),
					resource.TestCheckResourceAttr(name, "spec.expires", "2026-07-11T20:00:00Z"),
					resource.TestCheckResourceAttr(name, "spec.membership_kind", "1"),
					resource.TestCheckResourceAttr(name, "spec.reason", "example reason"),

					resource.TestCheckNoResourceAttr(name, "spec.added_by"),
					resource.TestCheckNoResourceAttr(name, "spec.joined"),
					resource.TestCheckNoResourceAttr(name, "spec.name"),
				),
			},
			{
				Config:   s.getFixture("access_list_member_defaults.tf"),
				PlanOnly: true,
			},
		},
	})
}
