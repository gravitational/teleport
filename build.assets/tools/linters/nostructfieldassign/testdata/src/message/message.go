// Package message contains test fixtures for the nostructfieldassign linter.
// The analyzer is configured with a custom message:
// "example.com/mypkg.MyStruct.Forbidden# use SetForbidden() instead"
package message

import "example.com/mypkg"

func directAssignment() {
	var s mypkg.MyStruct
	s.Forbidden = "bad" // want `direct assignment to mypkg\.MyStruct\.Forbidden is forbidden because "use SetForbidden\(\) instead"`
}

func compositeLiteral() {
	_ = mypkg.MyStruct{
		Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden because "use SetForbidden\(\) instead"`
	}
}
