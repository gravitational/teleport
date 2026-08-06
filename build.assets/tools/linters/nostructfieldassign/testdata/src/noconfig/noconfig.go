// Package noconfig contains test fixtures for the nostructfieldassign linter
// when no fields are configured. No diagnostics should be reported.
package noconfig

import "example.com/mypkg"

func f() {
	var s mypkg.MyStruct
	s.Forbidden = "would be flagged if configured"
	_ = mypkg.MyStruct{Forbidden: "same"}
}
