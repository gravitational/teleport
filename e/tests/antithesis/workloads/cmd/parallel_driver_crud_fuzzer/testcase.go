package main

import (
	"context"
)

// TestCase describes a single Teleport CRUD TestCase to be ran within Antithesis
type TestCase struct {
	// Name is the name of this test case
	Name string

	// Identity is the name of the identity to use.
	Identity string

	// Kinds are resource kinds which this test should run against
	Kinds []string

	// run is the parametized test case function to execute
	run TestCaseRunFunc
}

// TestCaseRunFunc is the signature for test case run function
type TestCaseRunFunc = func(context.Context, *TestCaseParams) error

// Run executes the TestCase test.
func (tc *TestCase) Run(ctx context.Context, params *TestCaseParams) error {
	return tc.run(ctx, params)
}

// TestCaseParams contains the parameters to use for a given test case.
type TestCaseParams struct {
	// Identity is the name of the identity to use.
	Identity string

	// Kind is the Teleport kind name to use for. this run.
	Kind string
}

// Details is a helper for [TestCaseParams] to add standard details to Antithesis assertions.
func (p *TestCaseParams) Details(details map[string]any) map[string]any {
	if details == nil {
		details = make(map[string]any)
	}
	details["identity"] = p.Identity
	details["kind"] = p.Kind
	return details
}
