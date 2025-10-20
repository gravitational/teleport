package feature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetCloudEntitlements(t *testing.T) {
	expected := map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.AccessGraphDemoMode:        {Enabled: false},
		entitlements.AccessLists:                {Enabled: true},
		entitlements.AccessMonitoring:           {Enabled: true, Limit: 1},
		entitlements.AccessRequests:             {Enabled: false},
		entitlements.App:                        {Enabled: false},
		entitlements.CloudAuditLogRetention:     {Enabled: false},
		entitlements.DB:                         {Enabled: false},
		entitlements.Desktop:                    {Enabled: false},
		entitlements.DeviceTrust:                {Enabled: true, Limit: 3},
		entitlements.ExternalAuditStorage:       {Enabled: false},
		entitlements.FeatureHiding:              {Enabled: false},
		entitlements.HSM:                        {Enabled: false},
		entitlements.Identity:                   {Enabled: false},
		entitlements.JoinActiveSessions:         {Enabled: false},
		entitlements.K8s:                        {Enabled: false},
		entitlements.MobileDeviceManagement:     {Enabled: false},
		entitlements.OIDC:                       {Enabled: false},
		entitlements.OktaSCIM:                   {Enabled: false},
		entitlements.OktaUserSync:               {Enabled: false},
		entitlements.Policy:                     {Enabled: false},
		entitlements.SAML:                       {Enabled: false},
		entitlements.SessionLocks:               {Enabled: false},
		entitlements.UpsellAlert:                {Enabled: false},
		entitlements.UsageReporting:             {Enabled: false},
		entitlements.LicenseAutoUpdate:          {Enabled: false},
		entitlements.UnrestrictedManagedUpdates: {Enabled: false},
		entitlements.ClientIPRestrictions:       {Enabled: false},
	}

	e := map[string]*cloudapi.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 1},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 3},
	}

	actual := GetCloudEntitlements(e)
	require.Equal(t, expected, actual)
}

func TestGetLicenseEntitlements(t *testing.T) {
	expected := map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.AccessGraphDemoMode:        {Enabled: false},
		entitlements.AccessLists:                {Enabled: true},
		entitlements.AccessMonitoring:           {Enabled: true, Limit: 11},
		entitlements.AccessRequests:             {Enabled: false},
		entitlements.App:                        {Enabled: false},
		entitlements.CloudAuditLogRetention:     {Enabled: false},
		entitlements.DB:                         {Enabled: false},
		entitlements.Desktop:                    {Enabled: false},
		entitlements.DeviceTrust:                {Enabled: true, Limit: 33},
		entitlements.ExternalAuditStorage:       {Enabled: false},
		entitlements.FeatureHiding:              {Enabled: false},
		entitlements.HSM:                        {Enabled: false},
		entitlements.Identity:                   {Enabled: false},
		entitlements.JoinActiveSessions:         {Enabled: false},
		entitlements.K8s:                        {Enabled: false},
		entitlements.MobileDeviceManagement:     {Enabled: false},
		entitlements.OIDC:                       {Enabled: false},
		entitlements.OktaSCIM:                   {Enabled: false},
		entitlements.OktaUserSync:               {Enabled: false},
		entitlements.Policy:                     {Enabled: false},
		entitlements.SAML:                       {Enabled: false},
		entitlements.SessionLocks:               {Enabled: false},
		entitlements.UpsellAlert:                {Enabled: false},
		entitlements.UsageReporting:             {Enabled: false},
		entitlements.LicenseAutoUpdate:          {Enabled: false},
		entitlements.UnrestrictedManagedUpdates: {Enabled: false},
		entitlements.ClientIPRestrictions:       {Enabled: false},
	}

	e := map[string]types.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 11},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 33},
	}

	actual := GetLicenseEntitlements(e)
	require.Equal(t, expected, actual)
}
