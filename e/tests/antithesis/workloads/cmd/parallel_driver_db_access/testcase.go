package main

import (
	"context"
	"maps"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestCase describes a single Teleport TestCase to be ran within Antithesis
type TestCase struct {
	// Name is the name of this test case
	Name string

	// Identity is the name of the identity to use.
	Identity string

	// Target describes the target database.
	Target DatabaseTarget

	// run is the parametized test case function to execute
	run TestCaseRunFunc
}

// TestCaseRunFunc is the signature for test case run function
type TestCaseRunFunc = func(context.Context, *TestCaseParams) error

// Params returns the params to execute this test case.
func (tc *TestCase) Params() *TestCaseParams {
	const (
		defaultDatabaseName = "postgres"
		defaultDatabaseUser = "testuser"
	)
	return &TestCaseParams{
		Identity:     tc.Identity,
		Target:       tc.Target,
		DatabaseName: defaultDatabaseName,
		DatabaseUser: defaultDatabaseUser,
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

	// Target describes the target database.
	Target DatabaseTarget

	// DatabaseName is the Postgres database name to connect to.
	DatabaseName string

	// DatabaseUser is the Postgres user to connect as.
	DatabaseUser string
}

// Details is a helper for [TestCaseParams] to add standard details to Antithesis assertions.
func (p *TestCaseParams) Details(details map[string]any) map[string]any {
	if details == nil {
		details = make(map[string]any)
	}
	details["identity"] = p.Identity
	details["target"] = p.Target.Details()
	details["db_name"] = p.DatabaseName
	details["db_user"] = p.DatabaseUser
	return details
}

func (p *TestCaseParams) routeFor(db types.Database) proto.RouteToDatabase {
	return proto.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    p.DatabaseUser,
		Database:    p.DatabaseName,
	}
}

// staticRoute builds a route to the target database without resolving it
// first.
func (p *TestCaseParams) staticRoute() proto.RouteToDatabase {
	return proto.RouteToDatabase{
		ServiceName: p.Target.Name,
		Protocol:    defaults.ProtocolPostgres,
		Username:    p.DatabaseUser,
		Database:    p.DatabaseName,
	}
}

// DatabaseTarget describes how to select a database used by a test case.
type DatabaseTarget struct {
	// Name selects a database by name.
	Name string

	// Labels selects a database by labels.
	Labels map[string]string

	// PredicateExpression selects a database by predicate expression.
	PredicateExpression string
}

func (t DatabaseTarget) name() string {
	return t.Name
}

func (t DatabaseTarget) labels() map[string]string {
	return maps.Clone(t.Labels)
}

func (t *DatabaseTarget) Details() map[string]any {
	return map[string]any{
		"name":                 t.Name,
		"labels":               t.Labels,
		"predicate_expression": t.PredicateExpression,
	}
}

func (t *DatabaseTarget) Matches(db types.Database) bool {
	if t.Name != "" && db.GetName() != t.Name {
		return false
	}
	labels := db.GetAllLabels()
	for key, want := range t.Labels {
		if labels[key] != want {
			return false
		}
	}
	return true
}

func (t DatabaseTarget) UsesResourceMatcher() bool {
	return len(t.Labels) > 0 || t.PredicateExpression != ""
}
