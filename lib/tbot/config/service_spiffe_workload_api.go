/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package config

import (
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
)

const SPIFFEWorkloadAPIServiceType = "spiffe-workload-api"

// SVIDRequestWithRules is the configuration for a single SVID along with the
// workload attestation rules that must be passed by a workload for this SVID
// to be issued to it.
type SVIDRequestWithRules struct {
	SVIDRequest `yaml:",inline"`
	// Rules is a list of workload attestation rules. At least one rule must be
	// satisfied for the SVID to be issued to a workload.
	//
	// If no rules are specified, the SVID will be issued to all workloads that
	// connect to this listener.
	Rules []SVIDRequestRule `yaml:"rules,omitempty"`
}

// SVIDRequestRuleUnix is a workload attestation ruleset for workloads that
// connect via Unix domain sockets.
type SVIDRequestRuleUnix struct {
	// PID is the process ID that a process must have to be issued this SVID.
	//
	// If unspecified, the process ID is not checked.
	PID *int `yaml:"pid,omitempty"`
	// UID is the user ID that a process must have to be issued this SVID.
	//
	// If unspecified, the user ID is not checked.
	UID *int `yaml:"uid,omitempty"`
	// GID is the primary group ID that a process must have to be issued this
	// SVID.
	//
	// If unspecified, the primary group ID is not checked.
	GID *int `yaml:"gid,omitempty"`
}

// SVIDRequestRuleKubernetes is a workload attestation ruleset for workloads
// that connect via Unix domain sockets and are running in a Kubernetes pod.
//
// Requires the "kubernetes" attestor to be enabled.
//
// Fields should be a subset of workloadattest.KubernetesAttestation.
type SVIDRequestRuleKubernetes struct {
	// Namespace is the Kubernetes namespace that a workload must be running in
	// to be issued this SVID.
	// If unspecified, the namespace is not checked.
	Namespace string `yaml:"namespace,omitempty"`
	// ServiceAccount is the Kubernetes service account that a workload must be
	// running as to be issued this SVID.
	// If unspecified, the service account is not checked.
	ServiceAccount string `yaml:"service_account,omitempty"`
	// PodName is the Kubernetes pod name that a workload must be running in to
	// be issued this SVID.
	// If unspecified, the pod name is not checked.
	PodName string `yaml:"pod_name,omitempty"`
}

// SVIDRequestRule is an individual workload attestation rule. All values
// specified within the rule must be satisfied for the rule itself to pass.
type SVIDRequestRule struct {
	// Unix is the workload attestation ruleset for workloads that connect via
	// Unix domain sockets. If any value here is set, the rule will not pass
	// unless the workload is connecting via a Unix domain socket.
	Unix SVIDRequestRuleUnix `yaml:"unix"`
	// Kubernetes is the workload attestation ruleset for workloads that connect
	// via the Unix domain socket and are running in a Kubernetes pod.
	// The "kubernetes" attestor must be enabled or these rules will fail.
	Kubernetes SVIDRequestRuleKubernetes `yaml:"kubernetes"`
}

func (o SVIDRequestRule) LogValue() slog.Value {
	var unixAttrs []any
	if o.Unix.PID != nil {
		unixAttrs = append(unixAttrs, slog.Int("pid", *o.Unix.PID))
	}
	if o.Unix.GID != nil {
		unixAttrs = append(unixAttrs, slog.Int("gid", *o.Unix.GID))
	}
	if o.Unix.UID != nil {
		unixAttrs = append(unixAttrs, slog.Int("uid", *o.Unix.UID))
	}
	return slog.GroupValue(
		slog.Group("unix", unixAttrs...),
	)
}

// SPIFFEWorkloadAPIService is the configuration for the SPIFFE Workload API
// service.
type SPIFFEWorkloadAPIService struct {
	// Listen is the address on which the SPIFFE Workload API server should
	// listen. This should either be prefixed with "unix://" or "tcp://".
	Listen string `yaml:"listen"`
	// SVIDs is the list of SVIDs that the SPIFFE Workload API server should
	// provide.
	SVIDs []SVIDRequestWithRules `yaml:"svids"`
	// Attestors is the configuration for the workload attestation process.
	Attestors workloadattest.Config `yaml:"attestors"`
	// JWTSVIDTTL specifies how long that JWT SVIDs issued by this SPIFFE
	// Workload API server are valid for. If unspecified, this falls back to
	// the globally configured default.
	JWTSVIDTTL time.Duration `yaml:"jwt_svid_ttl,omitempty"`
}

func (s *SPIFFEWorkloadAPIService) Type() string {
	return SPIFFEWorkloadAPIServiceType
}

func (s *SPIFFEWorkloadAPIService) MarshalYAML() (interface{}, error) {
	type raw SPIFFEWorkloadAPIService
	return withTypeHeader((*raw)(s), SPIFFEWorkloadAPIServiceType)
}

func (s *SPIFFEWorkloadAPIService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SPIFFEWorkloadAPIService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *SPIFFEWorkloadAPIService) CheckAndSetDefaults() error {
	if s.Listen == "" {
		return trace.BadParameter("listen: should not be empty")
	}
	if len(s.SVIDs) == 0 {
		return trace.BadParameter("svids: should not be empty")
	}
	for i, svid := range s.SVIDs {
		if err := svid.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating svid[%d]", i)
		}
	}
	if err := s.Attestors.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating attestor")
	}
	return nil
}
