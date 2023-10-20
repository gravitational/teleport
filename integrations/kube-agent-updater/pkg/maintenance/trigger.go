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
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Trigger is evaluated to decide whether a maintenance can happen or not.
// Maintenances can happen because of multiple reasons like:
// - attempt to recover from a broken state
// - we are in a maintenance window
// - emergency security patch
// Each Trigger has a Name() for logging purposes and a Default() method
// returning whether the trigger should allow the maintenance or not in case\
// of error.
type Trigger interface {
	Name() string
	CanStart(ctx context.Context, object client.Object) (bool, error)
	Default() bool
}

// Triggers is a list of Trigger. Triggers are OR-ed: any trigger firing in the
// list will cause the maintenance to be triggered.
type Triggers []Trigger

// CanStart checks if the maintenance can be started. It will return true if at
// least a Trigger approves the maintenance.
func (t Triggers) CanStart(ctx context.Context, object client.Object) bool {
	log := ctrllog.FromContext(ctx).V(1)
	for _, trigger := range t {
		start, err := trigger.CanStart(ctx, object)
		if err != nil {
			start = trigger.Default()
			log.Error(err, "trigger failed to evaluate, using its default value", "trigger", trigger.Name(), "defaultValue", start)
		} else {
			log.Info("trigger evaluated", "trigger", trigger.Name(), "result", start)
		}
		if start {
			log.Info("maintenance triggered", "trigger", trigger.Name())
			return true
		}
	}
	return false
}
