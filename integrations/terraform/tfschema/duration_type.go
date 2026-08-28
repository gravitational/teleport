/*
Copyright 2015-2022 Gravitational, Inc.

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

package tfschema

import (
	"context"
	fmt "fmt"
	time "time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	tftypes "github.com/hashicorp/terraform-plugin-go/tftypes"
)

// DurationType represents time.Time Terraform type which is stored in RFC3339 format, nanoseconds truncated
type DurationType struct {
	attr.Type
}

// DurationValue represents Terraform value of type DurationType
type DurationValue struct {
	// Unknown will be true if the value is not yet known.
	Unknown bool
	// Null will be true if the value was not set, or was explicitly set to null.
	Null bool
	// Value contains the set value, as long as Unknown and Null are both false.
	Value time.Duration
	// rawValue represents the string original string value of a Value
	rawValue string
}

// ApplyTerraform5AttributePathStep is not implemented for DurationType
func (t DurationType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return nil, fmt.Errorf("cannot apply AttributePathStep %T to %s", step, t.String())
}

// String returns string representation of DurationType
func (t DurationType) String() string {
	return "DurationType"
}

// Equal returns type equality
func (t DurationType) Equal(o attr.Type) bool {
	other, ok := o.(DurationType)
	if !ok {
		return false
	}
	return t == other
}

// TerraformType returns type which is used in Terraform status (time is stored as string)
func (t DurationType) TerraformType(_ context.Context) tftypes.Type {
	return tftypes.String
}

// ValueFromTerraform decodes terraform value and returns it as DurationType
func (t DurationType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if !in.IsKnown() {
		return DurationValue{Unknown: true}, nil
	}
	if in.IsNull() {
		return DurationValue{Null: true}, nil
	}

	var raw string
	err := in.As(&raw)
	if err != nil {
		return nil, err
	}

	// Error is deliberately silenced here. If a value is corrupted, this would be caught in Validate() method which
	// for some reason is called after ValueFromTerraform().
	current, err := time.ParseDuration(raw)
	if err != nil {
		return nil, err
	}

	return DurationValue{Value: current, rawValue: raw}, nil
}

// Type returns value type
func (t DurationValue) Type(_ context.Context) attr.Type {
	return DurationType{}
}

// ToTerraformValue returns the data contained in the *String as a string. If
// Unknown is true, it returns a tftypes.UnknownValue. If Null is true, it
// returns nil.
func (t DurationValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	if t.Null {
		return tftypes.NewValue(tftypes.String, nil), nil
	}
	if t.Unknown {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
	}

	value := t.Value.String()

	// NOTE:
	// Non-empty rawValue means that we have just parsed it and need to return back the same string representation.
	// Otherwise, if the value was written as "2s" in the config, t.Value.String() would generate "0h2s"
	// and it, in turn, would lead to state drift.
	// String value must stay the same until value gets explicitly changed.
	// This possibly would be fixed in future Terraform framework releases because.
	// It acts as in-step cache.
	// The reason behind this is possibly because plan diff calculator does not use Eual()
	if t.rawValue != "" {
		currentRawValue, err := time.ParseDuration(t.rawValue)
		if err != nil {
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), trace.Wrap(err)
		}

		if currentRawValue == t.Value {
			value = t.rawValue
		}
	}

	return tftypes.NewValue(tftypes.String, value), nil
}

// Equal returns true if `other` if durations are equal
func (t DurationValue) Equal(other attr.Value) bool {
	o, ok := other.(DurationValue)
	if !ok {
		return false
	}
	if t.Unknown != o.Unknown {
		return false
	}
	if t.Null != o.Null {
		return false
	}
	return t.Value == o.Value
}

// IsNull returns true if receiver is null
func (t DurationValue) IsNull() bool {
	return t.Null
}

// IsUnknown returns true if receiver is unknown
func (t DurationValue) IsUnknown() bool {
	return t.Unknown
}

// String returns the string representation of the receiver
func (t DurationValue) String() string {
	if t.Unknown {
		return attr.UnknownValueString
	}

	if t.Null {
		return attr.NullValueString
	}

	return t.Value.String()
}
