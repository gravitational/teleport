/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime"

	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
)

func AddToScheme(scheme *runtime.Scheme) error {
	return trace.NewAggregate(
		resourcesv1.AddToScheme(scheme),
		resourcesv2.AddToScheme(scheme),
		resourcesv3.AddToScheme(scheme),
		resourcesv5.AddToScheme(scheme),
	)
}

func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	return scheme, trace.Wrap(err)
}
