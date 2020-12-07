package types

import (
	fmt "fmt"
	"regexp"
	time "time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// SetName sets the name of the resource
func (m *Metadata) SetName(name string) {
	m.Name = name
}

// SetExpiry sets expiry time for the object
func (m *Metadata) SetExpiry(expires time.Time) {
	m.Expires = &expires
}

// Expiry returns object expiry setting.
func (m *Metadata) Expiry() time.Time {
	if m.Expires == nil {
		return time.Time{}
	}
	return *m.Expires
}

// SetTTL sets Expires header using realtime clock
func (m *Metadata) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	expireTime := clock.Now().UTC().Add(ttl)
	m.Expires = &expireTime
}

// GetID returns resource ID
func (m *Metadata) GetID() int64 {
	return m.ID
}

// SetID sets resource ID
func (m *Metadata) SetID(id int64) {
	m.ID = id
}

// GetMetadata returns object metadata
func (m *Metadata) GetMetadata() Metadata {
	return *m
}

// GetName returns the name of the resource
func (m *Metadata) GetName() string {
	return m.Name
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (m *Metadata) CheckAndSetDefaults() error {
	if m.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if m.Namespace == "" {
		m.Namespace = defaults.Namespace
	}

	// adjust expires time to utc if it's set
	if m.Expires != nil {
		utils.UTC(m.Expires)
	}

	for key := range m.Labels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	return nil
}

const labelPattern = `^[a-zA-Z/.0-9_*-]+$`

var validLabelKey = regexp.MustCompile(labelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}

// MetadataSchema is a schema for resource metadata
var MetadataSchema = fmt.Sprintf(baseMetadataSchema, labelPattern)

const baseMetadataSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["name"],
  "properties": {
    "name": {"type": "string"},
    "namespace": {"type": "string", "default": "default"},
    "description": {"type": "string"},
    "expires": {"type": "string"},
    "id": {"type": "integer"},
    "labels": {
      "type": "object",
      "additionalProperties": false,
      "patternProperties": {
         "%s":  { "type": "string" }
      }
    }
  }
}`
