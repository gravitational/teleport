// Package forbidden contains test fixtures for the nostructfieldassign linter.
// The analyzer is configured with "example.com/mypkg.MyStruct.Forbidden".
package forbidden

import "example.com/mypkg"

// directAssignment verifies that x.Forbidden = v is caught.
func directAssignment() {
	var s mypkg.MyStruct
	s.Forbidden = "bad" // want `direct assignment to mypkg\.MyStruct\.Forbidden is forbidden`
	s.Allowed = "ok"    // allowed field: no diagnostic
}

// pointerReceiver verifies that (*T).Forbidden = v is caught.
func pointerReceiver() {
	p := &mypkg.MyStruct{}
	p.Forbidden = "bad" // want `direct assignment to mypkg\.MyStruct\.Forbidden is forbidden`
	p.Allowed = "ok"    // allowed field: no diagnostic
}

// compositeLiteral verifies that MyStruct{Forbidden: v} is caught.
func compositeLiteral() {
	_ = mypkg.MyStruct{
		Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden`
		Allowed:   "ok",  // allowed field: no diagnostic
	}
}

// pointerCompositeLiteral verifies that &MyStruct{Forbidden: v} is also caught.
func pointerCompositeLiteral() {
	_ = &mypkg.MyStruct{
		Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden`
	}
}

// nestedElidedCompositeLiteral verifies that type-elided nested literals are also caught.
func nestedElidedCompositeLiteral() {
	_ = []mypkg.MyStruct{
		{
			Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden`
		},
	}
	_ = [1]mypkg.MyStruct{
		{
			Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden`
		},
	}
	_ = [...]mypkg.MyStruct{
		{
			Allowed: "ok", // allowed field: no diagnostic
		},
		{
			Forbidden: "bad", // want `setting mypkg\.MyStruct\.Forbidden in a composite literal is forbidden`
		},
	}
}
