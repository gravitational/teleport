/*
Copyright 2018 Gravitational, Inc.

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

package legacy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string `json:"name"`
	// Namespace is object namespace. The field should be called "namespace"
	// when it returns in Teleport 2.4.
	Namespace string `json:"-"`
	// Description is object description
	Description string `json:"description,omitempty"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
	// Expires is a global expiry time header can be set on any resource in the system.
	Expires *time.Time `json:"expires,omitempty"`
}

// NewDuration returns Duration struct based on time.Duration
func NewDuration(d time.Duration) Duration {
	return Duration{Duration: d}
}

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration struct {
	time.Duration
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%v", d.Duration))
}

// Value returns time.Duration value of this wrapper
func (d Duration) Value() time.Duration {
	return d.Duration
}

// UnmarshalJSON marshals Duration to string
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == teleport.DurationNever {
		d.Duration = 0
	} else {
		out, err := time.ParseDuration(stringVar)
		if err != nil {
			return trace.BadParameter(err.Error())
		}
		d.Duration = out
	}
	return nil
}

// MarshalYAML marshals duration into YAML value,
// encodes it as a string in format "1m"
func (d Duration) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%v", d.Duration), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == teleport.DurationNever {
		d.Duration = 0
	} else {
		out, err := time.ParseDuration(stringVar)
		if err != nil {
			return trace.BadParameter(err.Error())
		}
		d.Duration = out
	}
	return nil
}
