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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	diag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	tftypes "github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	// timeThreshold represents time rounding threshold (nanoseconds would be cut off)
	timeThreshold = time.Nanosecond
)

// TimeType represents time.Time Terraform type which is stored in RFC3339 format, nanoseconds truncated
type TimeType struct {
	attr.Type
	Format string
}

// Time represents Terraform value of type TimeType
type TimeValue struct {
	// Unknown will be true if the value is not yet known.
	Unknown bool
	// Null will be true if the value was not set, or was explicitly set to null
	Null bool
	// Value contains the set value, as long as Unknown and Null are both false
	Value time.Time
	// Format is time.Time format used to parse/encode
	Format string
}

// UseRFC3339Time creates TimeType for rfc3339
func UseRFC3339Time() TimeType {
	return TimeType{Format: time.RFC3339}
}

// ApplyTerraform5AttributePathStep is not implemented for TimeType
func (t TimeType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return nil, fmt.Errorf("cannot apply AttributePathStep %T to %s", step, t.String())
}

// String returns string representation of TimeType
func (t TimeType) String() string {
	return "TimeType(" + t.Format + ")"
}

// Equal returns type equality
func (t TimeType) Equal(o attr.Type) bool {
	other, ok := o.(TimeType)
	if !ok {
		return false
	}
	return t == other
}

// TerraformType returns type which is used in Terraform status (time is stored as string)
func (t TimeType) TerraformType(_ context.Context) tftypes.Type {
	return tftypes.String
}

// ValueFromTerraform decodes terraform value and returns it as TimeType
func (t TimeType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if !in.IsKnown() {
		return TimeValue{Unknown: true, Format: t.Format}, nil
	}
	if in.IsNull() {
		return TimeValue{Null: true, Format: t.Format}, nil
	}
	var raw string
	err := in.As(&raw)
	if err != nil {
		return nil, err
	}

	// Error is deliberately silenced here. If a value is corrupted, this would be caught in Validate() method which
	// for some reason is called after ValueFromTerraform().
	current, err := time.Parse(t.Format, raw)
	if err != nil {
		return nil, err
	}

	return TimeValue{Value: current, Format: t.Format}, nil
}

// Validate validates Terraform Time valud
func (t TimeType) Validate(ctx context.Context, in tftypes.Value, path path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if in.Type() == nil {
		return diags
	}

	if !in.Type().Is(tftypes.String) {
		diags.AddAttributeError(
			path,
			"Time Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. Please report the following to the provider developer:\n\n"+
				fmt.Sprintf("Expected Time value, received %T with value: %v", in, in),
		)
		return diags
	}

	if !in.IsKnown() || in.IsNull() {
		return diags
	}

	var s string
	err := in.As(&s)

	if err != nil {
		diags.AddAttributeError(
			path,
			"Time Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. Please report the following to the provider developer:\n\n"+
				fmt.Sprintf("Cannot convert value to Time: %s", err),
		)
		return diags
	}

	current, err := time.Parse(t.Format, s)
	if err != nil {
		diags.AddAttributeError(
			path,
			"Time Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. Please report the following to the provider developer:\n\n"+
				fmt.Sprintf("Cannot parse value as Time in format RFC3339: %s", err),
		)
		return diags
	}

	if current.Nanosecond() > 0 {
		diags.AddAttributeError(
			path,
			"Time Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. Please report the following to the provider developer:\n\n"+
				"Time must not contain nanoseconds",
		)
		return diags
	}

	return diags
}

// Type returns value type
func (t TimeValue) Type(_ context.Context) attr.Type {
	return TimeType{Format: t.Format}
}

// ToTerraformValue returns the data contained in the *String as a string. If
// Unknown is true, it returns a tftypes.UnknownValue. If Null is true, it
// returns nil.
func (t TimeValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	if t.Null {
		return tftypes.NewValue(tftypes.String, nil), nil
	}
	if t.Unknown {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
	}

	return tftypes.NewValue(tftypes.String, t.Value.Truncate(timeThreshold).Format(t.Format)), nil
}

// Equal returns true if `other` is a *String and has the same value as `s`.
func (t TimeValue) Equal(other attr.Value) bool {
	o, ok := other.(TimeValue)
	if !ok {
		return false
	}
	if t.Unknown != o.Unknown {
		return false
	}
	if t.Null != o.Null {
		return false
	}

	return t.Value.Equal(o.Value)
}

// IsNull returns true if receiver is null
func (t TimeValue) IsNull() bool {
	return t.Null
}

// IsUnknown returns true if receiver is unknown
func (t TimeValue) IsUnknown() bool {
	return t.Unknown
}

// String returns the string representation of the receiver
func (t TimeValue) String() string {
	if t.Unknown {
		return attr.UnknownValueString
	}

	if t.Null {
		return attr.NullValueString
	}

	return t.Value.String()
}
