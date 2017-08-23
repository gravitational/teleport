package secret

import (
	"crypto/subtle"
	"fmt"
	"testing"

	"github.com/mailgun/lemma/random"
)

var _ = fmt.Printf // for testing

func TestEncryptDecryptCycle(t *testing.T) {
	randomProvider = &random.FakeRNG{}

	key, err := NewKey()
	if err != nil {
		t.Errorf("Got unexpected response from NewKey: %v", err)
	}

	s, err := New(&Config{KeyBytes: key})
	if err != nil {
		t.Errorf("Got unexpected response from NewWithKeyBytes: %v", err)
	}

	message := []byte("hello, box!")
	sealed, err := s.Seal(message)
	if err != nil {
		t.Errorf("Got unexpected response from Seal: %v", err)
	}

	out, err := s.Open(sealed)
	if err != nil {
		t.Errorf("Got unexpected response from Open: %v", err)
	}

	// compare the messages
	if subtle.ConstantTimeCompare(message, out) != 1 {
		t.Errorf("Contents do not match: %v, %v", message, out)
	}
}

func TestEncryptDecryptCyclePackage(t *testing.T) {
	randomProvider = &random.FakeRNG{}

	key, err := NewKey()
	if err != nil {
		t.Errorf("Got unexpected response from NewKey: %v", err)
	}

	message := []byte("hello, box!")
	sealed, err := Seal(message, key)
	if err != nil {
		t.Errorf("Got unexpected response from Seal: %v", err)
	}

	out, err := Open(sealed, key)
	if err != nil {
		t.Errorf("Got unexpected response from Open: %v", err)
	}

	// compare the messages
	if subtle.ConstantTimeCompare(message, out) != 1 {
		t.Errorf("Contents do not match: %v, %v", message, out)
	}
}
