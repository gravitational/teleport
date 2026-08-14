package main

import (
	"context"
	"maps"
)

// TestCase describes a single Teleport TestCase to be ran within Antithesis
type TestCase struct {
	// Name is the name of this test case
	Name string

	// Identity is the name of the identity to use.
	Identity string

	// Target describes how the target SSH node should be selected.
	Target TargetSelector

	// run is the parametized test case function to execute
	run TestCaseRunFunc
}

// TestCaseRunFunc is the signature for test case run function
type TestCaseRunFunc = func(context.Context, *TestCaseParams) error

// Params returns the params to execute this test case.
func (tc *TestCase) Params() *TestCaseParams {
	const defaultHostUser = "root"

	return &TestCaseParams{
		Identity: tc.Identity,
		Target:   tc.Target,
		HostUser: defaultHostUser,
	}
}

// Run executes the TestCase test.
func (tc *TestCase) Run(ctx context.Context, params *TestCaseParams) error {
	return tc.run(ctx, params)
}

// TestCaseParams contains the parameters to use for a given test case.
type TestCaseParams struct {
	// Identity is the name of the identity to use.
	Identity string

	// Target describes how the target SSH node should be selected.
	Target TargetSelector

	// HostUser is a user login on a remote host
	HostUser string
}

// Details is a helper for [TestCaseParams] to add standard details to Antithesis assertions.
func (p *TestCaseParams) Details(details map[string]any) map[string]any {
	if details == nil {
		details = make(map[string]any)
	}
	details["identity"] = p.Identity
	details["target"] = p.Target.Details()
	return details
}

// TargetSelector describes how to select the target SSH node for a test case.
type TargetSelector struct {
	// Host selects a node by name.
	Host string

	// Labels selects a node by labels.
	Labels map[string]string

	// PredicateExpression selects a node by predicate expression.
	PredicateExpression string
}

func (t TargetSelector) host() string {
	return t.Host
}

func (t TargetSelector) labels() map[string]string {
	return maps.Clone(t.Labels)
}

func (t TargetSelector) usesResourceMatcher() bool {
	return len(t.Labels) > 0 || t.PredicateExpression != ""
}

func (t *TargetSelector) Details() map[string]any {
	return map[string]any{
		"host":                 t.Host,
		"labels":               t.Labels,
		"predicate_expression": t.PredicateExpression,
	}
}
