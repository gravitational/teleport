// package test contains a backend acceptance test suite that is backend implementation independant
// each backend will use the suite to test itself
package test

import (
	"testing"
	"time"

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

func (s *BackendSuite) UserKeyCRUD(c *C) {
	k := backend.AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.B.UpsertUserKey("user1", k, 0), IsNil)

	keys, err := s.B.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []backend.AuthorizedKey{k})

	c.Assert(s.B.DeleteUserKey("user1", k.ID), IsNil)

	keys, err = s.B.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []backend.AuthorizedKey{})
}

func (s *BackendSuite) UsersCRUD(c *C) {
	k := backend.AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.B.UpsertUserKey("user1", k, 0), IsNil)
	c.Assert(s.B.UpsertUserKey("user2", k, 0), IsNil)

	u, err := s.B.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user1": struct{}{}, "user2": struct{}{}})

	c.Assert(s.B.DeleteUser("user1"), IsNil)

	u, err = s.B.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user2": struct{}{}})

	c.Assert(s.B.DeleteUser("user1"), FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) UserCACRUD(c *C) {
	ca := backend.CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.B.UpsertUserCA(ca), IsNil)

	out, err := s.B.GetUserCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.B.GetUserCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *BackendSuite) HostCACRUD(c *C) {
	ca := backend.CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.B.UpsertHostCA(ca), IsNil)

	out, err := s.B.GetHostCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.B.GetHostCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *BackendSuite) ServerCRUD(c *C) {
	srv := backend.Server{ID: "srv1", Addr: "localhost:2022"}
	c.Assert(s.B.UpsertServer(srv, 0), IsNil)

	out, err := s.B.GetServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []backend.Server{srv})
}

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}
