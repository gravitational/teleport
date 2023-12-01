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
	// you should encounter this value in practice is when un-marshaling an
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

var inclusionToText map[Inclusion]string = map[Inclusion]string{
	InclusionUnspecified: inclusionUnspecifiedText,
	InclusionExplicit:    inclusionExplicitText,
	InclusionImplicit:    inclusionImplicitText,
}

var textToInclusion map[string]Inclusion = map[string]Inclusion{
	inclusionUnspecifiedText: InclusionUnspecified,
	inclusionExplicitText:    InclusionExplicit,
	inclusionImplicitText:    InclusionImplicit,
}

// MarshalYAML implements custom YAML marshaling for the Inclusion
// type, rendering the value in YAML as a self-describing string,
// rather than a cryptic number.
func (i Inclusion) MarshalYAML() (interface{}, error) {
	if val, err := i.marshal(); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return val, nil
	}
}

// MarshalJSON implements custom JSON marshaling for the Inclusion
// type, rendering the value in JSON as a self-describing string,
// rather than a cryptic number.
func (i Inclusion) MarshalJSON() ([]byte, error) {
	if text, err := i.marshal(); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return json.Marshal(&text)
	}
}

// String implements Stringer for Inclusion values
func (i Inclusion) String() string {
	if text, err := i.marshal(); err != nil {
		return fmt.Sprintf("invalid inclusion (%d)", uint(i))
	} else {
		return text
	}
}

// marshall implements all of the marshaling behavior common to
// the top-level marshalers.
func (i Inclusion) marshal() (string, error) {
	if text, ok := inclusionToText[i]; ok {
		return text, nil
	}

	return "", trace.BadParameter("invalid inclusion value: %d", uint(i))
}

// UnmarshalYAML implements custom YAML un-marshaling for an Inclusion value,
// reading and parsing a self-describing string rather than cryptic inclusion
// code number.
func (i *Inclusion) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var text string
	if err := unmarshal(&text); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(i.unmarshal(text))
}

// UnmarshalJSON implements custom JSON un-marshaling for an Inclusion value,
// reading and parsing a self-describing string rather than cryptic inclusion
// code number.
func (i *Inclusion) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(i.unmarshal(text))
}

// unmarshal implements all of the un-marshaling operation common to both the
// JSON and YAML un-marshaler.
func (i *Inclusion) unmarshal(text string) error {
	if value, ok := textToInclusion[text]; ok {
		(*i) = value
		return nil
	}

	return trace.BadParameter("Invalid inclusion mode text %q", text)
}
