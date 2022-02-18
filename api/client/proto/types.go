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

// Package proto provides the protobuf API specification for Teleport.
package proto

import (
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
)

// Duration is a wrapper around duration
type Duration time.Duration

// Get returns time.Duration value
func (d Duration) Get() time.Duration {
	return time.Duration(d)
}

// Set sets time.Duration value
func (d *Duration) Set(value time.Duration) {
	*d = Duration(value)
}

// FromWatchKind converts the watch kind value between internal
// and the protobuf format
func FromWatchKind(wk types.WatchKind) WatchKind {
	return WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}

// ToWatchKind converts the watch kind value between the protobuf
// and the internal format
func ToWatchKind(wk WatchKind) types.WatchKind {
	return types.WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}

// CheckAndSetDefaults checks and sets default values
func (req *HostCertsRequest) CheckAndSetDefaults() error {
	if req.HostID == "" {
		return trace.BadParameter("missing parameter HostID")
	}

	return req.Role.Check()
}

// CheckAndSetDefaults checks and sets default values.
func (req *ListResourcesRequest) CheckAndSetDefaults() error {
	if req.Namespace == "" {
		req.Namespace = apidefaults.Namespace
	}

	if req.Limit <= 0 {
		return trace.BadParameter("nonpositive parameter limit")
	}

	return nil
}
