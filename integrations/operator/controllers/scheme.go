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
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
)

// Scheme is a singleton scheme for all controllers
var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(resourcesv5.AddToScheme(Scheme))
	utilruntime.Must(resourcesv3.AddToScheme(Scheme))
	utilruntime.Must(resourcesv2.AddToScheme(Scheme))
	utilruntime.Must(resourcesv1.AddToScheme(Scheme))

	// Not needed to reconcile the teleport CRs, but needed for the controller manager.
	// We are not doing something very kubernetes friendly, but it's easier to have a single
	// scheme rather than having to build and propagate schemes in multiple places, which
	// is error-prone and can lead to inconsistencies.
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(apiextv1.AddToScheme(Scheme))
}
