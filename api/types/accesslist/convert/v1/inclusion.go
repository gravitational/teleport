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

package v1

import (
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func toInclusionProto(i accesslist.Inclusion) (accesslistv1.Inclusion, bool) {
	switch i {
	case accesslist.InclusionUnspecified:
		return accesslistv1.Inclusion_INCLUSION_UNSPECIFIED, false

	case accesslist.InclusionExplicit:
		return accesslistv1.Inclusion_INCLUSION_EXPLICIT, false

	case accesslist.InclusionImplicit:
		return accesslistv1.Inclusion_INCLUSION_IMPLICIT, false

	default:
		return 0, false
	}
}

func fromInclusionProto(i accesslistv1.Inclusion) (accesslist.Inclusion, bool) {
	switch i {
	case accesslistv1.Inclusion_INCLUSION_UNSPECIFIED:
		return accesslist.InclusionUnspecified, false

	case accesslistv1.Inclusion_INCLUSION_EXPLICIT:
		return accesslist.InclusionExplicit, false

	case accesslistv1.Inclusion_INCLUSION_IMPLICIT:
		return accesslist.InclusionImplicit, false

	default:
		return 0, false
	}
}
