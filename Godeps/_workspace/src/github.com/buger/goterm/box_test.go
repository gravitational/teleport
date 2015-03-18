package goterm

import (
	"fmt"
	"testing"
)

func TestBox(t *testing.T) {
	boxSample := `
┌--------┐
│ hello  │
│ world  │
│ test   │
└--------┘`

	box := NewBox(10, 5, 0)
	fmt.Fprint(box, "hello\nworld\ntest")

	if box.String() != boxSample[1:] {
		t.Error("\n" + box.String())
		t.Error("!=")
		t.Error(boxSample)
	}
}
