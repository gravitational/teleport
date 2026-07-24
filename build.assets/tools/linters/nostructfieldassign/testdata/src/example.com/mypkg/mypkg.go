// Package mypkg provides test types for the nostructfieldassign linter tests.
package mypkg

// MyStruct is the test subject. Fields are either forbidden or allowed depending
// on how the analyzer is configured in each test case.
type MyStruct struct {
	Forbidden string
	Allowed   string
	WithMsg   string
	PtrField  string
}
