/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package controllers

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

// Reconciler extends the reconcile.Reconciler interface by adding a
// SetupWithManager function that creates a controller in the given manager.
// Every reconciler from the reconcilers package must implement this interface.
type Reconciler interface {
	reconcile.Reconciler
	SetupWithManager(mgr manager.Manager) error
	GVK() schema.GroupVersionKind
	TeleportKind() string
	Scoped() bool
	CheckFeatures(features *proto.Features) bool
}

type CheckFeaturesFunc func(*proto.Features) bool

func AlwaysEnabled(_ *proto.Features) bool {
	return true
}

func RequireEnterprise(features *proto.Features) bool {
	// In previous licenses and features we had no enterprise flags so we used the advanced workflow flag.
	// We keep doing this for backward compatibility.
	return features.GetAdvancedAccessWorkflows()
}

func RequirePolicy(features *proto.Features) bool {
	return modules.GetProtoEntitlement(features, entitlements.Policy).Enabled
}
