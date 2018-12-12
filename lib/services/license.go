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

package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// License defines teleport License Information
type License interface {
	Resource
	// GetReportsUsage returns true if teleport cluster reports usage
	// to control plane
	GetReportsUsage() Bool

	// SetReportsUsage sets usage report
	SetReportsUsage(Bool)

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

	// SetLabels sets metadata labels
	SetLabels(labels map[string]string)

	// GetAccountID returns Account ID
	GetAccountID() string

	// CheckAndSetDefaults sets and default values and then
	// verifies the constraints for License.
	CheckAndSetDefaults() error
}

// NewLicense is a convenience method to to create LicenseV3.
func NewLicense(name string, spec LicenseSpecV3) (License, error) {
	return &LicenseV3{
		Kind:    KindLicense,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, nil
}

// LicenseV3 represents License resource version V3
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

// SetTTL sets Expires header using current clock
func (c *LicenseV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *LicenseV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetReportsUsage returns true if teleport cluster reports usage
// to control plane
func (c *LicenseV3) GetReportsUsage() Bool {
	return c.Spec.ReportsUsage
}

// SetReportsUsage sets usage report
func (c *LicenseV3) SetReportsUsage(reports Bool) {
	c.Spec.ReportsUsage = reports
}

// CheckAndSetDefaults verifies the constraints for License.
func (c *LicenseV3) CheckAndSetDefaults() error {
	return c.Metadata.CheckAndSetDefaults()
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

// String represents a human readable version of license enabled features
func (c *LicenseV3) String() string {
	var features []string
	if !c.Expiry().IsZero() {
		features = append(features, fmt.Sprintf("expires at %v", c.Expiry()))
	}
	if c.Spec.ReportsUsage.Value() {
		features = append(features, "reports usage")
	}
	if c.Spec.SupportsKubernetes.Value() {
		features = append(features, "supports kubernetes")
	}
	if c.Spec.AWSProductID != "" {
		features = append(features, fmt.Sprintf("is limited to AWS product ID %q", c.Spec.AWSProductID))
	}
	if c.Spec.AWSAccountID != "" {
		features = append(features, fmt.Sprintf("is limited to AWS account ID %q", c.Spec.AWSAccountID))
	}
	if len(features) == 0 {
		return ""
	}
	return strings.Join(features, ",")
}

// LicenseSpecV3 is the actual data we care about for LicenseV3.
type LicenseSpecV3 struct {
	// AccountID is a customer account ID
	AccountID string `json:"account_id,omitempty"`
	// AWSProductID limits usage to AWS instance with a product ID
	AWSProductID string `json:"aws_pid,omitempty"`
	// AWSAccountID limits usage to AWS instance within account ID
	AWSAccountID string `json:"aws_account,omitempty"`
	// SupportsKubernetes turns kubernetes support on or off
	SupportsKubernetes Bool `json:"k8s"`
	// ReportsUsage is turned on when system reports usage
	ReportsUsage Bool `json:"usage,omitempty"`
}

// LicenseSpecV3Template is a template for V3 License JSON schema
const LicenseSpecV3Template = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
	"account_id": {
		"type": ["string"]
	},
	"plan_id": {
		"type": ["string"]
	},
	"usage": {
		"type": ["string", "boolean"]
	},
	"aws_pid": {
		"type": ["string"]
	},
	"aws_account": {
		"type": ["string"]
	},
	"k8s": {
		"type": ["string", "boolean"]
	}
  }
}`

// UnmarshalLicense unmarshals License from JSON or YAML
// and validates schema
func UnmarshalLicense(bytes []byte) (License, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, LicenseSpecV3Template, DefaultDefinitions)

	var license LicenseV3
	err := utils.UnmarshalWithSchema(schema, &license, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if license.Version != V3 {
		return nil, trace.BadParameter("unsupported version %v, expected version %v", license.Version, V3)
	}

	if err := license.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &license, nil
}

// MarshalLicense marshals role to JSON or YAML.
func MarshalLicense(license License, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := license.(type) {
	case *LicenseV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", license)
	}
}
