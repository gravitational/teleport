/*
Copyright 2015-2019 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

// Server represents a Node, Proxy or Auth server in a Teleport cluster
type Server interface {
	// Resource provides common resource headers
	Resource
	// GetTeleportVersion returns the teleport version the server is running on
	GetTeleportVersion() string
	// GetAddr return server address
	GetAddr() string
	// GetHostname returns server hostname
	GetHostname() string
	// GetNamespace returns server namespace
	GetNamespace() string
	// GetAllLabels returns server's static and dynamic label values merged together
	GetAllLabels() map[string]string
	// GetLabels returns server's static label key pairs
	GetLabels() map[string]string
	// GetCmdLabels returns command labels
	GetCmdLabels() map[string]CommandLabel
	// GetPublicAddr is an optional field that returns the public address this cluster can be reached at.
	GetPublicAddr() string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// GetUseTunnel gets if a reverse tunnel should be used to connect to this node.
	GetUseTunnel() bool
	// GetKubernetesClusters gets kubernetes clusters accessible through this
	// proxy.
	GetKubernetesClusters() []string
	// SetUseTunnel sets if a reverse tunnel should be used to connect to this node.
	SetUseTunnel(bool)
	// String returns string representation of the server
	String() string
	// SetAddr sets server address
	SetAddr(addr string)
	// SetPublicAddr sets the public address this cluster can be reached at.
	SetPublicAddr(string)
	// SetNamespace sets server namespace
	SetNamespace(namespace string)
	// SetKubernetesClusters sets kubernetes clusters accessible through this
	// proxy.
	SetKubernetesClusters([]string)
	// V1 returns V1 version for backwards compatibility
	V1() *ServerV1
	// MatchAgainst takes a map of labels and returns True if this server
	// has ALL of them
	//
	// Any server matches against an empty label set
	MatchAgainst(labels map[string]string) bool
	// LabelsString returns a comma separated string with all node's labels
	LabelsString() string
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// ServersToV1 converts list of servers to slice of V1 style ones
func ServersToV1(in []Server) []ServerV1 {
	out := make([]ServerV1, len(in))
	for i := range in {
		out[i] = *(in[i].V1())
	}
	return out
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

// V2 returns version 2 of the resource, itself
func (s *ServerV2) V2() *ServerV2 {
	return s
}

// V1 returns V1 version of the resource
func (s *ServerV2) V1() *ServerV1 {
	labels := make(map[string]CommandLabelV1, len(s.Spec.CmdLabels))
	for key := range s.Spec.CmdLabels {
		val := s.Spec.CmdLabels[key]
		labels[key] = CommandLabelV1{
			Period:  val.Period.Duration(),
			Result:  val.Result,
			Command: val.Command,
		}
	}
	return &ServerV1{
		ID:        s.Metadata.Name,
		Kind:      s.Kind,
		Namespace: ProcessNamespace(s.Metadata.Namespace),
		Addr:      s.Spec.Addr,
		Hostname:  s.Spec.Hostname,
		Labels:    s.Metadata.Labels,
		CmdLabels: labels,
	}
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

// Expires returns object expiry setting
func (s *ServerV2) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (s *ServerV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	s.Metadata.SetTTL(clock, ttl)
}

// SetPublicAddr sets the public address this cluster can be reached at.
func (s *ServerV2) SetPublicAddr(addr string) {
	s.Spec.PublicAddr = addr
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

// GetPublicAddr is an optional field that returns the public address this cluster can be reached at.
func (s *ServerV2) GetPublicAddr() string {
	return s.Spec.PublicAddr
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

// GetLabels returns server's static label key pairs
func (s *ServerV2) GetLabels() map[string]string {
	return s.Metadata.Labels
}

// GetCmdLabels returns command labels
func (s *ServerV2) GetCmdLabels() map[string]CommandLabel {
	if s.Spec.CmdLabels == nil {
		return nil
	}
	out := make(map[string]CommandLabel, len(s.Spec.CmdLabels))
	for key := range s.Spec.CmdLabels {
		val := s.Spec.CmdLabels[key]
		out[key] = &val
	}
	return out
}

func (s *ServerV2) String() string {
	return fmt.Sprintf("Server(name=%v, namespace=%v, addr=%v, labels=%v)", s.Metadata.Name, s.Metadata.Namespace, s.Spec.Addr, s.Metadata.Labels)
}

// GetNamespace returns server namespace
func (s *ServerV2) GetNamespace() string {
	return ProcessNamespace(s.Metadata.Namespace)
}

// GetAllLabels returns the full key:value map of both static labels and
// "command labels"
func (s *ServerV2) GetAllLabels() map[string]string {
	lmap := make(map[string]string)
	for key, value := range s.Metadata.Labels {
		lmap[key] = value
	}
	for key, cmd := range s.Spec.CmdLabels {
		lmap[key] = cmd.Result
	}
	return lmap
}

// GetKubernetesClusters gets kubernetes clusters accessible through this
// proxy.
func (s *ServerV2) GetKubernetesClusters() []string {
	return s.Spec.KubernetesClusters
}

// SetKubernetesClusters sets kubernetes clusters accessible through this
// proxy.
func (s *ServerV2) SetKubernetesClusters(clusters []string) {
	s.Spec.KubernetesClusters = clusters
}

// MatchAgainst takes a map of labels and returns True if this server
// has ALL of them
//
// Any server matches against an empty label set
func (s *ServerV2) MatchAgainst(labels map[string]string) bool {
	if labels != nil {
		myLabels := s.GetAllLabels()
		for key, value := range labels {
			if myLabels[key] != value {
				return false
			}
		}
	}
	return true
}

// LabelsString returns a comma separated string with all node's labels
func (s *ServerV2) LabelsString() string {
	labels := []string{}
	for key, val := range s.Metadata.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range s.Spec.CmdLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val.Result))
	}
	sort.Strings(labels)
	return strings.Join(labels, ",")
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (s *ServerV2) CheckAndSetDefaults() error {
	err := s.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	for key := range s.Spec.CmdLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	if len(s.Spec.KubernetesClusters) > 0 && s.Kind != KindProxy {
		return trace.BadParameter("KubernetesClusters are only allowed on Proxy servers; got clusters %q set on a %q server %q", s.Spec.KubernetesClusters, s.Kind, s.Metadata.Name)
	}

	return nil
}

const (
	// Equal means two objects are equal
	Equal = iota
	// OnlyTimestampsDifferent is true when only timestamps are different
	OnlyTimestampsDifferent = iota
	// Different means that some fields are different
	Different = iota
)

// CompareServers returns difference between two server
// objects, Equal (0) if identical, OnlyTimestampsDifferent(1) if only timestamps differ, Different(2) otherwise
func CompareServers(a, b Server) int {
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetAddr() != b.GetAddr() {
		return Different
	}
	if a.GetHostname() != b.GetHostname() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if a.GetPublicAddr() != b.GetPublicAddr() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if a.GetUseTunnel() != b.GetUseTunnel() {
		return Different
	}
	if !utils.StringMapsEqual(a.GetLabels(), b.GetLabels()) {
		return Different
	}
	if !CmdLabelMapsEqual(a.GetCmdLabels(), b.GetCmdLabels()) {
		return Different
	}
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	if !utils.StringSlicesEqual(a.GetKubernetesClusters(), b.GetKubernetesClusters()) {
		return Different
	}
	return Equal
}

// CmdLabelMapsEqual compares two maps with command labels,
// returns true if label sets are equal
func CmdLabelMapsEqual(a, b map[string]CommandLabel) bool {
	if len(a) != len(b) {
		return false
	}
	for key, val := range a {
		val2, ok := b[key]
		if !ok {
			return false
		}
		if !val.Equals(val2) {
			return false
		}
	}
	return true
}

// ServerSpecV2Schema is JSON schema for server
const ServerSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
	"version": {"type": "string"},
    "addr": {"type": "string"},
    "public_addr": {"type": "string"},
    "hostname": {"type": "string"},
    "use_tunnel": {"type": "boolean"},
    "labels": {
      "type": "object",
      "additionalProperties": false,
      "patternProperties": {
        "^.*$":  { "type": "string" }
      }
    },
    "cmd_labels": {
      "type": "object",
      "additionalProperties": false,
      "patternProperties": {
        "^.*$": {
          "type": "object",
          "additionalProperties": false,
          "required": ["command"],
          "properties": {
            "command": {"type": "array", "items": {"type": "string"}},
            "period": {"type": "string"},
            "result": {"type": "string"}
          }
        }
      }
    },
    "rotation": %v,
    "kubernetes_clusters": {"type": "array", "items": {"type": "string"}}
  }
}`

// ServerV1 represents V1 spec of the server
type ServerV1 struct {
	Kind      string                    `json:"kind"`
	ID        string                    `json:"id"`
	Addr      string                    `json:"addr"`
	Hostname  string                    `json:"hostname"`
	Namespace string                    `json:"namespace"`
	Labels    map[string]string         `json:"labels"`
	CmdLabels map[string]CommandLabelV1 `json:"cmd_labels"`
}

// V1 returns V1 version of the resource
func (s *ServerV1) V1() *ServerV1 {
	return s
}

// V2 returns V2 version of the resource
func (s *ServerV1) V2() *ServerV2 {
	labels := make(map[string]CommandLabelV2, len(s.CmdLabels))
	for key := range s.CmdLabels {
		val := s.CmdLabels[key]
		labels[key] = CommandLabelV2{
			Period:  Duration(val.Period),
			Result:  val.Result,
			Command: val.Command,
		}
	}
	return &ServerV2{
		Kind:    s.Kind,
		Version: V2,
		Metadata: Metadata{
			Name:      s.ID,
			Namespace: ProcessNamespace(s.Namespace),
			Labels:    s.Labels,
		},
		Spec: ServerSpecV2{
			Addr:      s.Addr,
			Hostname:  s.Hostname,
			CmdLabels: labels,
		},
	}
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

// CommandLabelV2 is a label that has a value as a result of the
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
	// Equals returns true if label is equal to the other one
	// false otherwise
	Equals(CommandLabel) bool
}

// Equals returns true if labels are equal, false otherwise
func (c *CommandLabelV2) Equals(other CommandLabel) bool {
	if c.GetPeriod() != other.GetPeriod() {
		return false
	}
	if c.GetResult() != other.GetResult() {
		return false
	}
	if !utils.StringSlicesEqual(c.GetCommand(), other.GetCommand()) {
		return false
	}
	return true
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

// CommandLabelV1 is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabelV1 struct {
	// Period is a time between command runs
	Period time.Duration `json:"period"`
	// Command is a command to run
	Command []string `json:"command"` //["/usr/bin/hostname", "--long"]
	// Result captures standard output
	Result string `json:"result"`
}

// CommandLabels is a set of command labels
type CommandLabels map[string]CommandLabel

// Clone returns copy of the set
func (c *CommandLabels) Clone() CommandLabels {
	out := make(CommandLabels, len(*c))
	for name, label := range *c {
		out[name] = label.Clone()
	}
	return out
}

// SetEnv sets the value of the label from environment variable
func (c *CommandLabels) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), c); err != nil {
		return trace.Wrap(err, "can not parse Command Labels")
	}
	return nil
}

// GetServerSchema returns role schema with optionally injected
// schema for extensions
func GetServerSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(ServerSpecV2Schema, RotationSchema), DefaultDefinitions)
}

// UnmarshalServerResource unmarshals role from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalServerResource(data []byte, kind string, cfg *MarshalConfig) (Server, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing server data")
	}

	var h ResourceHeader
	err := utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case "":
		var s ServerV1
		err := utils.FastUnmarshal(data, &s)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		s.Kind = kind
		v2 := s.V2()
		if cfg.ID != 0 {
			v2.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			v2.SetExpiry(cfg.Expires)
		}
		return v2, nil
	case V2:
		var s ServerV2

		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &s); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetServerSchema(), &s, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
		s.Kind = kind
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("server resource version %q is not supported", h.Version)
}

var serverMarshaler ServerMarshaler = &TeleportServerMarshaler{}

func SetServerMarshaler(m ServerMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	serverMarshaler = m
}

func GetServerMarshaler() ServerMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return serverMarshaler
}

// ServerMarshaler implements marshal/unmarshal of Role implementations
// mostly adds support for extended versions
type ServerMarshaler interface {
	// UnmarshalServer from binary representation.
	UnmarshalServer(bytes []byte, kind string, opts ...MarshalOption) (Server, error)

	// MarshalServer to binary representation.
	MarshalServer(Server, ...MarshalOption) ([]byte, error)

	// UnmarshalServers is used to unmarshal multiple servers from their
	// binary representation.
	UnmarshalServers(bytes []byte) ([]Server, error)

	// MarshalServers is used to marshal multiple servers to their binary
	// representation.
	MarshalServers([]Server) ([]byte, error)
}

type TeleportServerMarshaler struct{}

// UnmarshalServer unmarshals server from JSON
func (*TeleportServerMarshaler) UnmarshalServer(bytes []byte, kind string, opts ...MarshalOption) (Server, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return UnmarshalServerResource(bytes, kind, cfg)
}

// MarshalServer marshals server into JSON.
func (*TeleportServerMarshaler) MarshalServer(s Server, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type serverv1 interface {
		V1() *ServerV1
	}
	type serverv2 interface {
		V2() *ServerV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := s.(serverv1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return utils.FastMarshal(v.V1())
	case V2:
		v, ok := s.(serverv2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		v2 := v.V2()
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *v2
			copy.SetResourceID(0)
			v2 = &copy
		}
		return utils.FastMarshal(v2)
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}

// UnmarshalServers is used to unmarshal multiple servers from their
// binary representation.
func (*TeleportServerMarshaler) UnmarshalServers(bytes []byte) ([]Server, error) {
	var servers []ServerV2

	err := utils.FastUnmarshal(bytes, &servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]Server, len(servers))
	for i, v := range servers {
		out[i] = Server(&v)
	}
	return out, nil
}

// MarshalServers is used to marshal multiple servers to their binary
// representation.
func (*TeleportServerMarshaler) MarshalServers(s []Server) ([]byte, error) {
	bytes, err := utils.FastMarshal(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bytes, nil
}

// SortedServers is a sort wrapper that sorts servers by name
type SortedServers []Server

func (s SortedServers) Len() int {
	return len(s)
}

func (s SortedServers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

func (s SortedServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// SortedReverseTunnels sorts reverse tunnels by cluster name
type SortedReverseTunnels []ReverseTunnel

func (s SortedReverseTunnels) Len() int {
	return len(s)
}

func (s SortedReverseTunnels) Less(i, j int) bool {
	return s[i].GetClusterName() < s[j].GetClusterName()
}

func (s SortedReverseTunnels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// GuessProxyHostAndVersion tries to find the first proxy with a public
// address configured and return that public addr and version.
// If no proxies are configured, it will return a guessed value by concatenating
// the first proxy's hostname with default port number, and the first proxy's
// version will also be returned.
//
// Returns empty value if there are no proxies.
func GuessProxyHostAndVersion(proxies []Server) (string, string, error) {
	if len(proxies) == 0 {
		return "", "", trace.NotFound("list of proxies empty")
	}

	// Find the first proxy with a public address set and return it.
	for _, proxy := range proxies {
		proxyHost := proxy.GetPublicAddr()
		if proxyHost != "" {
			return proxyHost, proxy.GetTeleportVersion(), nil
		}
	}

	// No proxies have a public address set, return guessed value.
	guessProxyHost := fmt.Sprintf("%v:%v", proxies[0].GetHostname(), defaults.HTTPListenPort)
	return guessProxyHost, proxies[0].GetTeleportVersion(), nil
}
