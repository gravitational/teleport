/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package parser

import (
	"strings"

	optionsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/options/v1"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ParseServiceDescriptor extracts a normalized ResourceSpec from a proto service descriptor.
func ParseServiceDescriptor(sd protoreflect.ServiceDescriptor) (spec.ResourceSpec, error) {
	if sd == nil {
		return spec.ResourceSpec{}, trace.BadParameter("service descriptor is required")
	}

	opts, ok := sd.Options().(*descriptorpb.ServiceOptions)
	if !ok || opts == nil {
		return spec.ResourceSpec{}, trace.BadParameter("service options are required")
	}
	if !hasResourceConfigOption(sd) {
		return spec.ResourceSpec{}, trace.BadParameter("service %s is missing teleport.resource_config option", sd.FullName())
	}
	cfg, err := resourceConfigFromOptions(opts)
	if err != nil {
		return spec.ResourceSpec{}, trace.Wrap(err)
	}

	kindName := inferKindName(sd.Name())
	rs, err := applyDefaults(string(sd.FullName()), kindName, cfg)
	if err != nil {
		return spec.ResourceSpec{}, trace.Wrap(err)
	}

	reqs := detectOperations(sd, kindName, &rs.Operations)
	if err := validateRequestShapes(rs.Storage, reqs); err != nil {
		return spec.ResourceSpec{}, trace.Wrap(err)
	}
	if err := rs.Validate(); err != nil {
		return spec.ResourceSpec{}, trace.Wrap(err)
	}

	return rs, nil
}

func hasResourceConfigOption(sd protoreflect.ServiceDescriptor) bool {
	if sd == nil {
		return false
	}
	opts, ok := sd.Options().(*descriptorpb.ServiceOptions)
	if !ok || opts == nil {
		return false
	}
	return opts.ProtoReflect().Has(optionsv1.E_ResourceConfig.TypeDescriptor())
}

func resourceConfigFromOptions(opts *descriptorpb.ServiceOptions) (*optionsv1.ResourceConfig, error) {
	if opts == nil {
		return nil, trace.BadParameter("service options are required")
	}

	field := optionsv1.E_ResourceConfig.TypeDescriptor()
	if !opts.ProtoReflect().Has(field) {
		return nil, trace.BadParameter("missing teleport.resource_config option")
	}

	msg := opts.ProtoReflect().Get(field).Message().Interface()
	wire, err := proto.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := &optionsv1.ResourceConfig{}
	if err := proto.Unmarshal(wire, cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

func inferKindName(serviceName protoreflect.Name) string {
	name := string(serviceName)
	return strings.TrimSuffix(name, "Service")
}

func detectOperations(sd protoreflect.ServiceDescriptor, kindName string, ops *spec.OperationSet) map[string]protoreflect.MessageDescriptor {
	reqs := map[string]protoreflect.MessageDescriptor{}

	for i := range sd.Methods().Len() {
		method := sd.Methods().Get(i)
		methodName := string(method.Name())
		suffix := methodName

		switch {
		case strings.HasPrefix(methodName, "Get"):
			suffix = strings.TrimPrefix(methodName, "Get")
			if isKindMethod(suffix, kindName) {
				ops.Get = true
				reqs["get"] = method.Input()
			}
		case strings.HasPrefix(methodName, "List"):
			suffix = strings.TrimPrefix(methodName, "List")
			if isKindMethod(suffix, kindName) {
				ops.List = true
				reqs["list"] = method.Input()
			}
		case strings.HasPrefix(methodName, "Create"):
			suffix = strings.TrimPrefix(methodName, "Create")
			if isKindMethod(suffix, kindName) {
				ops.Create = true
				reqs["create"] = method.Input()
			}
		case strings.HasPrefix(methodName, "Update"):
			suffix = strings.TrimPrefix(methodName, "Update")
			if isKindMethod(suffix, kindName) {
				ops.Update = true
				reqs["update"] = method.Input()
			}
		case strings.HasPrefix(methodName, "Upsert"):
			suffix = strings.TrimPrefix(methodName, "Upsert")
			if isKindMethod(suffix, kindName) {
				ops.Upsert = true
				reqs["upsert"] = method.Input()
			}
		case strings.HasPrefix(methodName, "Delete"):
			suffix = strings.TrimPrefix(methodName, "Delete")
			if isKindMethod(suffix, kindName) {
				ops.Delete = true
				reqs["delete"] = method.Input()
			}
		}
	}

	return reqs
}

func isKindMethod(methodSuffix, kindName string) bool {
	if methodSuffix == "" || kindName == "" {
		return false
	}
	// Check both singular (e.g. "AccessPolicy") and plural (e.g. "AccessPolicies")
	// because List RPCs use the plural form which may differ from a simple suffix
	// (e.g. "Policy" → "Policies", not "Policys").
	return strings.Contains(methodSuffix, kindName) || strings.Contains(methodSuffix, pluralizeName(kindName))
}

// pluralizeName applies English pluralization rules. This is duplicated from
// generators/path.go because the parser package cannot import generators.
func pluralizeName(s string) string {
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") {
		return s + "es"
	}
	if strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return s + "es"
	}
	if strings.HasSuffix(lower, "y") && len(s) > 1 {
		c := lower[len(lower)-2]
		if c != 'a' && c != 'e' && c != 'i' && c != 'o' && c != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	return s + "s"
}
