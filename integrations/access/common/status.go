/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"context"

	"github.com/gravitational/teleport/api/types"

	log "github.com/sirupsen/logrus"
)

// StatusSink defines a destination for PluginStatus
type StatusSink interface {
	Emit(ctx context.Context, s types.PluginStatus) error
}

// TryEmitStatus tries to emit status (if the sink set).
// It logs an error, but does not fail if emitting the status fails.
func TryEmitStatus(ctx context.Context, sink StatusSink, s types.PluginStatus) {
	if sink == nil {
		return
	}

	if err := sink.Emit(ctx, s); err != nil {
		log.Errorf("Error while emitting plugin status: %v", err)
	}
}
