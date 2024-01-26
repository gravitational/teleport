/*
Copyright 2018-2019 Gravitational, Inc.

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

package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// License defines teleport License Information
type License interface {
	Resource

	// GetReportsUsage returns true if the Teleport cluster should report usage
	// to the Houston control plane.
	GetReportsUsage() Bool
	// SetReportsUsage sets the Houston usage reporting flag.
	SetReportsUsage(Bool)
	// GetSalesCenterReporting returns true if the Teleport cluster should
	// report usage to Sales Center.
	GetSalesCenterReporting() Bool
	// SetSalesCenterReporting sets the Sales Center usage reporting flag.
	SetSalesCenterReporting(Bool)

	// GetCloud returns true if teleport cluster is hosted by Gravitational
	GetCloud() Bool
	// SetCloud sets cloud flag
	SetCloud(Bool)

	// GetAWSProductID returns product id that limits usage to AWS instance
	// with a similar product ID
	GetAWSProductID() string
	// SetAWSProductID sets AWS product ID
	SetAWSProductID(string)

	// GetAWSAccountID limits usage to AWS instance within account ID
	GetAWSAccountID() string
	// SetAWSAccountID sets AWS account ID that will be limiting
	// usage to AWS instance
	SetAWSAccountID(accountID string)

	// GetSupportsKubernetes returns kubernetes support flag
	GetSupportsKubernetes() Bool
	// SetSupportsKubernetes sets kubernetes support flag
	SetSupportsKubernetes(Bool)

	// GetSupportsApplicationAccess returns application access support flag
	GetSupportsApplicationAccess() Bool
	// SetSupportsApplicationAccess sets application access support flag
	SetSupportsApplicationAccess(Bool)

	// GetSupportsDatabaseAccess returns database access support flag
	GetSupportsDatabaseAccess() Bool
	// SetSupportsDatabaseAccess sets database access support flag
	SetSupportsDatabaseAccess(Bool)

	// GetSupportsDesktopAccess returns desktop access support flag
	GetSupportsDesktopAccess() Bool
	// SetSupportsDesktopAccess sets desktop access support flag
	SetSupportsDesktopAccess(Bool)

	// GetSupportsModeratedSessions returns moderated sessions support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	GetSupportsModeratedSessions() Bool
	// SetSupportsModeratedSessions sets moderated sessions support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	SetSupportsModeratedSessions(Bool)

	// GetSupportsMachineID returns MachineID support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	GetSupportsMachineID() Bool
	// SetSupportsMachineID sets MachineID support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	SetSupportsMachineID(Bool)

	// GetSupportsResourceAccessRequests returns resource access requests support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	GetSupportsResourceAccessRequests() Bool
	// SetSupportsResourceAccessRequests sets resource access requests support flag
	// Note: this flag is unused in Teleport v11+ but it's still used to
	// generate licenses that support older versions of Teleport
	SetSupportsResourceAccessRequests(Bool)

	// GetSupportsFeatureHiding returns feature hiding support flag.
	GetSupportsFeatureHiding() Bool
	// GetSupportsFeatureHiding sets feature hiding support flag.
	SetSupportsFeatureHiding(Bool)

	// GetTrial returns the trial flag.
	//  Note: This is not applicable to Cloud licenses
	GetTrial() Bool
	// SetTrial sets the trial flag.
	//  Note: This is not applicable to Cloud licenses
	SetTrial(Bool)

	// SetLabels sets metadata labels
	SetLabels(labels map[string]string)

	// GetAccountID returns Account ID.
	//  Note: This is not applicable to all Cloud licenses
	GetAccountID() string

	// GetFeatureSource returns where the features should be loaded from.
	//
	// Deprecated.
	// FeatureSource was used to differentiate between
	// cloud+team vs cloud+enterprise. cloud+enterprise read from license
	// and cloud+team read from salescenter. With the new EUB product,
	// all cloud+ will read from salescenter.
	GetFeatureSource() FeatureSource

	// GetCustomTheme returns the name of the WebUI custom theme
	GetCustomTheme() string

	// SetCustomTheme sets the name of the WebUI custom theme
	SetCustomTheme(themeName string)

	// GetSupportsIdentityGovernanceSecurity returns IGS features support flag.
	// IGS includes: access list, access request, access monitoring and device trust.
	GetSupportsIdentityGovernanceSecurity() Bool
	// SetSupportsIdentityGovernanceSecurity sets IGS feature support flag.
	// IGS includes: access list, access request, access monitoring and device trust.
	SetSupportsIdentityGovernanceSecurity(Bool)
	// GetUsageBasedBilling returns if usage based billing is turned on or off
	GetUsageBasedBilling() Bool
	// SetUsageBasedBilling sets flag for usage based billing
	SetUsageBasedBilling(Bool)

	// GetAnonymizationKey returns a key that should be used to
	// anonymize usage data if it's set.
	GetAnonymizationKey() string
	// SetAnonymizationKey sets the anonymization key.
	SetAnonymizationKey(string)

	// GetSupportsPolicy returns Teleport Policy support flag.
	GetSupportsPolicy() Bool
	//SGetSupportsPolicy sets Teleport Policy support flag.
	SetSupportsPolicy(Bool)
}

// FeatureSource defines where the list of features enabled
// by the license is.
type FeatureSource string

const (
	FeatureSourceLicense FeatureSource = "license"
	FeatureSourceCloud   FeatureSource = "cloud"
)

// NewLicense is a convenience method to create LicenseV3.
func NewLicense(name string, spec LicenseSpecV3) (License, error) {
	l := &LicenseV3{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := l.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return l, nil
}

// LicenseV3 represents License resource version V3. When changing this, keep in
// mind that other consumers of teleport/api (Houston, Sales Center) might still
// need to generate or parse licenses for older versions of Teleport.
type LicenseV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec LicenseSpecV3 `json:"spec"`
}

// GetVersion returns resource version
func (c *LicenseV3) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource sub kind
func (c *LicenseV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *LicenseV3) SetSubKind(s string) {
	c.SubKind = s
}

// GetKind returns resource kind
func (c *LicenseV3) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *LicenseV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *LicenseV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetRevision returns the revision
func (c *LicenseV3) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *LicenseV3) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// GetName returns the name of the resource
func (c *LicenseV3) GetName() string {
	return c.Metadata.Name
}

// SetLabels sets metadata labels
func (c *LicenseV3) SetLabels(labels map[string]string) {
	c.Metadata.Labels = labels
}

// GetLabels returns metadata labels
func (c *LicenseV3) GetLabels() map[string]string {
	return c.Metadata.Labels
}

// SetName sets the name of the resource
func (c *LicenseV3) SetName(name string) {
	c.Metadata.Name = name
}

// Expiry returns object expiry setting
func (c *LicenseV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (c *LicenseV3) SetExpiry(t time.Time) {
	c.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata
func (c *LicenseV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetReportsUsage returns true if the Teleport cluster should report usage to
// the Houston control plane.
func (c *LicenseV3) GetReportsUsage() Bool {
	return c.Spec.ReportsUsage
}

// GetSalesCenterReporting returns true if the Teleport cluster should report
// usage to Sales Center.
func (c *LicenseV3) GetSalesCenterReporting() Bool {
	return c.Spec.SalesCenterReporting
}

// GetCloud returns true if teleport cluster is hosted by Gravitational
func (c *LicenseV3) GetCloud() Bool {
	return c.Spec.Cloud
}

// SetCloud sets cloud flag
func (c *LicenseV3) SetCloud(cloud Bool) {
	c.Spec.Cloud = cloud
}

// SetReportsUsage sets the Houston usage reporting flag.
func (c *LicenseV3) SetReportsUsage(reports Bool) {
	c.Spec.ReportsUsage = reports
}

// SetSalesCenterReporting sets the Sales Center usage reporting flag.
func (c *LicenseV3) SetSalesCenterReporting(reports Bool) {
	c.Spec.SalesCenterReporting = reports
}

// setStaticFields sets static resource header and metadata fields.
func (c *LicenseV3) setStaticFields() {
	c.Kind = KindLicense
	c.Version = V3
}

// CheckAndSetDefaults verifies the constraints for License.
func (c *LicenseV3) CheckAndSetDefaults() error {
	c.setStaticFields()
	if c.Spec.FeatureSource == "" {
		c.Spec.FeatureSource = FeatureSourceLicense
	}
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAWSProductID returns product ID that limits usage to AWS instance
// with a similar product ID
func (c *LicenseV3) GetAWSProductID() string {
	return c.Spec.AWSProductID
}

// SetAWSProductID sets AWS product ID
func (c *LicenseV3) SetAWSProductID(pid string) {
	c.Spec.AWSProductID = pid
}

// GetAccountID sets AWS product ID
func (c *LicenseV3) GetAccountID() string {
	return c.Spec.AccountID
}

// GetAWSAccountID limits usage to AWS instance within account ID
func (c *LicenseV3) GetAWSAccountID() string {
	return c.Spec.AWSAccountID
}

// SetAWSAccountID sets AWS account ID that will be limiting
// usage to AWS instance
func (c *LicenseV3) SetAWSAccountID(accountID string) {
	c.Spec.AWSAccountID = accountID
}

// GetSupportsKubernetes returns kubernetes support flag
func (c *LicenseV3) GetSupportsKubernetes() Bool {
	return c.Spec.SupportsKubernetes
}

// SetSupportsKubernetes sets kubernetes support flag
func (c *LicenseV3) SetSupportsKubernetes(supportsK8s Bool) {
	c.Spec.SupportsKubernetes = supportsK8s
}

// GetSupportsApplicationAccess returns application access support flag
func (c *LicenseV3) GetSupportsApplicationAccess() Bool {
	// For backward compatibility return true if app access flag isn't set,
	// or it will stop working for all users who are already using it and
	// were issued licenses without this flag.
	if c.Spec.SupportsApplicationAccess == nil {
		return Bool(true)
	}
	return *c.Spec.SupportsApplicationAccess
}

// SetSupportsApplicationAccess sets application access support flag
func (c *LicenseV3) SetSupportsApplicationAccess(value Bool) {
	c.Spec.SupportsApplicationAccess = &value
}

// GetSupportsDatabaseAccess returns database access support flag
func (c *LicenseV3) GetSupportsDatabaseAccess() Bool {
	return c.Spec.SupportsDatabaseAccess
}

// SetSupportsDatabaseAccess sets database access support flag
func (c *LicenseV3) SetSupportsDatabaseAccess(value Bool) {
	c.Spec.SupportsDatabaseAccess = value
}

// GetSupportsDesktopAccess returns desktop access support flag
func (c *LicenseV3) GetSupportsDesktopAccess() Bool {
	return c.Spec.SupportsDesktopAccess
}

// SetSupportsDesktopAccess sets desktop access support flag
func (c *LicenseV3) SetSupportsDesktopAccess(value Bool) {
	c.Spec.SupportsDesktopAccess = value
}

// GetSupportsModeratedSessions returns moderated sessions support flag
func (c *LicenseV3) GetSupportsModeratedSessions() Bool {
	return c.Spec.SupportsModeratedSessions
}

// SetSupportsModeratedSessions sets moderated sessions support flag
func (c *LicenseV3) SetSupportsModeratedSessions(value Bool) {
	c.Spec.SupportsModeratedSessions = value
}

// GetSupportsMachineID returns MachineID support flag
func (c *LicenseV3) GetSupportsMachineID() Bool {
	return c.Spec.SupportsMachineID
}

// SetSupportsMachineID sets MachineID support flag
func (c *LicenseV3) SetSupportsMachineID(value Bool) {
	c.Spec.SupportsMachineID = value
}

// GetSupportsResourceAccessRequests returns resource access requests support flag
func (c *LicenseV3) GetSupportsResourceAccessRequests() Bool {
	return c.Spec.SupportsResourceAccessRequests
}

// SetSupportsResourceAccessRequests sets resource access requests support flag
func (c *LicenseV3) SetSupportsResourceAccessRequests(value Bool) {
	c.Spec.SupportsResourceAccessRequests = value
}

// GetSupportsFeatureHiding returns feature hiding requests support flag
func (c *LicenseV3) GetSupportsFeatureHiding() Bool {
	return c.Spec.SupportsFeatureHiding
}

// SetSupportsFeatureHiding sets feature hiding requests support flag
func (c *LicenseV3) SetSupportsFeatureHiding(value Bool) {
	c.Spec.SupportsFeatureHiding = value
}

// GetCustomTheme returns the name of the WebUI custom theme
func (c *LicenseV3) GetCustomTheme() string {
	return c.Spec.CustomTheme
}

// SetCustomTheme sets the name of the WebUI custom theme
func (c *LicenseV3) SetCustomTheme(themeName string) {
	c.Spec.CustomTheme = themeName
}

// GetSupportsIdentityGovernanceSecurity returns IGS feature support flag.
// IGS includes: access list, access request, access monitoring and device trust.
func (c *LicenseV3) GetSupportsIdentityGovernanceSecurity() Bool {
	return c.Spec.SupportsIdentityGovernanceSecurity
}

// SetSupportsIdentityGovernanceSecurity sets IGS feature support flag.
// IGS includes: access list, access request, access monitoring and device trust.
func (c *LicenseV3) SetSupportsIdentityGovernanceSecurity(b Bool) {
	c.Spec.SupportsIdentityGovernanceSecurity = b
}

// GetUsageBasedBilling returns if usage based billing is turned on or off
func (c *LicenseV3) GetUsageBasedBilling() Bool {
	return c.Spec.UsageBasedBilling
}

// SetUsageBasedBilling sets flag for usage based billing.
func (c *LicenseV3) SetUsageBasedBilling(b Bool) {
	c.Spec.UsageBasedBilling = b
}

// GetTrial returns the trial flag
func (c *LicenseV3) GetTrial() Bool {
	return c.Spec.Trial
}

// SetTrial sets the trial flag
func (c *LicenseV3) SetTrial(value Bool) {
	c.Spec.Trial = value
}

// GetAnonymizationKey returns a key that should be used to
// anonymize usage data if it's set.
func (c *LicenseV3) GetAnonymizationKey() string {
	return c.Spec.AnonymizationKey
}

// SetAnonymizationKey sets the anonymization key.
func (c *LicenseV3) SetAnonymizationKey(anonKey string) {
	c.Spec.AnonymizationKey = anonKey
}

// GetSupportsPolicy returns Teleport Policy support flag
func (c *LicenseV3) GetSupportsPolicy() Bool {
	return c.Spec.SupportsPolicy
}

// SetSupportsPolicy sets Teleport Policy support flag
func (c *LicenseV3) SetSupportsPolicy(value Bool) {
	c.Spec.SupportsPolicy = value
}

// String represents a human readable version of license enabled features
func (c *LicenseV3) String() string {
	var features []string
	if !c.Expiry().IsZero() {
		features = append(features, fmt.Sprintf("expires at %v", c.Expiry()))
	}
	if c.GetTrial() {
		features = append(features, "is trial")
	}
	if c.GetReportsUsage() {
		features = append(features, "reports usage")
	}
	if c.GetSupportsKubernetes() {
		features = append(features, "supports kubernetes")
	}
	if c.GetSupportsApplicationAccess() {
		features = append(features, "supports application access")
	}
	if c.GetSupportsDatabaseAccess() {
		features = append(features, "supports database access")
	}
	if c.GetSupportsDesktopAccess() {
		features = append(features, "supports desktop access")
	}
	if c.GetSupportsFeatureHiding() {
		features = append(features, "supports feature hiding")
	}
	if c.GetCloud() {
		features = append(features, "is hosted by Gravitational")
	}
	if c.GetAWSProductID() != "" {
		features = append(features, fmt.Sprintf("is limited to AWS product ID %q", c.Spec.AWSProductID))
	}
	if c.GetAWSAccountID() != "" {
		features = append(features, fmt.Sprintf("is limited to AWS account ID %q", c.Spec.AWSAccountID))
	}
	if len(features) == 0 {
		return ""
	}
	return strings.Join(features, ",")
}

// GetFeatureSource returns the source Teleport should use to read the features
func (c *LicenseV3) GetFeatureSource() FeatureSource {
	// defaults to License for backward compatibility
	if c.Spec.FeatureSource == "" {
		return FeatureSourceLicense
	}

	return c.Spec.FeatureSource
}

// LicenseSpecV3 is the actual data we care about for LicenseV3. When changing
// this, keep in mind that other consumers of teleport/api (Houston, Sales
// Center) might still need to generate or parse licenses for older versions of
// Teleport.
type LicenseSpecV3 struct {
	// AccountID is a customer account ID
	AccountID string `json:"account_id,omitempty"`
	// AWSProductID limits usage to AWS instance with a product ID
	AWSProductID string `json:"aws_pid,omitempty"`
	// AWSAccountID limits usage to AWS instance within account ID
	AWSAccountID string `json:"aws_account,omitempty"`
	// SupportsKubernetes turns kubernetes support on or off
	SupportsKubernetes Bool `json:"k8s"`
	// SupportsApplicationAccess turns application access on or off
	// Note it's a pointer for backward compatibility
	SupportsApplicationAccess *Bool `json:"app,omitempty"`
	// SupportsDatabaseAccess turns database access on or off
	SupportsDatabaseAccess Bool `json:"db,omitempty"`
	// SupportsDesktopAccess turns desktop access on or off
	SupportsDesktopAccess Bool `json:"desktop,omitempty"`
	// ReportsUsage turns Houston usage reporting on or off
	ReportsUsage Bool `json:"usage,omitempty"`
	// SalesCenterReporting turns Sales Center usage reporting on or off
	SalesCenterReporting Bool `json:"reporting,omitempty"`
	// Cloud is turned on when teleport is hosted by Gravitational
	Cloud Bool `json:"cloud,omitempty"`
	// SupportsModeratedSessions turns on moderated sessions
	SupportsModeratedSessions Bool `json:"moderated_sessions,omitempty"`
	// SupportsMachineID turns MachineID support on or off
	SupportsMachineID Bool `json:"machine_id,omitempty"`
	// SupportsResourceAccessRequests turns resource access request support on or off
	SupportsResourceAccessRequests Bool `json:"resource_access_requests,omitempty"`
	// SupportsFeatureHiding turns feature hiding support on or off
	SupportsFeatureHiding Bool `json:"feature_hiding,omitempty"`
	// Trial is true for trial licenses
	Trial Bool `json:"trial,omitempty"`
	// FeatureSource is the source of the set of enabled feature
	//
	// Deprecated.
	// FeatureSource was used to differentiate between
	// cloud+team vs cloud+enterprise. cloud+enterprise read from license
	// and cloud+team read from salescenter. With the new EUB product,
	// all cloud+ will read from salescenter.
	FeatureSource FeatureSource `json:"feature_source"`
	// CustomTheme is the name of the WebUI custom theme
	CustomTheme string `json:"custom_theme,omitempty"`
	// SupportsIdentityGovernanceSecurity turns IGS features on or off.
	SupportsIdentityGovernanceSecurity Bool `json:"identity_governance_security,omitempty"`
	// UsageBasedBilling determines if the user subscription is usage-based (pay-as-you-go).
	UsageBasedBilling Bool `json:"usage_based_billing,omitempty"`
	// AnonymizationKey is a key that is used to anonymize usage data when it is set.
	// It should only be set when UsageBasedBilling is true.
	AnonymizationKey string `json:"anonymization_key,omitempty"`
	// SupportsPolicy turns Teleport Policy features on or off.
	SupportsPolicy Bool `json:"policy,omitempty"`
}
