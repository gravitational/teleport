/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package controller

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// Teleport container name in the `teleport-kube-agent` Helm chart
	teleportContainerName = "teleport"
	defaultRequeue        = 30 * time.Minute
	reconciliationTimeout = 2 * time.Minute
	kubeClientTimeout     = 1 * time.Minute
	// skipReconciliationAnnotation is inspired by the tenant-operator one
	// (from the Teleport Cloud) but namespaced under `teleport.dev`
	skipReconciliationAnnotation = "teleport.dev/skipreconcile"
)

var (
	requeueLater = ctrl.Result{
		Requeue:      true,
		RequeueAfter: defaultRequeue,
	}
	requeueNow = ctrl.Result{
		Requeue:      true,
		RequeueAfter: 0,
	}
)
