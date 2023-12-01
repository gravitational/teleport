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
	"github.com/gravitational/teleport/api/types/accesslist"
)

const (
	inclusionUnspecifiedText string = ""
	inclusionExplicitText    string = "explicit"
	inclusionImplicitText    string = "implicit"
)

func toInclusionProto(i accesslist.Inclusion) string {
	switch i {
	case accesslist.InclusionExplicit:
		return inclusionExplicitText

	case accesslist.InclusionImplicit:
		return inclusionImplicitText

	default:
		return inclusionUnspecifiedText
	}
}

func fromInclusionProto(text string) accesslist.Inclusion {
	switch text {
	case inclusionExplicitText:
		return accesslist.InclusionExplicit

	case inclusionImplicitText:
		return accesslist.InclusionImplicit

	default:
		return accesslist.InclusionUnspecified
	}
}
