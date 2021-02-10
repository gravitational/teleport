/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// DefaultClusterConfig is used as the default cluster configuration when
// one is not specified (record at node).
func DefaultClusterConfig() ClusterConfig {
	return &ClusterConfigV3{
		Kind:    KindClusterConfig,
		Version: V3,
		Metadata: Metadata{
			Name:      MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: ClusterConfigSpecV3{
			SessionRecording:    RecordAtNode,
			ProxyChecksHostKeys: HostKeyCheckYes,
			KeepAliveInterval:   NewDuration(defaults.KeepAliveInterval),
			KeepAliveCountMax:   int64(defaults.KeepAliveCountMax),
			LocalAuth:           NewBool(true),
		},
	}
}

// AuditConfigFromObject returns audit config from interface object
func AuditConfigFromObject(in interface{}) (*AuditConfig, error) {
	var cfg AuditConfig
	if in == nil {
		return &cfg, nil
	}
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// IsRecordAtProxy returns true if recording is sync or async at proxy
func IsRecordAtProxy(mode string) bool {
	return mode == RecordAtProxy || mode == RecordAtProxySync
}

// IsRecordSync returns true if recording is sync or async for proxy or node
func IsRecordSync(mode string) bool {
	return mode == RecordAtProxySync || mode == RecordAtNodeSync
}

// ShouldUploadSessions returns whether audit config
// instructs server to upload sessions
func ShouldUploadSessions(a AuditConfig) bool {
	return a.AuditSessionsURI != ""
}
