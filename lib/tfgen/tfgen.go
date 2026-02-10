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

package tfgen

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen/internal"
)

// Resource for which Terraform configuration can be generated. It's a subset of
// the common methods between types.Resource and types.Resource153.
type Resource interface {
	GetKind() string
	GetSubKind() string
	GetVersion() string
}

// Generate Terraform configuration for the given resource protobuf message, so
// that it can be used in IaC-first wizards. You can pass opts to customize the
// output (e.g to add field comments, or change the Terraform resource type).
//
// This method broadly tries to match the behavior of our protoc-gen-terraform
// plugin.
//
// If the resource uses the modern protobuf package (google.golang.org/protobuf)
// we will use the protoreflect method to discover its fields. Field names will
// be taken from the field's TextName.
//
// If the resource uses the legacy protobuf package and github.com/gogo/protobuf
// extensions, we will use Go's reflect package to discover its fields. Field
// names will be taken from the struct member's JSON tag.
//
// Note: This method may be incomplete! Especially for legacy structs that use
// custom field types. If you find a resource for which the generated Terraform
// isn't valid, please reach out to @boxofrad.
func Generate(resource Resource, opts ...GenerateOpt) ([]byte, error) {
	o := newGenerateOpts()
	for _, fn := range opts {
		fn(o)
	}
	file := hclwrite.NewEmptyFile()
	if err := generateResource(resource, file, o); err != nil {
		return nil, trace.Wrap(err)
	}
	return hclwrite.Format(file.Bytes()), nil
}

func generateResource(
	resource Resource,
	file *hclwrite.File,
	opts *generateOpts,
) error {
	resourceType := opts.resourceType
	if resourceType == "" {
		resourceType = fmt.Sprintf("teleport_%s", resource.GetKind())
	}

	resourceName := opts.resourceName
	if resourceName == "" {
		if v, ok := resource.(interface {
			GetMetadata() *headerv1.Metadata
		}); ok {
			resourceName = v.GetMetadata().GetName()
		}
		if v, ok := resource.(interface {
			GetMetadata() types.Metadata
		}); ok {
			resourceName = v.GetMetadata().Name
		}
	}

	resourceBlock := file.Body().AppendNewBlock(
		"resource",
		[]string{resourceType, resourceName},
	)

	// Top level fields: version and sub_kind.
	if v := resource.GetVersion(); v != "" {
		resourceBlock.Body().SetAttributeValue("version", cty.StringVal(v))
	}
	if v := resource.GetSubKind(); v != "" {
		resourceBlock.Body().SetAttributeValue("sub_kind", cty.StringVal(v))
	}
	resourceBlock.Body().AppendNewline()

	msg, err := reflectMessage(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	// Metadata object.
	if v := msg.AttributeNamed("metadata"); v != nil {
		tokens := messageToTokens(fieldPath{"metadata"}, v.Value, opts)
		if tokens != nil {
			resourceBlock.Body().SetAttributeRaw("metadata", tokens)
		}
	}
	resourceBlock.Body().AppendNewline()

	// Spec object.
	if v := msg.AttributeNamed("spec"); v != nil {
		tokens := messageToTokens(fieldPath{"spec"}, v.Value, opts)
		if tokens != nil {
			resourceBlock.Body().SetAttributeRaw("spec", tokens)
		}
	}
	return nil
}

func reflectMessage(msg any) (*internal.Message, error) {
	switch m := msg.(type) {
	case proto.Message:
		return internal.ReflectModern(m)
	case internal.LegacyProtoMessage:
		return internal.ReflectLegacy(m)
	default:
		return nil, trace.BadParameter("resource must be a protobuf message, was a %T", msg)
	}
}

// messageToTokens converts a message/struct to a collection of HCL tokens.
//
// We operate directly on tokens here, rather than converting the message to a
// cty object and allowing hclwrite to generate the tokens, because we want to
// be able to inject comments above fields.
func messageToTokens(
	path fieldPath,
	val *internal.Value,
	opts *generateOpts,
) hclwrite.Tokens {
	var tokens hclwrite.Tokens
	for _, attr := range val.Message().Attributes {
		fieldPath := append(path, attr.Name)
		comment, hasComment := opts.fieldComments[fieldPath.String()]

		valueTokens := valueToTokens(
			fieldPath,
			attr.Value,
			opts,
			hasComment, /* emitZeroValue - render the zero value if there's comment on the field */
		)

		if valueTokens == nil {
			continue
		}

		if hasComment {
			tokens = append(tokens, commentToTokens(comment)...)
		}
		tokens = append(tokens, assignmentToTokens(attr.Name, valueTokens)...)
		tokens = append(tokens, newlineToken())
	}

	if len(tokens) == 0 {
		return nil
	}

	return braceTokens(tokens)
}

// commentToTokens converts a comment string to a set of HCL tokens.
func commentToTokens(comment string) hclwrite.Tokens {
	return hclwrite.Tokens{
		&hclwrite.Token{
			Type:  hclsyntax.TokenComment,
			Bytes: append([]byte(`# `), []byte(comment)...),
		},
		newlineToken(),
	}
}

// newlineToken represent a single newline character.
func newlineToken() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte("\n"),
	}
}

// assignmentToTokens generates tokens for a field assignment statement.
func assignmentToTokens(name string, valueTokens hclwrite.Tokens) hclwrite.Tokens {
	var tokens hclwrite.Tokens
	tokens = append(tokens, hclwrite.TokensForIdentifier(name)...)
	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenEqual,
		Bytes: []byte(` = `),
	})
	return append(tokens, valueTokens...)
}

// braceTokens wraps the given set of tokens in a set of curly braces (e.g. for
// an object body).
func braceTokens(inner hclwrite.Tokens) hclwrite.Tokens {
	return append(
		hclwrite.Tokens{
			&hclwrite.Token{
				Type:  hclsyntax.TokenOBrace,
				Bytes: []byte(`{`),
			},
			newlineToken(),
		},
		append(
			inner,
			&hclwrite.Token{
				Type:  hclsyntax.TokenCBrace,
				Bytes: []byte("}"),
			},
		)...,
	)
}

// valueToTokens converts the given value to a set of HCL tokens.
//
// If the value is a message, we will call messageToTokens so we can inject
// comments above the message fields.
func valueToTokens(
	path fieldPath,
	val *internal.Value,
	opts *generateOpts,
	emitZeroVal bool,
) hclwrite.Tokens {
	if val.Type == internal.TypeMessage {
		return messageToTokens(path, val, opts)
	}

	ctyVal := valueToCty(path, val, opts, emitZeroVal)
	if ctyVal == cty.NilVal {
		return nil
	}
	return hclwrite.TokensForValue(ctyVal)
}

// valueToCty converts a given value to cty so that we can generate HCL tokens
// from it.
func valueToCty(
	path fieldPath,
	val *internal.Value,
	opts *generateOpts,
	emitZeroVal bool,
) cty.Value {
	// If there's a transformation on this field, apply it.
	if transform, ok := opts.fieldTransforms[path.String()]; ok {
		var err error
		if val, err = transform(val); err != nil {
			return cty.StringVal(fmt.Sprintf("<transformation failed, error: %v>", err))
		}
	}

	switch val.Type {
	case internal.TypeBool:
		boolVal := val.Bool()
		if !boolVal && !emitZeroVal {
			return cty.NilVal
		}
		return cty.BoolVal(boolVal)

	case internal.TypeString:
		stringVal := val.String()
		if stringVal == "" && !emitZeroVal {
			return cty.NilVal
		}
		return cty.StringVal(stringVal)

	case internal.TypeBytes:
		bytesVal := val.Bytes()
		if bytesVal == nil && !emitZeroVal {
			return cty.NilVal
		}
		// protoc-gen-terraform naively converts bytes into strings without
		// validating the contents is UTF-8, so we'll copy that behavior for
		// compatibility - but really we should base64 encode it instead.
		return cty.StringVal(string(bytesVal))

	case internal.TypeInt:
		intVal := val.Int()
		if intVal == 0 && !emitZeroVal {
			return cty.NilVal
		}
		// Note: we may truncate large uint64 values, but we don't use them!
		return cty.NumberIntVal(int64(intVal))

	case internal.TypeFloat:
		floatVal := val.Float()
		if floatVal == 0 && !emitZeroVal {
			return cty.NilVal
		}
		return cty.NumberFloatVal(floatVal)

	case internal.TypeList:
		listVal := val.List()
		if len(listVal.Elems) == 0 {
			if emitZeroVal {
				return cty.ListValEmpty(cty.NilType)
			}
			return cty.NilVal
		}
		elems := make([]cty.Value, 0, len(listVal.Elems))
		for idx, elem := range listVal.Elems {
			value := valueToCty(
				append(path, fmt.Sprintf("[%d]", idx)),
				elem,
				opts,
				false, /* emitZeroVal */
			)
			if value == cty.NilVal {
				continue
			}
			elems = append(elems, value)
		}
		return cty.ListVal(elems)

	case internal.TypeMap:
		mapVal := val.Map()
		if len(mapVal.Elems) == 0 {
			if emitZeroVal {
				return cty.MapValEmpty(cty.NilType)
			}
			return cty.NilVal
		}
		elems := make(map[string]cty.Value, len(mapVal.Elems))
		for k, v := range mapVal.Elems {
			key := fmt.Sprintf("%v", k)
			value := valueToCty(
				append(path, fmt.Sprintf("[%q]", key)),
				v,
				opts,
				false, /* emitZeroVal */
			)
			if value == cty.NilVal {
				continue
			}
			elems[key] = value
		}
		return cty.MapVal(elems)

	case internal.TypeDuration:
		durVal := val.Duration()
		if durVal == 0 && !emitZeroVal {
			return cty.NilVal
		}
		return cty.StringVal(durVal.String())

	case internal.TypeTimestamp:
		tsVal := val.Timestamp()
		if tsVal.IsZero() && !emitZeroVal {
			return cty.NilVal
		}
		return cty.StringVal(tsVal.Format(time.RFC3339))

	case internal.TypeMessage:
		// Note: this branch will only be reached for messages inside a list or
		// map, other messages will be handled by messageToTokens.
		messageVal := val.Message()
		attrs := make(map[string]cty.Value)
		for _, attr := range messageVal.Attributes {
			if val := valueToCty(
				append(path, attr.Name),
				attr.Value,
				opts,
				false, /* emitZeroVal */
			); val != cty.NilVal {
				attrs[attr.Name] = val
			}
		}
		return cty.ObjectVal(attrs)
	default:
		return cty.StringVal(fmt.Sprintf("<unknown value type: %s>", val.Type))
	}
}

type fieldPath []string

func (p fieldPath) String() string { return strings.Join(p, ".") }
