// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
)

func Test_supportEntitlementsCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		features *proto.Features
		expected map[string]*proto.EntitlementInfo
	}{
		{
			name: "entitlements present; keeps entitlement values",
			features: &proto.Features{
				DeviceTrust:                nil,
				AccessRequests:             nil,
				AccessList:                 nil,
				AccessMonitoring:           nil,
				Policy:                     nil,
				CustomTheme:                "",
				ProductType:                0,
				SupportType:                0,
				Kubernetes:                 false,
				App:                        false,
				DB:                         false,
				OIDC:                       false,
				SAML:                       false,
				AccessControls:             false,
				AdvancedAccessWorkflows:    false,
				Cloud:                      false,
				HSM:                        false,
				Desktop:                    false,
				RecoveryCodes:              false,
				Plugins:                    false,
				AutomaticUpgrades:          false,
				IsUsageBased:               false,
				Assist:                     false,
				FeatureHiding:              false,
				IdentityGovernance:         false,
				AccessGraph:                false,
				Questionnaire:              false,
				IsStripeManaged:            false,
				ExternalAuditStorage:       false,
				JoinActiveSessions:         false,
				MobileDeviceManagement:     false,
				AccessMonitoringConfigured: false,
				Entitlements: map[string]*proto.EntitlementInfo{
					string(entitlements.AccessLists):            {Enabled: true, Limit: 111},
					string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 2113},
					string(entitlements.AccessRequests):         {Enabled: true, Limit: 39},
					string(entitlements.App):                    {Enabled: false},
					string(entitlements.CloudAuditLogRetention): {Enabled: true},
					string(entitlements.DB):                     {Enabled: true},
					string(entitlements.Desktop):                {Enabled: true},
					string(entitlements.DeviceTrust):            {Enabled: true, Limit: 103},
					string(entitlements.ExternalAuditStorage):   {Enabled: true},
					string(entitlements.FeatureHiding):          {Enabled: true},
					string(entitlements.HSM):                    {Enabled: true},
					string(entitlements.Identity):               {Enabled: true},
					string(entitlements.JoinActiveSessions):     {Enabled: true},
					string(entitlements.K8s):                    {Enabled: true},
					string(entitlements.MobileDeviceManagement): {Enabled: true},
					string(entitlements.OIDC):                   {Enabled: true},
					string(entitlements.OktaSCIM):               {Enabled: true},
					string(entitlements.OktaUserSync):           {Enabled: true},
					string(entitlements.Policy):                 {Enabled: true},
					string(entitlements.SAML):                   {Enabled: true},
					string(entitlements.SessionLocks):           {Enabled: true},
					string(entitlements.UpsellAlert):            {Enabled: true},
					string(entitlements.UsageReporting):         {Enabled: true},
				},
			},
			expected: map[string]*proto.EntitlementInfo{
				string(entitlements.AccessLists):            {Enabled: true, Limit: 111},
				string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 2113},
				string(entitlements.AccessRequests):         {Enabled: true, Limit: 39},
				string(entitlements.App):                    {Enabled: false},
				string(entitlements.CloudAuditLogRetention): {Enabled: true},
				string(entitlements.DB):                     {Enabled: true},
				string(entitlements.Desktop):                {Enabled: true},
				string(entitlements.DeviceTrust):            {Enabled: true, Limit: 103},
				string(entitlements.ExternalAuditStorage):   {Enabled: true},
				string(entitlements.FeatureHiding):          {Enabled: true},
				string(entitlements.HSM):                    {Enabled: true},
				string(entitlements.Identity):               {Enabled: true},
				string(entitlements.JoinActiveSessions):     {Enabled: true},
				string(entitlements.K8s):                    {Enabled: true},
				string(entitlements.MobileDeviceManagement): {Enabled: true},
				string(entitlements.OIDC):                   {Enabled: true},
				string(entitlements.OktaSCIM):               {Enabled: true},
				string(entitlements.OktaUserSync):           {Enabled: true},
				string(entitlements.Policy):                 {Enabled: true},
				string(entitlements.SAML):                   {Enabled: true},
				string(entitlements.SessionLocks):           {Enabled: true},
				string(entitlements.UpsellAlert):            {Enabled: true},
				string(entitlements.UsageReporting):         {Enabled: true},
			},
		},
		{
			name: "entitlements not present; identity on - sets legacy fields & drops limits",
			features: &proto.Features{
				DeviceTrust: &proto.DeviceTrustFeature{
					Enabled:           true,
					DevicesUsageLimit: 33,
				},
				AccessRequests: &proto.AccessRequestsFeature{
					MonthlyRequestLimit: 22,
				},
				AccessList: &proto.AccessListFeature{
					CreateLimit: 44,
				},
				AccessMonitoring: &proto.AccessMonitoringFeature{
					Enabled:             true,
					MaxReportRangeLimit: 55,
				},
				Policy: &proto.PolicyFeature{
					Enabled: true,
				},
				CustomTheme:                "",
				ProductType:                0,
				SupportType:                0,
				Kubernetes:                 true,
				App:                        true,
				DB:                         true,
				OIDC:                       true,
				SAML:                       true,
				AccessControls:             true,
				AdvancedAccessWorkflows:    true,
				Cloud:                      true,
				HSM:                        true,
				Desktop:                    true,
				RecoveryCodes:              true,
				Plugins:                    true,
				AutomaticUpgrades:          true,
				IsUsageBased:               true,
				Assist:                     true,
				FeatureHiding:              true,
				IdentityGovernance:         true,
				AccessGraph:                true,
				Questionnaire:              true,
				IsStripeManaged:            true,
				ExternalAuditStorage:       true,
				JoinActiveSessions:         true,
				MobileDeviceManagement:     true,
				AccessMonitoringConfigured: true,
			},
			expected: map[string]*proto.EntitlementInfo{
				string(entitlements.AccessLists):            {Enabled: true},
				string(entitlements.AccessMonitoring):       {Enabled: true},
				string(entitlements.AccessRequests):         {Enabled: true},
				string(entitlements.App):                    {Enabled: true},
				string(entitlements.DB):                     {Enabled: true},
				string(entitlements.Desktop):                {Enabled: true},
				string(entitlements.DeviceTrust):            {Enabled: true},
				string(entitlements.ExternalAuditStorage):   {Enabled: true},
				string(entitlements.FeatureHiding):          {Enabled: true},
				string(entitlements.HSM):                    {Enabled: true},
				string(entitlements.Identity):               {Enabled: true},
				string(entitlements.JoinActiveSessions):     {Enabled: true},
				string(entitlements.K8s):                    {Enabled: true},
				string(entitlements.MobileDeviceManagement): {Enabled: true},
				string(entitlements.OIDC):                   {Enabled: true},
				string(entitlements.OktaSCIM):               {Enabled: true},
				string(entitlements.OktaUserSync):           {Enabled: true},
				string(entitlements.Policy):                 {Enabled: true},
				string(entitlements.SAML):                   {Enabled: true},
				string(entitlements.SessionLocks):           {Enabled: true},
				// defaults, no legacy equivalent
				string(entitlements.UsageReporting):         {Enabled: false},
				string(entitlements.UpsellAlert):            {Enabled: false},
				string(entitlements.CloudAuditLogRetention): {Enabled: false},
			},
		},
		{
			name: "entitlements not present; identity off - sets legacy fields",
			features: &proto.Features{
				DeviceTrust: &proto.DeviceTrustFeature{
					Enabled:           true,
					DevicesUsageLimit: 33,
				},
				AccessRequests: &proto.AccessRequestsFeature{
					MonthlyRequestLimit: 22,
				},
				AccessList: &proto.AccessListFeature{
					CreateLimit: 44,
				},
				AccessMonitoring: &proto.AccessMonitoringFeature{
					Enabled:             true,
					MaxReportRangeLimit: 55,
				},
				Policy: &proto.PolicyFeature{
					Enabled: true,
				},
				CustomTheme:                "",
				ProductType:                0,
				SupportType:                0,
				Kubernetes:                 true,
				App:                        true,
				DB:                         true,
				OIDC:                       true,
				SAML:                       true,
				AccessControls:             true,
				AdvancedAccessWorkflows:    true,
				Cloud:                      true,
				HSM:                        true,
				Desktop:                    true,
				RecoveryCodes:              true,
				Plugins:                    true,
				AutomaticUpgrades:          true,
				IsUsageBased:               true,
				Assist:                     true,
				FeatureHiding:              true,
				IdentityGovernance:         false,
				AccessGraph:                true,
				Questionnaire:              true,
				IsStripeManaged:            true,
				ExternalAuditStorage:       true,
				JoinActiveSessions:         true,
				MobileDeviceManagement:     true,
				AccessMonitoringConfigured: true,
			},
			expected: map[string]*proto.EntitlementInfo{
				string(entitlements.AccessLists):            {Enabled: true, Limit: 44},
				string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 55},
				string(entitlements.AccessRequests):         {Enabled: true, Limit: 22},
				string(entitlements.DeviceTrust):            {Enabled: true, Limit: 33},
				string(entitlements.App):                    {Enabled: true},
				string(entitlements.DB):                     {Enabled: true},
				string(entitlements.Desktop):                {Enabled: true},
				string(entitlements.ExternalAuditStorage):   {Enabled: true},
				string(entitlements.FeatureHiding):          {Enabled: true},
				string(entitlements.HSM):                    {Enabled: true},
				string(entitlements.JoinActiveSessions):     {Enabled: true},
				string(entitlements.K8s):                    {Enabled: true},
				string(entitlements.MobileDeviceManagement): {Enabled: true},
				string(entitlements.OIDC):                   {Enabled: true},
				string(entitlements.Policy):                 {Enabled: true},
				string(entitlements.SAML):                   {Enabled: true},

				// defaults, no legacy equivalent
				string(entitlements.UsageReporting):         {Enabled: false},
				string(entitlements.UpsellAlert):            {Enabled: false},
				string(entitlements.CloudAuditLogRetention): {Enabled: false},
				// Identity off, fields false
				string(entitlements.Identity):     {Enabled: false},
				string(entitlements.SessionLocks): {Enabled: false},
				string(entitlements.OktaSCIM):     {Enabled: false},
				string(entitlements.OktaUserSync): {Enabled: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := apiutils.CloneProtoMsg(tt.features)

			supportEntitlementsCompatibility(cloned)
			require.Equal(t, tt.expected, cloned.Entitlements)
		})
	}
}
