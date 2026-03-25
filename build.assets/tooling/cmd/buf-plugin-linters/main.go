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
	"strings"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
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
	},
}

func main() {
	check.Main(paginationSpec)
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
