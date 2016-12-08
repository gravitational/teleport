package services

import (
	"encoding/json"
	"time"

	"github.com/gravitational/configure/jsonschema"
	"github.com/gravitational/trace"
)

const (
	// DefaultAPIGroup is a default group of permissions API,
	// lets us to add different permission types
	DefaultAPIGroup = "gravitational.io/teleport"

	// ActionRead grants read access to resource
	ActionRead = "read"

	// ActionDelete allows to delete some resource
	ActionDelete = "delete"

	// ActionUpsert allows to update or create the resource
	ActionUpdate = "upsert"

	// Wildcard is a special wildcard character matching everything
	Wildcard = "*"

	// DefaultNamespace is a default namespace of all resources
	DefaultNamespace = "default"
)

// Role specifies a set of permissions to access the cluster
type Role interface {
	// Kind returns resource kind
	Kind() string
	// Version returns version of this resource
	Version() string
	// ID is a unique role ID
	ID() string
	// Permissions returns a list of permissions collected for this resource
	Permissions() ([]Permission, error)
	// StandardPermission returns list of standard permissions
	StandardPermissions() ([]PermissionResource, error)
}

// Permission allows us to add pluggable permissions in the future
type Permission interface {
	// APIGroup communicates back API group of this permission
	APIGroup() string
	// Payload returns raw payload of this permission
	Payload() []byte
	// StandardPermission returns permission assuming DefaultAPIGroup
	StandardPermission() (*PermissionResource, error)
}

// PermissionsResource specifies teleport specific permission,
// but leaves room
type PermissionResource struct {
	// APIGroup allows us to detect extensions vs Teleport-native extensions
	APIGroup string `json:"api_group"`
	// Resource is used to control access to stored resources
	Resource *ResourcePermission `json:"resource"`
	// SSH is used to set up SSH access
	SSH *SSHPermission `json:"ssh"`
	// Access sets types of access allowed for this role
	Access *AccessPermission `json:"access"`
}

// AccessPermission controls types of access and their parameters
type AccessPermission struct {
	// SSH controls access via SSH CLI
	SSH SSHAccess `json:"ssh"`
	// Web controls access via UI
	Web WebAccess `json:"web"`
}

// SSHAccess controls SSH access
type SSHAccess struct {
	// Enabled turns access on or off
	Enabled bool `json:"enabled"`
	// MaxTTL sets expiration date for issued certificate
	MaxTTL time.Duration `json:"max_ttl"`
}

// WebAccess controls access via web UI
type WebAccess struct {
	// Enabled turns access on or off
	Enabled bool `json:"enabled"`
	// MaxTTL is a maximum duration of a web session
	MaxTTL time.Duration `json:"max_ttl"`
}

// ResourcePermission controls access to particular resource, e.g.
// session or log entry, or role itself
type ResourcePermission struct {
	// Kind is a resource kind, e.g. Session, or CertAuthority
	Kind string `json:"kind"`
	// Actions is a set of actions allowed on this resource, e.g. 'upsert' or 'create'
	Actions map[string]bool `json:"actions"`
	// Namespaces is a set of namespaces this permission grants access to
	Namespaces map[string]bool `json:"namespace"`
}

// SSHPermission controls SSH access to nodes
type SSHPermission struct {
	// Logins is a set of Unix login users this role has access to
	Logins map[string]bool `json:"logins"`
	// NodesSelector is a selector to match nodes
	NodesSelector map[string]string `json:"nodes"`
	// Channels limits types of channels allowed to open
	Channels map[string]string `json:"channels"`
}

// NodeSelector helps to select nodes
type NodeSelector struct {
	// MatchLabels finds nodes matching certain labels
	MatchLabels map[string]string `json:"match_labels"`
	// Namespaces matches namespaces
	Namespaces map[string]bool
}

type roleRaw struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	ID      string `json:"id"`
	Spec    struct {
		Permissions []json.RawMessage
	} `json:"spec"`
}

// PermissionHeader is a permission header that we parse
// to determine the permission type
type PermissionHeader struct {
	APIGroup string `json:"api_group"`
}

type rawPermission struct {
	PermissionHeader
	Payload []byte
}

func ParseRoleJSON(data []byte) (Role, error) {

	schema, err := jsonschema.New([]byte(RoleSchema))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, trace.Wrap(err)
	}
	// schema will check format and set defaults
	processed, err := schema.ProcessObject(raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// since ProcessObject works with unstructured data, the
	// manifest needs to be re-interpreted in structured form
	bytes, err := json.Marshal(processed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// peek the metadata to determine the application type
	rawRole := roleRaw{}
	if err := json.Unmarshal(data, &rawRole); err != nil {
		return nil, trace.Wrap(err)
	}

	var permissions []PermissionResource
	var allPermissions []rawPermission

	for i := range rawRole.Spec.Permissions {
		data := rawRole.Spec.Permissions[i]
		var header PermissionHeader
		if err := json.Unmarshal(data, &header); err != nil {
			return nil, trace.Wrap(err)
		}
		switch header.APIGroup {
			case "", 
		}
	}

	manifest := Manifest{ObjectHeader: header}
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return nil, trace.Wrap(err)
	}
	return &manifest, nil
}

const RoleSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["id", "spec"],
  "properties": {
    "kind": {"type": "string", "default": "role"},
    "version": {"type": "string", "default": "v1"},
    "id": {"type": "string",  "pattern": "^[a-zA-Z0-9_-\.]+$"},
    "spec": {
      "type": "object",
      "default": {}
    }
  }
}`

const NodeSelectorSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "properties": {
    "match_labels": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
      }
    },
    "namespaces": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "patternProperties": {
         ".*":  { "type": "boolean" }
      }
    }
  }
}`
