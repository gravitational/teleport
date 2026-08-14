package main

import (
	"context"
)

// TestCase describes a single Teleport TestCase to be ran within Antithesis
type TestCase struct {
	// Name is the name of this test case
	Name string

	// Identity is the name of the identity to use.
	Identity string

	// App describes the target application.
	App AppTarget

	// run is the parametized test case function to execute
	run TestCaseRunFunc
}

// TestCaseRunFunc is the signature for test case run function
type TestCaseRunFunc = func(context.Context, *TestCaseParams) error

// Params returns the params to execute this test case.
func (tc *TestCase) Params() *TestCaseParams {
	return &TestCaseParams{
		Identity: tc.Identity,
		App:      tc.App,
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

	// App describes the target application.
	App AppTarget
}

// Details is a helper for [TestCaseParams] to add standard details to Antithesis assertions.
func (p *TestCaseParams) Details(details map[string]any) map[string]any {
	if details == nil {
		details = make(map[string]any)
	}
	details["identity"] = p.Identity
	details["app_target"] = p.App.Details()
	return details
}

// AppTarget describes an application used by a test case.
type AppTarget struct {
	// Name is the Teleport application name.
	Name string

	// PublicAddr is the application public address.
	PublicAddr string

	// URI is the application target URI.
	URI string
}

func (t *AppTarget) Details() map[string]any {
	return map[string]any{
		"name":        t.Name,
		"public_addr": t.PublicAddr,
		"uri":         t.URI,
	}
}
