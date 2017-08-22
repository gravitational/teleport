package qr

import (
	"bytes"
	"testing"
)

func makeString(length int, content string) string {
	res := ""

	for i := 0; i < length; i++ {
		res += content
	}

	return res
}

func Test_AlphaNumericEncoding(t *testing.T) {
	encode := AlphaNumeric.getEncoder()

	x, vi, err := encode("HELLO WORLD", M)

	if x == nil || vi == nil || vi.Version != 1 || bytes.Compare(x.GetBytes(), []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}) != 0 {
		t.Errorf("\"HELLO WORLD\" failed to encode: %s", err)
	}

	x, vi, err = encode(makeString(4296, "A"), L)
	if x == nil || vi == nil || err != nil {
		t.Fail()
	}
	x, vi, err = encode(makeString(4297, "A"), L)
	if x != nil || vi != nil || err == nil {
		t.Fail()
	}
	x, vi, err = encode("ABc", L)
	if x != nil || vi != nil || err == nil {
		t.Fail()
	}
	x, vi, err = encode("hello world", M)

	if x != nil || vi != nil || err == nil {
		t.Error("\"hello world\" should not be encodable in alphanumeric mode")
	}
}
