// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"
)

// Inclusion values indicate how membership and ownership of an AccessList
// should be applied.
type Inclusion uint

const (
	// InclusionUnspecified is the default, un-set inclusion value used to
	// detect when inclusion is not specified in an access list. The only times
	// you should encounter this value in practice is when unmarshaling an
	// AccessList that pre-dates the implementation of dynamic access lists.
	InclusionUnspecified Inclusion = 0

	// InclusionImplicit indicates that a user need only meet a requirement set
	// to be considered included in a list. Both list membership and ownership
	// may be Implicit.
	InclusionImplicit Inclusion = 1

	// InclusionExplicit indicates that a user must meet a requirement set AND
	// be explicitly added to an access list to be included in it. Both list
	// membership and ownership may be Explicit.
	InclusionExplicit Inclusion = 2
)

const (
	inclusionUnspecifiedText string = ""
	inclusionImplicitText    string = "implicit"
	inclusionExplicitText    string = "explicit"
)

func (i Inclusion) MarshalYAML() (interface{}, error) {
	if val, err := i.marshal(); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return val, nil
	}
}

func (i Inclusion) MarshalJSON() ([]byte, error) {
	if text, err := i.marshal(); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return json.Marshal(&text)
	}
}

func (i Inclusion) String() string {
	if text, err := i.marshal(); err != nil {
		return fmt.Sprintf("invalid inclusion (%d)", uint(i))
	} else {
		return text
	}
}

func (i Inclusion) marshal() (string, error) {
	switch i {
	case InclusionUnspecified:
		return inclusionUnspecifiedText, nil

	case InclusionExplicit:
		return inclusionExplicitText, nil

	case InclusionImplicit:
		return inclusionImplicitText, nil

	default:
		return "", trace.BadParameter("invalid inclusion value: %d", uint(i))
	}
}

func (i *Inclusion) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var text string
	if err := unmarshal(&text); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(i.unmarshal(text))
}

func (i *Inclusion) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(i.unmarshal(text))
}

func (i *Inclusion) unmarshal(text string) error {
	var val Inclusion
	switch text {
	case inclusionUnspecifiedText:
		val = InclusionUnspecified

	case inclusionExplicitText:
		val = InclusionExplicit

	case inclusionImplicitText:
		val = InclusionImplicit

	default:
		return trace.BadParameter("Invalid inclusion mode text %q", text)
	}

	(*i) = val
	return nil
}
