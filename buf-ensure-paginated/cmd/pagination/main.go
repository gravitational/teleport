package main

import (
	"context"
	"strings"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var paginationRule = &check.RuleSpec{
	ID:      "PAGINATION_REQUIRED",
	Purpose: "Ensure RPCs starting with List or Search use 'limit', 'start_key', and 'next_key' pagination fields.",
	Type:    check.RuleTypeLint,
	Handler: checkutil.NewMethodRuleHandler(checkPagination, checkutil.WithoutImports()),
}

func main() {
	check.Main(&check.Spec{
		Rules: []*check.RuleSpec{paginationRule},
	})
}

func checkPagination(
	_ context.Context,
	responseWriter check.ResponseWriter,
	_ check.Request,
	method protoreflect.MethodDescriptor,
) error {
	name := string(method.Name())
	if !(strings.HasPrefix(name, "List") || strings.HasPrefix(name, "Search")) {
		return nil
	}

	req := method.Input()
	resp := method.Output()

	hasLimit := false
	hasStartKey := false
	hasNextKey := false

	for i := 0; i < req.Fields().Len(); i++ {
		field := req.Fields().Get(i).Name()
		switch field {
		case "limit":
			hasLimit = true
		case "start_key":
			hasStartKey = true
		}
	}

	for i := 0; i < resp.Fields().Len(); i++ {
		if resp.Fields().Get(i).Name() == "next_key" {
			hasNextKey = true
			break
		}
	}

	if !hasLimit || !hasStartKey || !hasNextKey {
		responseWriter.AddAnnotation(
			check.WithDescriptor(method),
			check.WithMessagef(
				"RPC %q must include `limit` and `start_key` in the request and `next_key` in the response",
				name,
			),
		)
	}

	return nil
}
