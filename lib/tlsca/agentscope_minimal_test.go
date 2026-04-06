package tlsca_test

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tlsca"
)

// TestAgentScopePanicMinimal demonstrates the unsafe type assertion panic
func TestAgentScopePanicMinimal(t *testing.T) {
	agentScopeOID := asn1.ObjectIdentifier{1, 3, 9999, 2, 25}

	t.Run("integer_value_panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("✓ Panic confirmed: %v", r)
			}
		}()

		subject := pkix.Name{
			CommonName:   "test",
			Organization: []string{"test"},
			Names: []pkix.AttributeTypeAndValue{
				{Type: agentScopeOID, Value: 999}, // Integer instead of string
			},
		}

		_, _ = tlsca.FromSubject(subject, time.Now().Add(time.Hour))
		t.Error("Expected panic")
	})

	t.Run("string_value_works", func(t *testing.T) {
		subject := pkix.Name{
			CommonName:   "test",
			Organization: []string{"test"},
			Names: []pkix.AttributeTypeAndValue{
				{Type: agentScopeOID, Value: "production"}, // String is safe
			},
		}

		id, err := tlsca.FromSubject(subject, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("FromSubject failed: %v", err)
		}

		if id.AgentScope != "production" {
			t.Errorf("Expected 'production', got '%s'", id.AgentScope)
		}
		t.Log("✓ String values work correctly")
	})
}
