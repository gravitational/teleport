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

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
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
	// GetPeerAddr returns the peer address of the server.
	GetPeerAddr() string
	// SetPeerAddr sets the peer address of the server.
	SetPeerAddr(string)
	// ProxiedService provides common methods for a proxied service.
	ProxiedService

	// DeepCopy creates a clone of this server value
	DeepCopy() Server

	// CloneResource is used to return a clone of the Server and match the CloneAny interface
	// This is helpful when interfacing with multiple types at the same time in unified resources
	CloneResource() ResourceWithLabels

	// GetCloudMetadata gets the cloud metadata for the server.
	GetCloudMetadata() *CloudMetadata
	// GetAWSInfo returns the AWSInfo for the server.
	GetAWSInfo() *AWSInfo
	// SetCloudMetadata sets the server's cloud metadata.
	SetCloudMetadata(meta *CloudMetadata)

	// IsOpenSSHNode returns whether the connection to this Server must use OpenSSH.
	// This returns true for SubKindOpenSSHNode and SubKindOpenSSHEICENode.
	IsOpenSSHNode() bool

	// IsEICE returns whether the Node is an EICE instance.
	// Must be `openssh-ec2-ice` subkind and have the AccountID and InstanceID information (AWS Metadata or Labels).
	IsEICE() bool

	// GetAWSInstanceID returns the AWS Instance ID if this node comes from an EC2 instance.
	GetAWSInstanceID() string
	// GetAWSAccountID returns the AWS Account ID if this node comes from an EC2 instance.
	GetAWSAccountID() string

	// GetGitHub returns the GitHub server spec.
	GetGitHub() *GitHubServerMetadata
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

// NewNode is a convenience method to create an EICE Node.
func NewEICENode(spec ServerSpecV2, labels map[string]string) (Server, error) {
	server := &ServerV2{
		Kind:    KindNode,
		SubKind: SubKindOpenSSHEICENode,
		Metadata: Metadata{
			Labels: labels,
		},
		Spec: spec,
	}
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// NewGitHubServer creates a new Git server for GitHub.
func NewGitHubServer(githubSpec GitHubServerMetadata) (Server, error) {
	server := &ServerV2{
		Kind:    KindGitServer,
		SubKind: SubKindGitHub,
		Spec: ServerSpecV2{
			GitHub: &githubSpec,
		},
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

// GetRevision returns the revision
func (s *ServerV2) GetRevision() string {
	return s.Metadata.GetRevision()
}

// SetRevision sets the revision
func (s *ServerV2) SetRevision(rev string) {
	s.Metadata.SetRevision(rev)
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
	if len(dynamic) == 0 {
		return static
	}

	lmap := make(map[string]string, len(static)+len(dynamic))
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

// setStaticFields sets static resource header and metadata fields.
func (s *ServerV2) setStaticFields() {
	s.Version = V2
}

// IsOpenSSHNode returns whether the connection to this Server must use OpenSSH.
// This returns true for SubKindOpenSSHNode and SubKindOpenSSHEICENode.
func (s *ServerV2) IsOpenSSHNode() bool {
	return IsOpenSSHNodeSubKind(s.SubKind)
}

// IsOpenSSHNodeSubKind returns whether the Node SubKind is from a server which accepts connections over the
// OpenSSH daemon (instead of a Teleport Node).
func IsOpenSSHNodeSubKind(subkind string) bool {
	return subkind == SubKindOpenSSHNode || subkind == SubKindOpenSSHEICENode
}

// GetAWSAccountID returns the AWS Account ID if this node comes from an EC2 instance.
func (s *ServerV2) GetAWSAccountID() string {
	awsAccountID, _ := s.GetLabel(AWSAccountIDLabel)

	awsMetadata := s.GetAWSInfo()
	if awsMetadata != nil && awsMetadata.AccountID != "" {
		awsAccountID = awsMetadata.AccountID
	}
	return awsAccountID
}

// GetAWSInstanceID returns the AWS Instance ID if this node comes from an EC2 instance.
func (s *ServerV2) GetAWSInstanceID() string {
	awsInstanceID, _ := s.GetLabel(AWSInstanceIDLabel)

	awsMetadata := s.GetAWSInfo()
	if awsMetadata != nil && awsMetadata.InstanceID != "" {
		awsInstanceID = awsMetadata.InstanceID
	}
	return awsInstanceID
}

// IsEICE returns whether the Node is an EICE instance.
// Must be `openssh-ec2-ice` subkind and have the AccountID and InstanceID information (AWS Metadata or Labels).
func (s *ServerV2) IsEICE() bool {
	if s.SubKind != SubKindOpenSSHEICENode {
		return false
	}

	return s.GetAWSAccountID() != "" && s.GetAWSInstanceID() != ""
}

// GetGitHub returns the GitHub server spec.
func (s *ServerV2) GetGitHub() *GitHubServerMetadata {
	return s.Spec.GitHub
}

// openSSHNodeCheckAndSetDefaults are common validations for OpenSSH nodes.
// They include SubKindOpenSSHNode and SubKindOpenSSHEICENode.
func (s *ServerV2) openSSHNodeCheckAndSetDefaults() error {
	if s.Spec.Addr == "" {
		return trace.BadParameter("addr must be set when server SubKind is %q", s.GetSubKind())
	}
	if len(s.GetPublicAddrs()) != 0 {
		return trace.BadParameter("publicAddrs must not be set when server SubKind is %q", s.GetSubKind())
	}
	if s.Spec.Hostname == "" {
		return trace.BadParameter("hostname must be set when server SubKind is %q", s.GetSubKind())
	}

	_, _, err := net.SplitHostPort(s.Spec.Addr)
	if err != nil {
		return trace.BadParameter("invalid Addr %q: %v", s.Spec.Addr, err)
	}
	return nil
}

// openSSHEC2InstanceConnectEndpointNodeCheckAndSetDefaults are validations for SubKindOpenSSHEICENode.
func (s *ServerV2) openSSHEC2InstanceConnectEndpointNodeCheckAndSetDefaults() error {
	if err := s.openSSHNodeCheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// AWS fields are required for SubKindOpenSSHEICENode.
	switch {
	case s.Spec.CloudMetadata == nil || s.Spec.CloudMetadata.AWS == nil:
		return trace.BadParameter("missing AWS CloudMetadata (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.AccountID == "":
		return trace.BadParameter("missing AWS Account ID (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.Region == "":
		return trace.BadParameter("missing AWS Region (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.Integration == "":
		return trace.BadParameter("missing AWS OIDC Integration (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.InstanceID == "":
		return trace.BadParameter("missing AWS InstanceID (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.VPCID == "":
		return trace.BadParameter("missing AWS VPC ID (required for %q SubKind)", s.SubKind)

	case s.Spec.CloudMetadata.AWS.SubnetID == "":
		return trace.BadParameter("missing AWS Subnet ID (required for %q SubKind)", s.SubKind)
	}

	return nil
}

// serverNameForEICE returns the deterministic Server's name for an EICE instance.
// This name must comply with the expected format for EC2 Nodes as defined here: api/utils/aws.IsEC2NodeID
// Returns an error if AccountID or InstanceID is not present.
func serverNameForEICE(s *ServerV2) (string, error) {
	awsAccountID := s.GetAWSAccountID()
	awsInstanceID := s.GetAWSInstanceID()

	if awsAccountID != "" && awsInstanceID != "" {
		eiceNodeName := fmt.Sprintf("%s-%s", awsAccountID, awsInstanceID)
		if !aws.IsEC2NodeID(eiceNodeName) {
			return "", trace.BadParameter("invalid account %q or instance id %q", awsAccountID, awsInstanceID)
		}
		return eiceNodeName, nil
	}

	return "", trace.BadParameter("missing account id or instance id in %s node", SubKindOpenSSHEICENode)
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (s *ServerV2) CheckAndSetDefaults() error {
	// TODO(awly): default s.Metadata.Expiry if not set (use
	// defaults.ServerAnnounceTTL).
	s.setStaticFields()

	if s.Metadata.Name == "" {
		switch s.SubKind {
		case SubKindOpenSSHEICENode:
			// For EICE nodes, use a deterministic name.
			eiceNodeName, err := serverNameForEICE(s)
			if err != nil {
				return trace.Wrap(err)
			}
			s.Metadata.Name = eiceNodeName
		case SubKindOpenSSHNode:
			// if the server is a registered OpenSSH node, allow the name to be
			// randomly generated
			s.Metadata.Name = uuid.NewString()
		case SubKindGitHub:
			s.Metadata.Name = uuid.NewString()
		}
	}

	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch s.Kind {
	case "":
		return trace.BadParameter("server Kind is empty")
	case KindNode:
		if err := s.nodeCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case KindGitServer:
		if err := s.gitServerCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	default:
		if s.SubKind != "" {
			return trace.BadParameter(`server SubKind must only be set when Kind is "node" or "git_server"`)
		}
	}

	for key := range s.Spec.CmdLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}
	return nil
}

func (s *ServerV2) nodeCheckAndSetDefaults() error {
	switch s.SubKind {
	case "", SubKindTeleportNode:
		// allow but do nothing
	case SubKindOpenSSHNode:
		if err := s.openSSHNodeCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

	case SubKindOpenSSHEICENode:
		if err := s.openSSHEC2InstanceConnectEndpointNodeCheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter("invalid SubKind %q of Kind %q", s.SubKind, s.Kind)
	}
	return nil
}

func (s *ServerV2) gitServerCheckAndSetDefaults() error {
	switch s.SubKind {
	case SubKindGitHub:
		return trace.Wrap(s.githubCheckAndSetDefaults())
	default:
		return trace.BadParameter("invalid SubKind %q of Kind %q", s.SubKind, s.Kind)
	}
}

func (s *ServerV2) githubCheckAndSetDefaults() error {
	if s.Spec.GitHub == nil {
		return trace.BadParameter("github must be set for Subkind %q", s.SubKind)
	}
	if s.Spec.GitHub.Integration == "" {
		return trace.BadParameter("integration must be set for Subkind %q", s.SubKind)
	}
	if err := ValidateGitHubOrganizationName(s.Spec.GitHub.Organization); err != nil {
		return trace.Wrap(err, "invalid GitHub organization name")
	}

	// Set SSH host port for connection and "fake" hostname for routing. These
	// values are hard-coded and cannot be customized.
	s.Spec.Addr = "github.com:22"
	s.Spec.Hostname = MakeGitHubOrgServerDomain(s.Spec.GitHub.Organization)
	if s.Metadata.Labels == nil {
		s.Metadata.Labels = make(map[string]string)
	}
	s.Metadata.Labels[GitHubOrgLabel] = s.Spec.GitHub.Organization
	return nil
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *ServerV2) MatchSearch(values []string) bool {
	if s.GetKind() != KindNode {
		return false
	}

	var custom func(val string) bool
	if s.GetUseTunnel() {
		custom = func(val string) bool {
			return strings.EqualFold(val, "tunnel")
		}
	}

	fieldVals := make([]string, 0, (len(s.Metadata.Labels)*2)+(len(s.Spec.CmdLabels)*2)+len(s.Spec.PublicAddrs)+3)

	labels := CombineLabels(s.Metadata.Labels, s.Spec.CmdLabels)
	for key, value := range labels {
		fieldVals = append(fieldVals, key, value)
	}

	fieldVals = append(fieldVals, s.Metadata.Name, s.Spec.Hostname, s.Spec.Addr)
	fieldVals = append(fieldVals, s.Spec.PublicAddrs...)

	return MatchSearch(fieldVals, values, custom)
}

// DeepCopy creates a clone of this server value
func (s *ServerV2) DeepCopy() Server {
	return utils.CloneProtoMsg(s)
}

// CloneResource creates a clone of this server value
func (s *ServerV2) CloneResource() ResourceWithLabels {
	return s.DeepCopy()
}

// GetCloudMetadata gets the cloud metadata for the server.
func (s *ServerV2) GetCloudMetadata() *CloudMetadata {
	return s.Spec.CloudMetadata
}

// GetAWSInfo gets the AWS Cloud metadata for the server.
func (s *ServerV2) GetAWSInfo() *AWSInfo {
	if s.Spec.CloudMetadata == nil {
		return nil
	}

	return s.Spec.CloudMetadata.AWS
}

// SetCloudMetadata sets the server's cloud metadata.
func (s *ServerV2) SetCloudMetadata(meta *CloudMetadata) {
	s.Spec.CloudMetadata = meta
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

// MakeGitHubOrgServerDomain creates a special domain name used in server's
// host address to identify the GitHub organization.
func MakeGitHubOrgServerDomain(org string) string {
	return fmt.Sprintf("%s.%s", org, GitHubOrgServerDomain)
}

// GetGitHubOrgFromNodeAddr parses the organization from the node address.
func GetGitHubOrgFromNodeAddr(addr string) (string, bool) {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if strings.HasSuffix(addr, "."+GitHubOrgServerDomain) {
		return strings.TrimSuffix(addr, "."+GitHubOrgServerDomain), true
	}
	return "", false
}

// GetOrganizationURL returns the URL to the GitHub organization.
func (m *GitHubServerMetadata) GetOrganizationURL() string {
	if m == nil {
		return ""
	}
	// Public github.com for now.
	return fmt.Sprintf("%s/%s", GithubURL, m.Organization)
}
