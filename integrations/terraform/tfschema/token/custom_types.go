package token

import (
	"context"
	"fmt"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GenSchemaLabels returns Terraform schema for Labels type
func GenSchemaLabels(ctx context.Context) tfsdk.Attribute {
	return tfschema.GenSchemaLabels(ctx)
}

// GenSchemaBoolOptionsNullable returns Terraform schema for BoolOption type
func GenSchemaBoolOptionNullable(_ context.Context) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional: true,
		Type:     types.BoolType,
	}
}

func CopyFromBoolOptionNullable(diags diag.Diagnostics, tf attr.Value, o **apitypes.BoolOption) {
	v, ok := tf.(types.Bool)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Bool", tf))
		return
	}
	if !v.Null && !v.Unknown {
		value := apitypes.BoolOption{Value: v.Value}
		*o = &value
		return
	}
}

func CopyToBoolOptionNullable(diags diag.Diagnostics, o *apitypes.BoolOption, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(types.Bool)
	if !ok {
		value = types.Bool{}
	}

	if o == nil {
		value.Null = true
		return value
	}

	value.Null = false
	value.Value = o.Value

	return value
}

func CopyFromLabels(diags diag.Diagnostics, v attr.Value, o *apitypes.Labels) {
	tfschema.CopyFromLabels(diags, v, o)
}

func CopyToLabels(diags diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value) attr.Value {
	return tfschema.CopyToLabels(diags, o, t, v)
}
