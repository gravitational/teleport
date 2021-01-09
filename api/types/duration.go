/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration time.Duration

// Duration returns time.Duration from Duration typex
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Value returns time.Duration value of this wrapper
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration().String())
}

// UnmarshalJSON interprets the given bytes as a Duration value
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == constants.DurationNever {
		*d = Duration(0)
		return nil
	}
	out, err := time.ParseDuration(stringVar)
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	*d = Duration(out)
	return nil
}

// MarshalYAML marshals duration into YAML value,
// encodes it as a string in format "1m"
func (d Duration) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%v", d.Duration()), nil
}

// UnmarshalYAML unmarshals duration from YAML value.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == constants.DurationNever {
		*d = Duration(0)
		return nil
	}
	out, err := time.ParseDuration(stringVar)
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	*d = Duration(out)
	return nil
}

// MaxDuration returns the maximum duration value
func MaxDuration() Duration {
	return NewDuration(1<<63 - 1)
}

// NewDuration converts the given time.Duration value to a duration
func NewDuration(d time.Duration) Duration {
	return Duration(d)
}
