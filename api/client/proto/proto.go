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

package proto

import "github.com/gravitational/teleport/lib/services"

// FromWatchKind converts the watch kind value between internal
// and the protobuf format
func FromWatchKind(wk services.WatchKind) WatchKind {
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
func ToWatchKind(wk WatchKind) services.WatchKind {
	return services.WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}
