/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
package main

import (
	"context"
	"strings"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	name := string(method.Name())
	req := method.Input()
	resp := method.Output()

	cfg, err := parseOptions(request.Options())
	if err != nil {
		return err
	}

	paginationExpected := hasAnyPrefix(name, cfg.prefixes)
	hasPageSize := hasAnyField(req.Fields(), cfg.sizeNames)
	hasPageToken := hasAnyField(req.Fields(), cfg.tokenNames)
	hasNextKey := hasAnyField(resp.Fields(), cfg.nextNames)
	checkRepeated := cfg.checkRepeated && hasRepeatedField(resp.Fields())

	if !paginationExpected && !checkRepeated {
		return nil
	}

	if !hasPageSize {
		responseWriter.AddAnnotation(
			check.WithDescriptor(req),
			check.WithMessagef(
				"%q taken by %q is missing page size field name: %v",
				req.Name(),
				name,
				cfg.sizeNames,
			),
		)
	}

	if !hasPageToken {
		responseWriter.AddAnnotation(
			check.WithDescriptor(req),
			check.WithMessagef(
				"%q taken by %q is missing page token field name: %v",
				req.Name(),
				name,
				cfg.tokenNames,
			),
		)
	}

	if !hasNextKey {
		responseWriter.AddAnnotation(
			check.WithDescriptor(resp),
			check.WithMessagef(
				"%q returned by %q is missing next page token field name: %v",
				resp.Name(),
				name,
				cfg.nextNames,
			),
		)
	}

	return nil
}

// hasAnyPrefix return true if given name starts with any of the given prefixes
func hasAnyPrefix(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// hasAnyField returns true if the proto fields contain any one of the requested names.
func hasAnyField(fields protoreflect.FieldDescriptors, names []string) bool {
	nameSet := make(map[protoreflect.Name]struct{}, len(names))
	for _, n := range names {
		nameSet[protoreflect.Name(n)] = struct{}{}
	}
	for i := 0; i < fields.Len(); i++ {
		if _, ok := nameSet[fields.Get(i).Name()]; ok {
			return true
		}
	}

	return false
}

// hasRepeatedField returns true any of the proto fields are marked as `repeated`
func hasRepeatedField(fields protoreflect.FieldDescriptors) bool {
	for i := 0; i < fields.Len(); i++ {
		if fields.Get(i).Cardinality() == protoreflect.Repeated {
			return true
		}
	}
	return false
}
