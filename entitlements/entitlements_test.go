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

package entitlements

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
)

func TestBackfillFeatures(t *testing.T) {
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
					string(AccessLists):                {Enabled: true, Limit: 111},
					string(AccessMonitoring):           {Enabled: true, Limit: 2113},
					string(AccessRequests):             {Enabled: true, Limit: 39},
					string(App):                        {Enabled: false},
					string(CloudAuditLogRetention):     {Enabled: true},
					string(DB):                         {Enabled: true},
					string(Desktop):                    {Enabled: true},
					string(DeviceTrust):                {Enabled: true, Limit: 103},
					string(ExternalAuditStorage):       {Enabled: true},
					string(FeatureHiding):              {Enabled: true},
					string(HSM):                        {Enabled: true},
					string(Identity):                   {Enabled: true},
					string(JoinActiveSessions):         {Enabled: true},
					string(K8s):                        {Enabled: true},
					string(MobileDeviceManagement):     {Enabled: true},
					string(OIDC):                       {Enabled: true},
					string(OktaSCIM):                   {Enabled: true},
					string(OktaUserSync):               {Enabled: true},
					string(Policy):                     {Enabled: true},
					string(SAML):                       {Enabled: true},
					string(SessionLocks):               {Enabled: true},
					string(UpsellAlert):                {Enabled: true},
					string(UsageReporting):             {Enabled: true},
					string(LicenseAutoUpdate):          {Enabled: true},
					string(AccessGraphDemoMode):        {Enabled: true},
					string(UnrestrictedManagedUpdates): {Enabled: true},
				},
			},
			expected: map[string]*proto.EntitlementInfo{
				string(AccessLists):                {Enabled: true, Limit: 111},
				string(AccessMonitoring):           {Enabled: true, Limit: 2113},
				string(AccessRequests):             {Enabled: true, Limit: 39},
				string(App):                        {Enabled: false},
				string(CloudAuditLogRetention):     {Enabled: true},
				string(DB):                         {Enabled: true},
				string(Desktop):                    {Enabled: true},
				string(DeviceTrust):                {Enabled: true, Limit: 103},
				string(ExternalAuditStorage):       {Enabled: true},
				string(FeatureHiding):              {Enabled: true},
				string(HSM):                        {Enabled: true},
				string(Identity):                   {Enabled: true},
				string(JoinActiveSessions):         {Enabled: true},
				string(K8s):                        {Enabled: true},
				string(MobileDeviceManagement):     {Enabled: true},
				string(OIDC):                       {Enabled: true},
				string(OktaSCIM):                   {Enabled: true},
				string(OktaUserSync):               {Enabled: true},
				string(Policy):                     {Enabled: true},
				string(SAML):                       {Enabled: true},
				string(SessionLocks):               {Enabled: true},
				string(UpsellAlert):                {Enabled: true},
				string(UsageReporting):             {Enabled: true},
				string(LicenseAutoUpdate):          {Enabled: true},
				string(AccessGraphDemoMode):        {Enabled: true},
				string(UnrestrictedManagedUpdates): {Enabled: true},
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
				string(AccessLists):            {Enabled: true},
				string(AccessMonitoring):       {Enabled: true},
				string(AccessRequests):         {Enabled: true},
				string(App):                    {Enabled: true},
				string(DB):                     {Enabled: true},
				string(Desktop):                {Enabled: true},
				string(DeviceTrust):            {Enabled: true},
				string(ExternalAuditStorage):   {Enabled: true},
				string(FeatureHiding):          {Enabled: true},
				string(HSM):                    {Enabled: true},
				string(Identity):               {Enabled: true},
				string(JoinActiveSessions):     {Enabled: true},
				string(K8s):                    {Enabled: true},
				string(MobileDeviceManagement): {Enabled: true},
				string(OIDC):                   {Enabled: true},
				string(OktaSCIM):               {Enabled: true},
				string(OktaUserSync):           {Enabled: true},
				string(Policy):                 {Enabled: true},
				string(SAML):                   {Enabled: true},
				string(SessionLocks):           {Enabled: true},
				// defaults, no legacy equivalent
				string(UsageReporting):             {Enabled: false},
				string(UpsellAlert):                {Enabled: false},
				string(CloudAuditLogRetention):     {Enabled: false},
				string(LicenseAutoUpdate):          {Enabled: false},
				string(AccessGraphDemoMode):        {Enabled: false},
				string(UnrestrictedManagedUpdates): {Enabled: false},
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
				string(AccessLists):            {Enabled: true, Limit: 44},
				string(AccessMonitoring):       {Enabled: true, Limit: 55},
				string(AccessRequests):         {Enabled: true, Limit: 22},
				string(DeviceTrust):            {Enabled: true, Limit: 33},
				string(App):                    {Enabled: true},
				string(DB):                     {Enabled: true},
				string(Desktop):                {Enabled: true},
				string(ExternalAuditStorage):   {Enabled: true},
				string(FeatureHiding):          {Enabled: true},
				string(HSM):                    {Enabled: true},
				string(JoinActiveSessions):     {Enabled: true},
				string(K8s):                    {Enabled: true},
				string(MobileDeviceManagement): {Enabled: true},
				string(OIDC):                   {Enabled: true},
				string(Policy):                 {Enabled: true},
				string(SAML):                   {Enabled: true},

				// defaults, no legacy equivalent
				string(UsageReporting):             {Enabled: false},
				string(UpsellAlert):                {Enabled: false},
				string(CloudAuditLogRetention):     {Enabled: false},
				string(LicenseAutoUpdate):          {Enabled: false},
				string(AccessGraphDemoMode):        {Enabled: false},
				string(UnrestrictedManagedUpdates): {Enabled: false},
				// Identity off, fields false
				string(Identity):     {Enabled: false},
				string(SessionLocks): {Enabled: false},
				string(OktaSCIM):     {Enabled: false},
				string(OktaUserSync): {Enabled: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := apiutils.CloneProtoMsg(tt.features)

			BackfillFeatures(cloned)
			require.Equal(t, tt.expected, cloned.Entitlements)
		})
	}
}
