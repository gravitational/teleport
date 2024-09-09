/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalConfig specifies marshaling options
type MarshalConfig struct {
	// Version specifies a particular version we should marshal resources with
	Version string

	// Revision of the resource to assign.
	Revision string

	// PreserveRevision preserves revision in resource
	// specs when marshaling
	PreserveRevision bool

	// Expires is an optional expiry time
	Expires time.Time
}

// GetVersion returns explicitly provided version or sets latest as default
func (m *MarshalConfig) GetVersion() string {
	if m.Version == "" {
		return types.V2
	}
	return m.Version
}

// MarshalOption sets marshaling option
type MarshalOption func(c *MarshalConfig) error

// CollectOptions collects all options from functional arg and returns config
func CollectOptions(opts []MarshalOption) (*MarshalConfig, error) {
	var cfg MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
}

// AddOptions adds marshal options and returns a new copy
func AddOptions(opts []MarshalOption, add ...MarshalOption) []MarshalOption {
	out := make([]MarshalOption, len(opts), len(opts)+len(add))
	copy(out, opts)
	return append(opts, add...)
}

// WithRevision assigns Revision to the resource
func WithRevision(rev string) MarshalOption {
	return func(c *MarshalConfig) error {
		c.Revision = rev
		return nil
	}
}

// WithExpires assigns expiry value
func WithExpires(expires time.Time) MarshalOption {
	return func(c *MarshalConfig) error {
		c.Expires = expires
		return nil
	}
}

// WithVersion sets marshal version
func WithVersion(v string) MarshalOption {
	return func(c *MarshalConfig) error {
		switch v {
		case types.V1, types.V2, types.V3:
			c.Version = v
			return nil
		default:
			return trace.BadParameter("version '%v' is not supported", v)
		}
	}
}

// PreserveRevision preserves revision when
// marshaling value
func PreserveRevision() MarshalOption {
	return func(c *MarshalConfig) error {
		c.PreserveRevision = true
		return nil
	}
}

// ParseShortcut parses resource shortcut
// Generally, this should include the plural of a singular resource name or vice
// versa.
func ParseShortcut(in string) (string, error) {
	if in == "" {
		return "", trace.BadParameter("missing resource name")
	}
	switch strings.ToLower(in) {
	case types.KindRole, "roles":
		return types.KindRole, nil
	case types.KindNamespace, "namespaces", "ns":
		return types.KindNamespace, nil
	case types.KindAuthServer, "auth_servers", "auth":
		return types.KindAuthServer, nil
	case types.KindProxy, "proxies":
		return types.KindProxy, nil
	case types.KindNode, "nodes":
		return types.KindNode, nil
	case types.KindOIDCConnector:
		return types.KindOIDCConnector, nil
	case types.KindSAMLConnector:
		return types.KindSAMLConnector, nil
	case types.KindGithubConnector:
		return types.KindGithubConnector, nil
	case types.KindConnectors, "connector":
		return types.KindConnectors, nil
	case types.KindUser, "users":
		return types.KindUser, nil
	case types.KindCertAuthority, "cert_authorities", "cas":
		return types.KindCertAuthority, nil
	case types.KindReverseTunnel, "reverse_tunnels", "rts":
		return types.KindReverseTunnel, nil
	case types.KindTrustedCluster, "tc", "cluster", "clusters":
		return types.KindTrustedCluster, nil
	case types.KindClusterAuthPreference, "cluster_authentication_preferences", "cluster_auth_preferences", "cap":
		return types.KindClusterAuthPreference, nil
	case types.KindUIConfig, "ui":
		return types.KindUIConfig, nil
	case types.KindClusterNetworkingConfig, "networking_config", "networking", "net_config", "netconfig":
		return types.KindClusterNetworkingConfig, nil
	case types.KindSessionRecordingConfig, "recording_config", "session_recording", "rec_config", "recconfig":
		return types.KindSessionRecordingConfig, nil
	case types.KindExternalAuditStorage:
		return types.KindExternalAuditStorage, nil
	case types.KindRemoteCluster, "remote_clusters", "rc", "rcs":
		return types.KindRemoteCluster, nil
	case types.KindSemaphore, "semaphores", "sem", "sems":
		return types.KindSemaphore, nil
	case types.KindKubernetesCluster, "kube_clusters":
		return types.KindKubernetesCluster, nil
	case types.KindKubeServer, "kube_servers":
		return types.KindKubeServer, nil
	case types.KindLock, "locks":
		return types.KindLock, nil
	case types.KindDatabaseServer:
		return types.KindDatabaseServer, nil
	case types.KindNetworkRestrictions:
		return types.KindNetworkRestrictions, nil
	case types.KindDatabase:
		return types.KindDatabase, nil
	case types.KindApp, "apps":
		return types.KindApp, nil
	case types.KindAppServer, "app_servers":
		return types.KindAppServer, nil
	case types.KindWindowsDesktopService, "windows_service", "win_desktop_service", "win_service":
		return types.KindWindowsDesktopService, nil
	case types.KindWindowsDesktop, "win_desktop":
		return types.KindWindowsDesktop, nil
	case types.KindToken, "tokens":
		return types.KindToken, nil
	case types.KindInstaller:
		return types.KindInstaller, nil
	case types.KindDatabaseService, types.KindDatabaseService + "s":
		return types.KindDatabaseService, nil
	case types.KindLoginRule, types.KindLoginRule + "s":
		return types.KindLoginRule, nil
	case types.KindSAMLIdPServiceProvider, types.KindSAMLIdPServiceProvider + "s", "saml_sp", "saml_sps":
		return types.KindSAMLIdPServiceProvider, nil
	case types.KindUserGroup, types.KindUserGroup + "s", "usergroup", "usergroups":
		return types.KindUserGroup, nil
	case types.KindDevice, types.KindDevice + "s":
		return types.KindDevice, nil
	case types.KindOktaImportRule, types.KindOktaImportRule + "s", "oktaimportrule", "oktaimportrules":
		return types.KindOktaImportRule, nil
	case types.KindOktaAssignment, types.KindOktaAssignment + "s", "oktaassignment", "oktaassignments":
		return types.KindOktaAssignment, nil
	case types.KindClusterMaintenanceConfig, "cmc":
		return types.KindClusterMaintenanceConfig, nil
	case types.KindIntegration, types.KindIntegration + "s":
		return types.KindIntegration, nil
	case types.KindAccessList, types.KindAccessList + "s", "accesslist", "accesslists":
		return types.KindAccessList, nil
	case types.KindDiscoveryConfig, types.KindDiscoveryConfig + "s", "discoveryconfig", "discoveryconfigs":
		return types.KindDiscoveryConfig, nil
	case types.KindAuditQuery:
		return types.KindAuditQuery, nil
	case types.KindSecurityReport:
		return types.KindSecurityReport, nil
	case types.KindServerInfo:
		return types.KindServerInfo, nil
	case types.KindBot, "bots":
		return types.KindBot, nil
	case types.KindBotInstance, types.KindBotInstance + "s":
		return types.KindBotInstance, nil
	case types.KindDatabaseObjectImportRule, "db_object_import_rules", "database_object_import_rule":
		return types.KindDatabaseObjectImportRule, nil
	case types.KindAccessMonitoringRule:
		return types.KindAccessMonitoringRule, nil
	case types.KindDatabaseObject, "database_object":
		return types.KindDatabaseObject, nil
	case types.KindCrownJewel, "crown_jewels":
		return types.KindCrownJewel, nil
	case types.KindVnetConfig:
		return types.KindVnetConfig, nil
	case types.KindAccessRequest, types.KindAccessRequest + "s", "accessrequest", "accessrequests":
		return types.KindAccessRequest, nil
	case types.KindPlugin, types.KindPlugin + "s":
		return types.KindPlugin, nil
	case types.KindAccessGraphSettings, "ags":
		return types.KindAccessGraphSettings, nil
	case types.KindSPIFFEFederation, types.KindSPIFFEFederation + "s":
		return types.KindSPIFFEFederation, nil
	case types.KindStaticHostUser, types.KindStaticHostUser + "s", "host_user", "host_users":
		return types.KindStaticHostUser, nil
	}
	return "", trace.BadParameter("unsupported resource: %q - resources should be expressed as 'type/name', for example 'connector/github'", in)
}

// ParseRef parses resource reference eg daemonsets/ds1
func ParseRef(ref string) (*Ref, error) {
	if ref == "" {
		return nil, trace.BadParameter("missing value")
	}
	parts := strings.FieldsFunc(ref, isDelimiter)
	switch len(parts) {
	case 1:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut}, nil
	case 2:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, Name: parts[1]}, nil
	case 3:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, SubKind: parts[1], Name: parts[2]}, nil
	}
	return nil, trace.BadParameter("failed to parse '%v'", ref)
}

// isDelimiter returns true if rune is space or /
func isDelimiter(r rune) bool {
	switch r {
	case '\t', ' ', '/':
		return true
	}
	return false
}

// Ref is a resource reference.  Typically of the form kind/name,
// but sometimes of the form kind/subkind/name.
type Ref struct {
	Kind    string
	SubKind string
	Name    string
}

// Set sets the name of the resource
func (r *Ref) Set(v string) error {
	out, err := ParseRef(v)
	if err != nil {
		return err
	}
	*r = *out
	return nil
}

func (r *Ref) String() string {
	if r.SubKind == "" {
		if r.Name == "" {
			return r.Kind
		}
		return fmt.Sprintf("%s/%s", r.Kind, r.Name)
	}
	return fmt.Sprintf("%s/%s/%s", r.Kind, r.SubKind, r.Name)
}

// Refs is a set of resource references
type Refs []Ref

// ParseRefs parses a comma-separated string of resource references (eg "users/alice,users/bob")
func ParseRefs(refs string) (Refs, error) {
	if refs == "all" {
		return []Ref{{Kind: "all"}}, nil
	}
	var escaped bool
	isBreak := func(r rune) bool {
		brk := false
		switch r {
		case ',':
			brk = true && !escaped
			escaped = false
		case '\\':
			escaped = true && !escaped
		default:
			escaped = false
		}
		return brk
	}
	var parsed []Ref
	split := fieldsFunc(strings.TrimSpace(refs), isBreak)
	for _, s := range split {
		ref, err := ParseRef(strings.ReplaceAll(s, `\,`, `,`))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		parsed = append(parsed, *ref)
	}
	return parsed, nil
}

// Set sets the value of `r` from a comma-separated string of resource
// references (in-place equivalent of `ParseRefs`).
func (r *Refs) Set(v string) error {
	refs, err := ParseRefs(v)
	if err != nil {
		return trace.Wrap(err)
	}
	*r = refs
	return nil
}

// IsAll checks if refs is special wildcard case `all`.
func (r *Refs) IsAll() bool {
	refs := *r
	if len(refs) != 1 {
		return false
	}
	return refs[0].Kind == "all"
}

func (r *Refs) String() string {
	var builder strings.Builder
	for i, ref := range *r {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(ref.String())
	}
	return builder.String()
}

// fieldsFunc is an exact copy of the current implementation of `strings.FieldsFunc`.
// The docs of `strings.FieldsFunc` indicate that future implementations may not call
// `f` on every rune, may not preserve ordering, or may panic if `f` does not return the
// same output for every instance of a given rune.  All of these changes would break
// our implementation of backslash-escaping, so we're using a local copy.
func fieldsFunc(s string, f func(rune) bool) []string {
	// A span is used to record a slice of s of the form s[start:end].
	// The start index is inclusive and the end index is exclusive.
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// Find the field start and end indices.
	wasField := false
	fromIndex := 0
	for i, rune := range s {
		if f(rune) {
			if wasField {
				spans = append(spans, span{start: fromIndex, end: i})
				wasField = false
			}
		} else {
			if !wasField {
				fromIndex = i
				wasField = true
			}
		}
	}

	// Last field might end at EOF.
	if wasField {
		spans = append(spans, span{fromIndex, len(s)})
	}

	// Create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}

	return a
}

// marshalerMutex is a mutex for resource marshalers/unmarshalers
var marshalerMutex sync.RWMutex

// ResourceMarshaler handles marshaling of a specific resource type.
type ResourceMarshaler func(types.Resource, ...MarshalOption) ([]byte, error)

// ResourceUnmarshaler handles unmarshaling of a specific resource type.
type ResourceUnmarshaler func([]byte, ...MarshalOption) (types.Resource, error)

// resourceMarshalers holds a collection of marshalers organized by kind.
var resourceMarshalers = make(map[string]ResourceMarshaler)

// resourceUnmarshalers holds a collection of unmarshalers organized by kind.
var resourceUnmarshalers = make(map[string]ResourceUnmarshaler)

// GetResourceMarshalerKinds lists all registered resource marshalers by kind.
func GetResourceMarshalerKinds() []string {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	kinds := make([]string, 0, len(resourceMarshalers))
	for kind := range resourceMarshalers {
		kinds = append(kinds, kind)
	}
	return kinds
}

// RegisterResourceMarshaler registers a marshaler for resources of a specific kind.
// WARNING!!
// Registering a resource Marshaler requires lib/services/local.CreateResources
// supports the resource kind or the standard backup/restore procedure of using
// `tctl get all` and then BootstrapResources in Teleport will fail.
func RegisterResourceMarshaler(kind string, marshaler ResourceMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceMarshalers[kind] = marshaler
}

// RegisterResourceUnmarshaler registers an unmarshaler for resources of a specific kind.
func RegisterResourceUnmarshaler(kind string, unmarshaler ResourceUnmarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceUnmarshalers[kind] = unmarshaler
}

func getResourceMarshaler(kind string) (ResourceMarshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	m, ok := resourceMarshalers[kind]
	if !ok {
		return nil, false
	}
	return m, true
}

func getResourceUnmarshaler(kind string) (ResourceUnmarshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	u, ok := resourceUnmarshalers[kind]
	if !ok {
		return nil, false
	}
	return u, true
}

func init() {
	RegisterResourceMarshaler(types.KindUser, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		user, ok := resource.(types.User)
		if !ok {
			return nil, trace.BadParameter("expected User, got %T", resource)
		}
		bytes, err := MarshalUser(user, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindUser, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		user, err := UnmarshalUser(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return user, nil
	})

	RegisterResourceMarshaler(types.KindCertAuthority, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		certAuthority, ok := resource.(types.CertAuthority)
		if !ok {
			return nil, trace.BadParameter("expected CertAuthority, got %T", resource)
		}
		bytes, err := MarshalCertAuthority(certAuthority, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindCertAuthority, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		certAuthority, err := UnmarshalCertAuthority(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return certAuthority, nil
	})

	RegisterResourceMarshaler(types.KindTrustedCluster, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		trustedCluster, ok := resource.(types.TrustedCluster)
		if !ok {
			return nil, trace.BadParameter("expected TrustedCluster, got %T", resource)
		}
		bytes, err := MarshalTrustedCluster(trustedCluster, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindTrustedCluster, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		trustedCluster, err := UnmarshalTrustedCluster(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return trustedCluster, nil
	})

	RegisterResourceMarshaler(types.KindGithubConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		githubConnector, ok := resource.(types.GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", resource)
		}
		bytes, err := MarshalOSSGithubConnector(githubConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindGithubConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		githubConnector, err := UnmarshalOSSGithubConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return githubConnector, nil
	})

	RegisterResourceMarshaler(types.KindSAMLConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		samlConnector, ok := resource.(types.SAMLConnector)
		if !ok {
			return nil, trace.BadParameter("expected SAMLConnector, got %T", resource)
		}
		bytes, err := MarshalSAMLConnector(samlConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindSAMLConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		samlConnector, err := UnmarshalSAMLConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return samlConnector, nil
	})

	RegisterResourceMarshaler(types.KindOIDCConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		oidConnector, ok := resource.(types.OIDCConnector)
		if !ok {
			return nil, trace.BadParameter("expected OIDCConnector, got %T", resource)
		}
		bytes, err := MarshalOIDCConnector(oidConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindOIDCConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		oidcConnector, err := UnmarshalOIDCConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return oidcConnector, nil
	})

	RegisterResourceMarshaler(types.KindRole, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		role, ok := resource.(types.Role)
		if !ok {
			return nil, trace.BadParameter("expected Role, got %T", resource)
		}
		bytes, err := MarshalRole(role, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindRole, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		role, err := UnmarshalRole(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return role, nil
	})
	RegisterResourceMarshaler(types.KindToken, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		token, ok := resource.(types.ProvisionToken)
		if !ok {
			return nil, trace.BadParameter("expected Token, got %T", resource)
		}
		bytes, err := MarshalProvisionToken(token, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindToken, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		token, err := UnmarshalProvisionToken(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return token, nil
	})
	RegisterResourceMarshaler(types.KindLock, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		lock, ok := resource.(types.Lock)
		if !ok {
			return nil, trace.BadParameter("expected lock, got %T", resource)
		}
		bytes, err := MarshalLock(lock, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindLock, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		lock, err := UnmarshalLock(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return lock, nil
	})
	RegisterResourceMarshaler(types.KindClusterNetworkingConfig, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		cnc, ok := resource.(types.ClusterNetworkingConfig)
		if !ok {
			return nil, trace.BadParameter("expected cluster_networking_config go %T", resource)
		}
		bytes, err := MarshalClusterNetworkingConfig(cnc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindClusterNetworkingConfig, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		cnc, err := UnmarshalClusterNetworkingConfig(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cnc, nil
	})
	RegisterResourceMarshaler(types.KindClusterAuthPreference, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		ap, ok := resource.(types.AuthPreference)
		if !ok {
			return nil, trace.BadParameter("expected cluster_auth_preference go %T", resource)
		}
		bytes, err := MarshalAuthPreference(ap, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindClusterAuthPreference, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		ap, err := UnmarshalAuthPreference(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ap, nil
	})
	RegisterResourceUnmarshaler(types.KindBot, func(bytes []byte, option ...MarshalOption) (types.Resource, error) {
		b := &machineidv1pb.Bot{}
		if err := protojson.Unmarshal(bytes, b); err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(b), nil
	})
}

// CheckAndSetDefaults calls [r.CheckAndSetDefaults] if r implements the method.
// If r does not implement, then this is a nop.
//
// This method exists for backwards compatibility with old-style resources.
// Prefer using RFD 153 style resources, passing concrete types and running
// validations before storage writes only.
func CheckAndSetDefaults(r any) error {
	if r, ok := r.(interface{ CheckAndSetDefaults() error }); ok {
		return trace.Wrap(r.CheckAndSetDefaults())
	}

	return nil
}

// MarshalResource attempts to marshal a resource dynamically, returning NotImplementedError
// if no marshaler has been registered.
//
// NOTE: This function only supports the subset of resources which may be imported/exported
// by users (e.g. via `tctl get`).
func MarshalResource(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
	marshal, ok := getResourceMarshaler(resource.GetKind())
	if !ok {
		return nil, trace.NotImplemented("cannot dynamically marshal resources of kind %q", resource.GetKind())
	}
	// Handle the case where `resource` was never fully unmarshaled.
	if r, ok := resource.(*UnknownResource); ok {
		u, err := UnmarshalResource(r.GetKind(), r.Raw, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resource = u
	}
	m, err := marshal(resource, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return m, nil
}

// UnmarshalResource attempts to unmarshal a resource dynamically, returning NotImplementedError
// if no unmarshaler has been registered.
//
// NOTE: This function only supports the subset of resources which may be imported/exported
// by users (e.g. via `tctl get`).
func UnmarshalResource(kind string, raw []byte, opts ...MarshalOption) (types.Resource, error) {
	unmarshal, ok := getResourceUnmarshaler(kind)
	if !ok {
		return nil, trace.NotImplemented("cannot dynamically unmarshal resources of kind %q", kind)
	}
	u, err := unmarshal(raw, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// UnknownResource is used to detect resources
type UnknownResource struct {
	types.ResourceHeader
	// Raw is raw representation of the resource
	Raw []byte
}

// UnmarshalJSON unmarshals header and captures raw state
func (u *UnknownResource) UnmarshalJSON(raw []byte) error {
	var h types.ResourceHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return trace.Wrap(err)
	}
	u.Raw = make([]byte, len(raw))
	u.ResourceHeader = h
	copy(u.Raw, raw)
	return nil
}

// setResourceName modifies the types.Metadata argument in place, setting the resource name.
// The name is calculated based on nameParts arguments which are joined by hyphens "-".
// If a name override label is present, it will replace the *first* name part.
func setResourceName(overrideLabels []string, meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	nameParts := append([]string{firstNamePart}, extraNameParts...)

	// apply override
	for _, overrideLabel := range overrideLabels {
		if override, found := meta.Labels[overrideLabel]; found && override != "" {
			nameParts[0] = override
			break
		}
	}

	meta.Name = strings.Join(nameParts, "-")

	return meta
}

type resetProtoResource interface {
	protoadapt.MessageV1
	SetRevision(string)
}

// maybeResetProtoRevision returns a clone of [r] with the identifiers reset to default values if
// preserveRevision is true, otherwise this is a nop, and the original value is returned unaltered.
//
// Prefer maybeResetProtoRevisionv2 for newer RFD153-style resources, only one or the other should compile
// for any given type.
func maybeResetProtoRevision[T resetProtoResource](preserveRevision bool, r T) T {
	if preserveRevision {
		return r
	}

	cp := apiutils.CloneProtoMsg(r)
	cp.SetRevision("")
	return cp
}

// ProtoResource abstracts a resource defined as a protobuf message.
type ProtoResource interface {
	proto.Message
	// GetMetadata returns the generic resource metadata.
	GetMetadata() *headerv1.Metadata
}

// ProtoResourcePtr is a ProtoResource that is also a pointer to T.
type ProtoResourcePtr[T any] interface {
	*T
	ProtoResource
}

// maybeResetProtoRevisionv2 returns a clone of [r] with the identifiers reset to default values if
// preserveRevision is true, otherwise this is a nop, and the original value is returned unaltered.
//
// This is like maybeResetProtoRevision but made for newer RFD153-style resources which implement a
// different interface, only one or the other should compile for any given type.
func maybeResetProtoRevisionv2[T ProtoResource](preserveRevision bool, r T) T {
	if preserveRevision {
		return r
	}

	cp := proto.Clone(r).(T)
	cp.GetMetadata().Revision = ""
	return cp
}

// MarshalProtoResource marshals a ProtoResource to JSON using [protojson.Marshal] and respecting [opts].
func MarshalProtoResource[T ProtoResource](resource T, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource = maybeResetProtoRevisionv2(cfg.PreserveRevision, resource)
	data, err := protojson.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalProtoResource unmarshals a ProtoResource from JSON using [protojson.Unmarshal] and respecting [opts].
// It is paramaterized on types T and U, where T is a pointer type that implements ProtoResource, and U is the
// type that T points to. This is so that it can allocate an instance of U to unmarshal into without
// reflection.
func UnmarshalProtoResource[T ProtoResourcePtr[U], U any](data []byte, opts ...MarshalOption) (T, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("nothing to unmarshal")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resource T = new(U)
	err = protojson.Unmarshal(data, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		resource.GetMetadata().Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		resource.GetMetadata().Expires = timestamppb.New(cfg.Expires)
	}
	return resource, nil
}

// FastMarshalProtoResourceDeprecated marshals a ProtoResource to JSON using [utils.FastMarshal] and respecting [opts].
//
// Deprecated: this should not be used for new types, prefer [MarshalProtoResource]. Existing types should not
// be converted to maintain compatibility.
func FastMarshalProtoResourceDeprecated[T ProtoResource](resource T, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource = maybeResetProtoRevisionv2(cfg.PreserveRevision, resource)
	data, err := utils.FastMarshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// FastUnmarshalProtoResourceDeprecated unmarshals a ProtoResource from JSON using [utils.FastUnmarshal] and respecting [opts].
// It is paramaterized on types T and U, where T is a pointer type that implements ProtoResource, and U is the
// type that T points to. This is so that it can allocate an instance of U to unmarshal into without
// reflection.
//
// Deprecated: this should not be used for new types, prefer [UnmarshalProtoResource]. Existing types should not
// be converted to maintain compatibility.
func FastUnmarshalProtoResourceDeprecated[T ProtoResourcePtr[U], U any](data []byte, opts ...MarshalOption) (T, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("nothing to unmarshal")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resource T = new(U)
	err = utils.FastUnmarshal(data, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		resource.GetMetadata().Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		resource.GetMetadata().Expires = timestamppb.New(cfg.Expires)
	}
	return resource, nil
}
