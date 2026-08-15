// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"iter"
	"slices"
	"strings"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	paginationRuleID = "PAGINATION_REQUIRED"
)

var paginationSpec = &check.Spec{
	Rules: []*check.RuleSpec{
		{
			ID:      paginationRuleID,
			Purpose: "Ensure RPCs returning a repeated field use pagination fields.",
			Type:    check.RuleTypeLint,
			Handler: checkutil.NewMethodRuleHandler(checkPaginationMethod),
		},
		{
			ID:      "RESOURCE_SHAPE",
			Purpose: "Ensure messages that represent resources have the appropriate shape.",
			Type:    check.RuleTypeLint,
			Handler: checkutil.NewMessageRuleHandler(checkResourceShape, checkutil.WithoutImports()),
		},
		{
			ID:      "NO_JSON_NAME",
			Purpose: "Ensure that fields don't set the json_name option.",
			Type:    check.RuleTypeLint,
			Handler: checkutil.NewFieldRuleHandler(func(_ context.Context, rw check.ResponseWriter, _ check.Request, f protoreflect.FieldDescriptor) error {
				if jsonCamelCase(string(f.Name())) != f.JSONName() {
					rw.AddAnnotation(
						check.WithDescriptor(f),
						check.WithMessagef("the json_name option is not allowed for field %+q", f.Name()),
					)
				}
				return nil
			}, checkutil.WithoutImports()),
		},
	},
}

// google.golang.org/protobuf@v1.36.11/reflect/protodesc/proto.go
func jsonCamelCase(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var wasUnderscore bool
	for _, c := range []byte(s) { // proto identifiers are always ASCII
		if c == '_' {
			wasUnderscore = true
			continue
		}
		if wasUnderscore && 'a' <= c && c <= 'z' {
			c -= 'a' - 'A' // convert to uppercase
		}
		b.WriteByte(c)
		wasUnderscore = false
	}
	return b.String()
}

func main() {
	check.Main(paginationSpec)
}

func fields(m protoreflect.MessageDescriptor) iter.Seq[protoreflect.FieldDescriptor] {
	return fieldsFromFieldDescriptors(m.Fields())
}

func fieldsFromFieldDescriptors(fds protoreflect.FieldDescriptors) iter.Seq[protoreflect.FieldDescriptor] {
	return func(yield func(protoreflect.FieldDescriptor) bool) {
		for i := range fds.Len() {
			if !yield(fds.Get(i)) {
				return
			}
		}
	}
}

func getGogoJsonTag(m protoreflect.ProtoMessage) (string, bool) {
	unk := m.ProtoReflect().GetUnknown()
	for len(unk) > 0 {
		num, typ, n := protowire.ConsumeTag(unk)
		if n < 0 {
			panic(protowire.ParseError(n))
		}
		unk = unk[n:]
		if num != 65005 {
			n := protowire.ConsumeFieldValue(num, typ, unk)
			if n < 0 {
				panic(protowire.ParseError(n))
			}
			unk = unk[n:]
			continue
		}
		if typ != protowire.BytesType {
			panic(typ)
		}
		v, n := protowire.ConsumeBytes(unk)
		if n < 0 {
			panic(protowire.ParseError(n))
		}
		return string(v), true
	}
	return "", false
}

func checkResourceShape(_ context.Context, rw check.ResponseWriter, _ check.Request, m protoreflect.MessageDescriptor) error {
	if !messageIsResource(m) {
		return nil
	}

	var hasKind, hasSubKind, hasVersion, hasMetadata, hasSpec, hasStatus, hasScope bool

	for field := range fields(m) {
		if fm := field.Message(); fm != nil &&
			(fm.FullName() == "types.ResourceHeader" || fm.FullName() == "teleport.header.v1.ResourceHeader") {
			// TODO(espadolini): check field name and gogoproto embedding
			if field.Cardinality() != protoreflect.Optional {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("ResourceHeader field %+q in resource %+q must not be repeated or required", field.Name(), m.FullName()),
				)
			}
			if hasKind || hasSubKind || hasVersion || hasMetadata {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("ResourceHeader field %+q in resource %+q must not be used with duplicated fields", field.Name(), m.FullName()),
				)
			}
			hasKind = true
			hasSubKind = true
			hasVersion = true
			hasMetadata = true
		} else {
			switch field.Name() {
			case "kind", "Kind",
				"sub_kind", "SubKind",
				"version", "Version",
				"metadata", "Metadata",
				"spec", "Spec",
				"status", "Status",
				"scope", "Scope":
			default:
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("unexpected top-level field %+q in resource %+q", field.Name(), m.FullName()),
				)
			}
		}

		validateStringField := func(lowercase, titlecase string, presence *bool) {
			if string(field.Name()) != lowercase && string(field.Name()) != titlecase {
				return
			}

			if field.Kind() != protoreflect.StringKind || field.Cardinality() != protoreflect.Optional || field.HasOptionalKeyword() || field.HasPresence() {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q must be an implicitly present string", lowercase, m.FullName()),
				)
			}

			jt, hasJT := getGogoJsonTag(field.Options())
			if string(field.Name()) == lowercase && hasJT && jt != lowercase && jt != lowercase+",omitempty" {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q should not have gogoproto.jsontag", lowercase, m.FullName()),
				)
			} else if string(field.Name()) != lowercase && jt != lowercase && jt != lowercase+",omitempty" {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q should be lowercase (or should set gogoproto.jsontag)", lowercase, m.FullName()),
				)
			}

			if *presence {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("duplicated %+q field in resource %+q", lowercase, m.FullName()),
				)
			}
			*presence = true
		}
		validateStringField("kind", "Kind", &hasKind)
		validateStringField("sub_kind", "SubKind", &hasSubKind)
		validateStringField("version", "Version", &hasVersion)
		validateStringField("scope", "Scope", &hasScope)

		validateMessageField := func(lowercase, titlecase string, presence *bool, allowedTypes ...string) {
			if string(field.Name()) != lowercase && string(field.Name()) != titlecase {
				return
			}

			if field.Kind() != protoreflect.MessageKind || field.Cardinality() != protoreflect.Optional || field.HasOptionalKeyword() {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q must be an implicitly present message", lowercase, m.FullName()),
				)
			}

			if len(allowedTypes) > 0 && !slices.Contains(allowedTypes, string(field.Message().FullName())) {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q must have the correct type", lowercase, m.FullName()),
				)
			}

			jt, hasJT := getGogoJsonTag(field.Options())
			if string(field.Name()) == lowercase && hasJT && jt != lowercase && jt != lowercase+",omitempty" {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q should not have gogoproto.jsontag", lowercase, m.FullName()),
				)
			} else if string(field.Name()) != lowercase && jt != lowercase && jt != lowercase+",omitempty" {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("%+q field in resource %+q should be lowercase (or should set gogoproto.jsontag)", lowercase, m.FullName()),
				)
			}

			if *presence {
				rw.AddAnnotation(
					check.WithDescriptor(field),
					check.WithMessagef("duplicated %+q field in resource %+q", lowercase, m.FullName()),
				)
			}
			*presence = true
		}
		validateMessageField("metadata", "Metadata", &hasMetadata, "types.Metadata", "teleport.header.v1.Metadata")
		validateMessageField("spec", "Spec", &hasSpec)
		validateMessageField("status", "Status", &hasStatus)
	}

	validatePresentField := func(lowercase string, presence bool) {
		if presence {
			return
		}
		rw.AddAnnotation(
			check.WithDescriptor(m),
			check.WithMessagef("missing %+q field in resource %+q", lowercase, m.FullName()),
		)
	}
	validatePresentField("kind", hasKind)
	validatePresentField("sub_kind", hasSubKind)
	validatePresentField("version", hasVersion)
	validatePresentField("metadata", hasMetadata)
	validatePresentField("spec", hasSpec)

	return nil
}

func messageIsResource(m protoreflect.MessageDescriptor) bool {
	switch m.FullName() {
	case "teleport.header.v1.ResourceHeader", "types.ResourceHeader":
		// a partial resource by construction
		return false
	case "proto.Event":
		// contains a types.ResourceHeader in a oneof
		return false
	case "types.WatchKind":
		// happens to have kind, subkind and version but is not a resource
		return false
	case "accessgraph.v1alpha.ResourceHeaderList":
		// uses a repeated types.ResourceHeader to specify a list of resources
		// to be deleted
		return false
	case "accessgraph.v1alpha.AccessPathChanged":
		// should be an audit log event but uses a teleport.header.v1.Metadata
		// by accident, which doesn't seem to be used anyway
		return false
	case "types.MessageWithHeader":
		// used to partially unmarshal a protojson-marshaled resource to extract
		// a version instead of doing something much simpler and straightforward
		return false
	}

	var count int
	for f := range fields(m) {
		if md := f.Message(); md != nil {
			switch md.FullName() {
			case "teleport.header.v1.ResourceHeader", "types.ResourceHeader":
				return true
			case "teleport.header.v1.Metadata", "types.Metadata":
				return true
			}
		}
		switch strings.ToLower(string(f.Name())) {
		case "kind", "subkind", "version", "scope", "spec", "status":
			count++
		}
	}
	return count >= 3
}

// checkPaginationMethod implements MethodRuleHandler for RuleSpec
func checkPaginationMethod(
	_ context.Context,
	responseWriter check.ResponseWriter,
	request check.Request,
	method protoreflect.MethodDescriptor,
) error {

	if method.IsStreamingServer() || method.IsStreamingClient() {
		// Streaming APIs do not expect pagination.
		return nil
	}

	if isMethodDeprecated(method) {
		// deprecated methods are skipped
		return nil
	}

	config := newDefaultConfig()
	if config.shouldSkip(method) {
		return nil
	}

	resp := method.Output()
	if !hasRepeated(resp.Fields()) {
		// Check if the method *should* have the repeated field.
		if strings.HasPrefix(string(method.Name()), methodPrefixMustHaveRepeated) {
			responseWriter.AddAnnotation(
				check.WithDescriptor(method),
				check.WithMessagef(
					"repeated fields expected for RPC names starting with: %q (RFD-0153)",
					methodPrefixMustHaveRepeated,
				),
			)
		}
		return nil
	}

	sizeName := config.getPageSizeFieldName(method)
	tokenName := config.getPageFieldName(method)
	nextName := config.getNextPageFieldName(method)

	req := method.Input()

	if !hasFieldName(req.Fields(), sizeName) {
		responseWriter.AddAnnotation(
			check.WithDescriptor(req),
			check.WithMessagef(
				"%q taken by %q is missing page size field name: %q (RFD-0153)",
				req.Name(),
				method.FullName(),
				sizeName,
			),
		)
	}

	if !hasFieldName(req.Fields(), tokenName) {
		responseWriter.AddAnnotation(
			check.WithDescriptor(req),
			check.WithMessagef(
				"%q taken by %q is missing page token field name: %q (RFD-0153)",
				req.Name(),
				method.FullName(),
				tokenName,
			),
		)
	}

	if !hasFieldName(resp.Fields(), nextName) {
		responseWriter.AddAnnotation(
			check.WithDescriptor(resp),
			check.WithMessagef(
				"%q returned by %q is missing next page token field name: %q (RFD-0153)",
				resp.Name(),
				method.FullName(),
				nextName,
			),
		)
	}

	return nil
}

// hasFieldName returns true if the proto fields match given name
func hasFieldName(fields protoreflect.FieldDescriptors, name string) bool {
	for i := 0; i < fields.Len(); i++ {
		if string(fields.Get(i).Name()) == name {
			return true
		}
	}

	return false
}

// hasRepeated returns true any of the proto fields are marked as `repeated`
func hasRepeated(fields protoreflect.FieldDescriptors) bool {
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)

		if field.IsMap() {
			// maps are technically repeated under the hood, but currently out of scope for the linter.
			continue
		}

		if field.Cardinality() == protoreflect.Repeated {
			return true
		}
	}

	return false
}

// isMethodDeprecated returns true if the RPC has been marked with deprecated.
func isMethodDeprecated(method protoreflect.MethodDescriptor) bool {
	options := method.Options()
	if options == nil {
		return false
	}
	if opts, ok := options.(*descriptorpb.MethodOptions); ok {
		return opts.GetDeprecated()
	}
	return false
}
