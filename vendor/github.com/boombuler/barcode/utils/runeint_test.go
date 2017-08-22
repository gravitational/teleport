package utils

import "testing"

func Test_RuneToIntIntToRune(t *testing.T) {
	if IntToRune(0) != '0' {
		t.Errorf("failed IntToRune(0) returned %d", string(IntToRune(0)))
	}
	if IntToRune(9) != '9' {
		t.Errorf("failed IntToRune(9) returned %d", IntToRune(9))
	}
	if IntToRune(10) != 'F' {
		t.Errorf("failed IntToRune(10) returned %d", IntToRune(10))
	}
	if RuneToInt('0') != 0 {
		t.Error("failed RuneToInt('0') returned %d", RuneToInt(0))
	}
	if RuneToInt('9') != 9 {
		t.Error("failed RuneToInt('9') returned %d", RuneToInt(9))
	}
	if RuneToInt('F') != -1 {
		t.Error("failed RuneToInt('F') returned %d", RuneToInt('F'))
	}
}
