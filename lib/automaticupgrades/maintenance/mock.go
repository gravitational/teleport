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

package maintenance

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StaticTrigger is a fake Trigger that return a static answer. This is used
// for testing purposes and is inherently disruptive.
type StaticTrigger struct {
	name     string
	canStart bool
	err      error
}

// Name returns the StaticTrigger name.
func (m StaticTrigger) Name() string {
	return m.name
}

// CanStart returns the statically defined maintenance approval result.
func (m StaticTrigger) CanStart(_ context.Context, _ client.Object) (bool, error) {
	return m.canStart, m.err
}

// Default returns the default behavior if the trigger fails. This cannot
// happen for a StaticTrigger and is here solely to implement the Trigger
// interface.
func (m StaticTrigger) Default() bool {
	return m.canStart
}

// NewMaintenanceStaticTrigger creates a StaticTrigger
func NewMaintenanceStaticTrigger(name string, canStart bool) Trigger {
	return StaticTrigger{
		name:     name,
		canStart: canStart,
	}
}
