/*
Copyright 2020 Gravitational, Inc.

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
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
)

// Server represents a Node, Proxy or Auth server in a Teleport cluster
type Server interface {
	// ResourceWithLabels provides common resource headers
	ResourceWithLabels
	// GetTeleportVersion returns the teleport version the server is running on
	GetTeleportVersion() string
	// GetAddr return server address
	GetAddr() string
	// GetHostname returns server hostname
	GetHostname() string
	// GetNamespace returns server namespace
	GetNamespace() string
	// GetLabels returns server's static label key pairs
	GetLabels() map[string]string
	// GetCmdLabels gets command labels
	GetCmdLabels() map[string]CommandLabel
	// SetCmdLabels sets command labels.
	SetCmdLabels(cmdLabels map[string]CommandLabel)
	// GetPublicAddr returns a public address where this server can be reached.
	GetPublicAddr() string
	// GetPublicAddrs returns a list of public addresses where this server can be reached.
	GetPublicAddrs() []string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// GetUseTunnel gets if a reverse tunnel should be used to connect to this node.
	GetUseTunnel() bool
	// SetUseTunnel sets if a reverse tunnel should be used to connect to this node.
	SetUseTunnel(bool)
	// String returns string representation of the server
	String() string
	// SetAddr sets server address
	SetAddr(addr string)
	// SetPublicAddrs sets the public addresses where this server can be reached.
	SetPublicAddrs([]string)
	// SetNamespace sets server namespace
	SetNamespace(namespace string)
	// GetApps gets the list of applications this server is proxying.
	// DELETE IN 9.0.
	GetApps() []*App
	// GetApps gets the list of applications this server is proxying.
	// DELETE IN 9.0.
	SetApps([]*App)
	// GetPeerAddr returns the peer address of the server.
	GetPeerAddr() string
	// SetPeerAddr sets the peer address of the server.
	SetPeerAddr(string)
	// ProxiedService provides common methods for a proxied service.
	ProxiedService
	// MatchAgainst takes a map of labels and returns True if this server
	// has ALL of them
	//
	// Any server matches against an empty label set
	MatchAgainst(labels map[string]string) bool
	// LabelsString returns a comma separated string with all node's labels
	LabelsString() string

	// DeepCopy creates a clone of this server value
	DeepCopy() Server

	// GetCloudMetadata gets the cloud metadata for the server.
	GetCloudMetadata() *CloudMetadata
	// SetCloudMetadata sets the server's cloud metadata.
	SetCloudMetadata(meta *CloudMetadata)

	// IsOpenSSHNode returns whether the connection to this Server must use OpenSSH.
	// This returns true for SubKindOpenSSHNode and SubKindOpenSSHEphemeralKeyNode.
	IsOpenSSHNode() bool
}

// NewServer creates an instance of Server.
func NewServer(name, kind string, spec ServerSpecV2) (Server, error) {
	return NewServerWithLabels(name, kind, spec, map[string]string{})
}

// NewServerWithLabels is a convenience method to create
// ServerV2 with a specific map of labels.
func NewServerWithLabels(name, kind string, spec ServerSpecV2, labels map[string]string) (Server, error) {
	server := &ServerV2{
		Kind: kind,
		Metadata: Metadata{
			Name:   name,
			Labels: labels,
		},
		Spec: spec,
	}
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// NewNode is a convenience method to create a Server of Kind Node.
func NewNode(name, subKind string, spec ServerSpecV2, labels map[string]string) (Server, error) {
	server := &ServerV2{
		Kind:    KindNode,
		SubKind: subKind,
		Metadata: Metadata{
			Name:   name,
			Labels: labels,
		},
		Spec: spec,
	}
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// GetVersion returns resource version
func (s *ServerV2) GetVersion() string {
	return s.Version
}

// GetTeleportVersion returns the teleport version the server is running on
func (s *ServerV2) GetTeleportVersion() string {
	return s.Spec.Version
}

// GetKind returns resource kind
func (s *ServerV2) GetKind() string {
	return s.Kind
}

// GetSubKind returns resource sub kind
func (s *ServerV2) GetSubKind() string {
	// if the server is a node subkind isn't set, this is a teleport node.
	if s.Kind == KindNode && s.SubKind == "" {
		return SubKindTeleportNode
	}

	return s.SubKind
}

// SetSubKind sets resource subkind
func (s *ServerV2) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns resource ID
func (s *ServerV2) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets resource ID
func (s *ServerV2) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetMetadata returns metadata
func (s *ServerV2) GetMetadata() Metadata {
	return s.Metadata
}

// SetNamespace sets server namespace
func (s *ServerV2) SetNamespace(namespace string) {
	s.Metadata.Namespace = namespace
}

// SetAddr sets server address
func (s *ServerV2) SetAddr(addr string) {
	s.Spec.Addr = addr
}

// SetExpiry sets expiry time for the object
func (s *ServerV2) SetExpiry(expires time.Time) {
	s.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (s *ServerV2) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// SetPublicAddrs sets the public proxy addresses where this server can be reached.
func (s *ServerV2) SetPublicAddrs(addrs []string) {
	s.Spec.PublicAddrs = addrs
	// DELETE IN 15.0. (Joerger) PublicAddr deprecated in favor of PublicAddrs
	if len(addrs) != 0 {
		s.Spec.PublicAddr = addrs[0]
	}
}

// GetName returns server name
func (s *ServerV2) GetName() string {
	return s.Metadata.Name
}

// SetName sets the name of the TrustedCluster.
func (s *ServerV2) SetName(e string) {
	s.Metadata.Name = e
}

// GetAddr return server address
func (s *ServerV2) GetAddr() string {
	return s.Spec.Addr
}

// GetPublicAddr returns a public address where this server can be reached.
func (s *ServerV2) GetPublicAddr() string {
	addrs := s.GetPublicAddrs()
	if len(addrs) != 0 {
		return addrs[0]
	}
	return ""
}

// GetPublicAddrs returns a list of public addresses where this server can be reached.
func (s *ServerV2) GetPublicAddrs() []string {
	// DELETE IN 15.0. (Joerger) PublicAddr deprecated in favor of PublicAddrs
	if len(s.Spec.PublicAddrs) == 0 && s.Spec.PublicAddr != "" {
		return []string{s.Spec.PublicAddr}
	}
	return s.Spec.PublicAddrs
}

// GetRotation gets the state of certificate authority rotation.
func (s *ServerV2) GetRotation() Rotation {
	return s.Spec.Rotation
}

// SetRotation sets the state of certificate authority rotation.
func (s *ServerV2) SetRotation(r Rotation) {
	s.Spec.Rotation = r
}

// GetUseTunnel gets if a reverse tunnel should be used to connect to this node.
func (s *ServerV2) GetUseTunnel() bool {
	return s.Spec.UseTunnel
}

// SetUseTunnel sets if a reverse tunnel should be used to connect to this node.
func (s *ServerV2) SetUseTunnel(useTunnel bool) {
	s.Spec.UseTunnel = useTunnel
}

// GetHostname returns server hostname
func (s *ServerV2) GetHostname() string {
	return s.Spec.Hostname
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (s *ServerV2) GetLabel(key string) (value string, ok bool) {
	if cmd, ok := s.Spec.CmdLabels[key]; ok {
		return cmd.Result, ok
	}

	v, ok := s.Metadata.Labels[key]
	return v, ok
}

// GetLabels returns server's static label key pairs.
// GetLabels and GetStaticLabels are the same, and that is intentional. GetLabels
// exists to preserve backwards compatibility, while GetStaticLabels exists to
// implement ResourcesWithLabels.
func (s *ServerV2) GetLabels() map[string]string {
	return s.Metadata.Labels
}

// GetStaticLabels returns the server static labels.
// GetLabels and GetStaticLabels are the same, and that is intentional. GetLabels
// exists to preserve backwards compatibility, while GetStaticLabels exists to
// implement ResourcesWithLabels.
func (s *ServerV2) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the server static labels.
func (s *ServerV2) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// GetCmdLabels returns command labels
func (s *ServerV2) GetCmdLabels() map[string]CommandLabel {
	if s.Spec.CmdLabels == nil {
		return nil
	}
	return V2ToLabels(s.Spec.CmdLabels)
}

// Origin returns the origin value of the resource.
func (s *ServerV2) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *ServerV2) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

// SetCmdLabels sets dynamic labels.
func (s *ServerV2) SetCmdLabels(cmdLabels map[string]CommandLabel) {
	s.Spec.CmdLabels = LabelsToV2(cmdLabels)
}

// GetApps gets the list of applications this server is proxying.
func (s *ServerV2) GetApps() []*App {
	return s.Spec.Apps
}

// SetApps sets the list of applications this server is proxying.
func (s *ServerV2) SetApps(apps []*App) {
	s.Spec.Apps = apps
}

func (s *ServerV2) String() string {
	return fmt.Sprintf("Server(name=%v, namespace=%v, addr=%v, labels=%v)", s.Metadata.Name, s.Metadata.Namespace, s.Spec.Addr, s.Metadata.Labels)
}

// GetNamespace returns server namespace
func (s *ServerV2) GetNamespace() string {
	return ProcessNamespace(s.Metadata.Namespace)
}

// GetProxyID returns the proxy id this server is connected to.
func (s *ServerV2) GetProxyIDs() []string {
	return s.Spec.ProxyIDs
}

// SetProxyID sets the proxy ids this server is connected to.
func (s *ServerV2) SetProxyIDs(proxyIDs []string) {
	s.Spec.ProxyIDs = proxyIDs
}

// GetAllLabels returns the full key:value map of both static labels and
// "command labels"
func (s *ServerV2) GetAllLabels() map[string]string {
	// server labels (static and dynamic)
	labels := CombineLabels(s.Metadata.Labels, s.Spec.CmdLabels)
	return labels
}

// CombineLabels combines the passed in static and dynamic labels.
func CombineLabels(static map[string]string, dynamic map[string]CommandLabelV2) map[string]string {
	lmap := make(map[string]string)
	for key, value := range static {
		lmap[key] = value
	}
	for key, cmd := range dynamic {
		lmap[key] = cmd.Result
	}
	return lmap
}

// GetPeerAddr returns the peer address of the server.
func (s *ServerV2) GetPeerAddr() string {
	return s.Spec.PeerAddr
}

// SetPeerAddr sets the peer address of the server.
func (s *ServerV2) SetPeerAddr(addr string) {
	s.Spec.PeerAddr = addr
}

// MatchAgainst takes a map of labels and returns True if this server
// has ALL of them
//
// Any server matches against an empty label set
func (s *ServerV2) MatchAgainst(labels map[string]string) bool {
	return MatchLabels(s, labels)
}

// LabelsString returns a comma separated string of all labels.
func (s *ServerV2) LabelsString() string {
	return LabelsAsString(s.Metadata.Labels, s.Spec.CmdLabels)
}

// LabelsAsString combines static and dynamic labels and returns a comma
// separated string.
func LabelsAsString(static map[string]string, dynamic map[string]CommandLabelV2) string {
	labels := []string{}
	for key, val := range static {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range dynamic {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val.Result))
	}
	sort.Strings(labels)
	return strings.Join(labels, ",")
}

// setStaticFields sets static resource header and metadata fields.
func (s *ServerV2) setStaticFields() {
	s.Version = V2
}

// IsOpenSSHNode returns whether the connection to this Server must use OpenSSH.
// This returns true for SubKindOpenSSHNode and SubKindOpenSSHEphemeralKeyNode.
func (s *ServerV2) IsOpenSSHNode() bool {
	return s.SubKind == SubKindOpenSSHNode || s.SubKind == SubKindOpenSSHEphemeralKeyNode
}

// openSSHNodeCheckAndSetDefaults are common validations for OpenSSH nodes.
// They include SubKindOpenSSHNode and SubKindOpenSSHEphemeralKeyNode.
func (s *ServerV2) openSSHNodeCheckAndSetDefaults() error {
	if s.Spec.Addr == "" {
		return trace.BadParameter(`addr must be set when server SubKind is "openssh"`)
	}
	if len(s.GetPublicAddrs()) != 0 {
		return trace.BadParameter(`publicAddrs must not be set when server SubKind is "openssh"`)
	}
	if s.Spec.Hostname == "" {
		return trace.BadParameter(`hostname must be set when server SubKind is "openssh"`)
	}

	_, _, err := net.SplitHostPort(s.Spec.Addr)
	if err != nil {
		return trace.BadParameter("invalid Addr %q: %v", s.Spec.Addr, err)
	}
	return nil
}

// openSSHEphemeralKeyNodeCheckAndSetDefaults are validations for SubKindOpenSSHEphemeralKeyNode.
func (s *ServerV2) openSSHEphemeralKeyNodeCheckAndSetDefaults() error {
	if err := s.openSSHNodeCheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Currently, only AWS EC2 is supported for EphemeralKey mode.
	if s.Spec.CloudMetadata == nil || s.Spec.CloudMetadata.AWS == nil {
		return trace.BadParameter("AWS CloudMetadata is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.AccountID == "" {
		return trace.BadParameter("AWS Account ID is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.Region == "" {
		return trace.BadParameter("AWS Region is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.Integration == "" {
		return trace.BadParameter("AWS OIDC Integration is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.InstanceID == "" {
		return trace.BadParameter("AWS InstanceID is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.AvailabilityZone == "" {
		return trace.BadParameter("AWS Availability Zone is required for %q SubKind", s.SubKind)
	}
	if s.Spec.CloudMetadata.AWS.VPCID == "" {
		return trace.BadParameter("AWS VPC ID is required for %q SubKind", s.SubKind)
	}

	return nil
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (s *ServerV2) CheckAndSetDefaults() error {
	// TODO(awly): default s.Metadata.Expiry if not set (use
	// defaults.ServerAnnounceTTL).
	s.setStaticFields()

	// if the server is a registered OpenSSH node, allow the name to be
	// randomly generated
	if s.Metadata.Name == "" && s.IsOpenSSHNode() {
		s.Metadata.Name = uuid.New().String()
	}

	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if s.Kind == "" {
		return trace.BadParameter("server Kind is empty")
	}
	if s.Kind != KindNode && s.SubKind != "" {
		return trace.BadParameter(`server SubKind must only be set when Kind is "node"`)
	}

	switch s.SubKind {
	case "", SubKindTeleportNode:
		// allow but do nothing
	case SubKindOpenSSHNode:
		if err := s.openSSHNodeCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

	case SubKindOpenSSHEphemeralKeyNode:
		if err := s.openSSHEphemeralKeyNodeCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter("invalid SubKind %q", s.SubKind)
	}

	for key := range s.Spec.CmdLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	return nil
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *ServerV2) MatchSearch(values []string) bool {
	var fieldVals []string
	var custom func(val string) bool

	if s.GetKind() == KindNode {
		fieldVals = append(utils.MapToStrings(s.GetAllLabels()), s.GetName(), s.GetHostname(), s.GetAddr())
		fieldVals = append(fieldVals, s.GetPublicAddrs()...)

		if s.GetUseTunnel() {
			custom = func(val string) bool {
				return strings.EqualFold(val, "tunnel")
			}
		}
	}

	return MatchSearch(fieldVals, values, custom)
}

// DeepCopy creates a clone of this server value
func (s *ServerV2) DeepCopy() Server {
	return utils.CloneProtoMsg(s)
}

// GetCloudMetadata gets the cloud metadata for the server.
func (s *ServerV2) GetCloudMetadata() *CloudMetadata {
	return s.Spec.CloudMetadata
}

// SetCloudMetadata sets the server's cloud metadata.
func (s *ServerV2) SetCloudMetadata(meta *CloudMetadata) {
	s.Spec.CloudMetadata = meta
}

// IsAWSConsole returns true if this app is AWS management console.
func (a *App) IsAWSConsole() bool {
	return strings.HasPrefix(a.URI, constants.AWSConsoleURL)
}

// GetAWSAccountID returns value of label containing AWS account ID on this app.
func (a *App) GetAWSAccountID() string {
	return a.StaticLabels[constants.AWSAccountIDLabel]
}

// CommandLabel is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabel interface {
	// GetPeriod returns label period
	GetPeriod() time.Duration
	// SetPeriod sets label period
	SetPeriod(time.Duration)
	// GetResult returns label result
	GetResult() string
	// SetResult sets label result
	SetResult(string)
	// GetCommand returns to execute and set as a label result
	GetCommand() []string
	// Clone returns label copy
	Clone() CommandLabel
}

// Clone returns non-shallow copy of the label
func (c *CommandLabelV2) Clone() CommandLabel {
	command := make([]string, len(c.Command))
	copy(command, c.Command)
	return &CommandLabelV2{
		Command: command,
		Period:  c.Period,
		Result:  c.Result,
	}
}

// SetResult sets label result
func (c *CommandLabelV2) SetResult(r string) {
	c.Result = r
}

// SetPeriod sets label period
func (c *CommandLabelV2) SetPeriod(p time.Duration) {
	c.Period = Duration(p)
}

// GetPeriod returns label period
func (c *CommandLabelV2) GetPeriod() time.Duration {
	return c.Period.Duration()
}

// GetResult returns label result
func (c *CommandLabelV2) GetResult() string {
	return c.Result
}

// GetCommand returns to execute and set as a label result
func (c *CommandLabelV2) GetCommand() []string {
	return c.Command
}

// V2ToLabels converts concrete type to command label interface.
func V2ToLabels(l map[string]CommandLabelV2) map[string]CommandLabel {
	out := make(map[string]CommandLabel, len(l))
	for key := range l {
		val := l[key]
		out[key] = &val
	}
	return out
}

// LabelsToV2 converts labels from interface to V2 spec
func LabelsToV2(labels map[string]CommandLabel) map[string]CommandLabelV2 {
	out := make(map[string]CommandLabelV2, len(labels))
	for key, val := range labels {
		out[key] = CommandLabelV2{
			Period:  NewDuration(val.GetPeriod()),
			Result:  val.GetResult(),
			Command: val.GetCommand(),
		}
	}
	return out
}

// Servers represents a list of servers.
type Servers []Server

// Len returns the slice length.
func (s Servers) Len() int { return len(s) }

// Less compares servers by name.
func (s Servers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two servers.
func (s Servers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// SortByCustom custom sorts by given sort criteria.
func (s Servers) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetName(), s[j].GetName(), isDesc)
		})
	case ResourceSpecHostname:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetHostname(), s[j].GetHostname(), isDesc)
		})
	case ResourceSpecAddr:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetAddr(), s[j].GetAddr(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindNode)
	}

	return nil
}

// AsResources returns as type resources with labels.
func (s Servers) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, 0, len(s))
	for _, server := range s {
		resources = append(resources, ResourceWithLabels(server))
	}
	return resources
}

// GetFieldVals returns list of select field values.
func (s Servers) GetFieldVals(field string) ([]string, error) {
	vals := make([]string, 0, len(s))
	switch field {
	case ResourceMetadataName:
		for _, server := range s {
			vals = append(vals, server.GetName())
		}
	case ResourceSpecHostname:
		for _, server := range s {
			vals = append(vals, server.GetHostname())
		}
	case ResourceSpecAddr:
		for _, server := range s {
			vals = append(vals, server.GetAddr())
		}
	default:
		return nil, trace.NotImplemented("getting field %q for resource %q is not supported", field, KindNode)
	}

	return vals, nil
}
