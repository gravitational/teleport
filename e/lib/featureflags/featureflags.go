package featureflags

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// Kind represents feature flags resource name
	Kind = "flags"
	// MetaName is a name for feature flags resource
	MetaName = "flags"
)

// Flags defines feature flags used in teleport
type Flags interface {
	services.Resource
	// GetReportsUsage returns true if teleport cluster reports usage
	// to control plane
	GetReportsUsage() services.Bool

	// SetReportsUsage sets usage report
	SetReportsUsage(services.Bool)

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
	GetSupportsKubernetes() services.Bool

	// SetSupportsKubernetes sets kubernetes support flag
	SetSupportsKubernetes(services.Bool)

	// String represents a human readable version of authentication settings.
	String() string

	// CheckAndSetDefaults sets and default values and then
	// verifies the constraints for Flags.
	CheckAndSetDefaults() error
}

// New is a convenience method to to create FlagsV3.
func New(name string, spec SpecV3) (Flags, error) {
	return &FlagsV3{
		Kind:    Kind,
		Version: services.V3,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, nil
}

// MustNew is like New, but panics in case of error,
// used in tests
func MustNew(name string, spec SpecV3) Flags {
	out, err := New(name, spec)
	if err != nil {
		panic(err)
	}
	return out
}

// FlagsV3 implements Flags with resource version V3
type FlagsV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata services.Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec SpecV3 `json:"spec"`
}

// GetName returns the name of the resource
func (c *FlagsV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource
func (c *FlagsV3) SetName(name string) {
	c.Metadata.Name = name
}

// Expiry returns object expiry setting
func (c *FlagsV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (c *FlagsV3) SetExpiry(t time.Time) {
	c.Metadata.SetExpiry(t)
}

// SetTTL sets Expires header using current clock
func (c *FlagsV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *FlagsV3) GetMetadata() services.Metadata {
	return c.Metadata
}

// GetReportsUsage returns true if teleport cluster reports usage
// to control plane
func (c *FlagsV3) GetReportsUsage() services.Bool {
	return c.Spec.ReportsUsage
}

// SetReportsUsage sets usage report
func (c *FlagsV3) SetReportsUsage(reports services.Bool) {
	c.Spec.ReportsUsage = reports
}

// CheckAndSetDefaults verifies the constraints for Flags.
func (c *FlagsV3) CheckAndSetDefaults() error {
	return c.Metadata.CheckAndSetDefaults()
}

// GetAWSProductID returns product ID that limits usage to AWS instance
// with a similar product ID
func (c *FlagsV3) GetAWSProductID() string {
	return c.Spec.AWSProductID
}

// SetAWSProductID sets AWS product ID
func (c *FlagsV3) SetAWSProductID(pid string) {
	c.Spec.AWSProductID = pid
}

// GetAWSAccountID limits usage to AWS instance within account ID
func (c *FlagsV3) GetAWSAccountID() string {
	return c.Spec.AWSAccountID
}

// SetAWSAccountID sets AWS account ID that will be limiting
// usage to AWS instance
func (c *FlagsV3) SetAWSAccountID(accountID string) {
	c.Spec.AWSAccountID = accountID
}

// GetSupportsKubernetes returns kubernetes support flag
func (c *FlagsV3) GetSupportsKubernetes() services.Bool {
	return c.Spec.SupportsKubernetes
}

// SetSupportsKubernetes sets kubernetes support flag
func (c *FlagsV3) SetSupportsKubernetes(supportsK8s services.Bool) {
	c.Spec.SupportsKubernetes = supportsK8s
}

// String represents a human readable version of authentication settings.
func (c *FlagsV3) String() string {
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

// SpecV3 is the actual data we care about for FlagsV3.
type SpecV3 struct {
	// ReportsUsage is turned on when system reports usage
	ReportsUsage services.Bool `json:"usage,omitempty"`
	// AWSProductID limits usage to AWS instance with a product ID
	AWSProductID string `json:"aws_pid,omitempty"`
	// AWSAccountID limits usage to AWS instance within account ID
	AWSAccountID string `json:"aws_account,omitempty"`
	// SupportsKubernetes turns kubernetes support on or off
	SupportsKubernetes services.Bool `json:"k8s"`
}

// SpecV3Template is a template for V3 featureflags JSON schema
const SpecV3Template = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
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

// GetSchema returns the schema with optionally injected
// schema for extensions.
func GetSchema() string {
	return fmt.Sprintf(services.V2SchemaTemplate, services.MetadataSchema, SpecV3Template, services.DefaultDefinitions)
}

// Unmarshal unmarshals Feature flags from JSON or YAML
// and validates schema
func Unmarshal(bytes []byte) (Flags, error) {
	var flags FlagsV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	err := utils.UnmarshalWithSchema(GetSchema(), &flags, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if flags.Version != services.V3 {
		return nil, trace.BadParameter("unsupported version %v, expected version %v", flags.Version, services.V3)
	}

	if err := flags.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &flags, nil
}

// Marshal marshals role to JSON or YAML.
func Marshal(flags Flags, opts ...services.MarshalOption) ([]byte, error) {
	return json.Marshal(flags)
}
