// package test contains a backend acceptance test suite that is backend implementation independant
// each backend will use the suite to test itself
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/random"
	"github.com/gravitational/teleport/backend"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
	B        backend.Backend
	ChangesC chan interface{}
}

func (s *BackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.ChangesC:
			// successfully collected changes
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout occured waiting for events")
		}
	}
	return changes
}

func (s *BackendSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *BackendSuite) BasicCRUD(c *C) {
	keys, err := s.B.GetKeys([]string{"keys"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{})

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	keys, err = s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey", "bkey"})

	out, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val-updated"), 0), IsNil)
	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val-updated")

	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), IsNil)
	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), FitsTypeOf, &teleport.NotFoundError{})
}

func (s *BackendSuite) Expiration(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	time.Sleep(2 * time.Second)

	keys, err := s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey"})
}

func (s *BackendSuite) ValueAndTTl(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey",
		[]byte("val1"), 2000*time.Millisecond), IsNil)

	time.Sleep(500 * time.Millisecond)

	value, ttl, err := s.B.GetValAndTTL([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(value), DeepEquals, "val1")
	ttlIsRight := (ttl < 1900*time.Millisecond) && (ttl > 1100*time.Millisecond)
	c.Assert(ttlIsRight, Equals, true)
}

func (s *BackendSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"

	c.Assert(s.B.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.B.AcquireLock(tok1, 30*time.Second), IsNil)
	x := 7
	go func() {
		time.Sleep(1 * time.Second)
		x = 9
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	x = x * 2
	c.Assert(x, Equals, 18)
	c.Assert(s.B.ReleaseLock(tok1), IsNil)

	y := 0
	go func() {
		c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
		c.Assert(s.B.AcquireLock(tok2, 0), IsNil)

		c.Assert(s.B.ReleaseLock(tok1), IsNil)
		c.Assert(s.B.ReleaseLock(tok2), IsNil)
		y = 15
	}()

	time.Sleep(1 * time.Second)
	c.Assert(y, Equals, 15)

	c.Assert(s.B.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})
}

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}

func randomToken() string {
	token, err := (&random.CSPRNG{}).HexDigest(32)
	if err != nil {
		panic(err)
	}
	return token
}
