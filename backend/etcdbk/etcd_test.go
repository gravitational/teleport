package etcdbk

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/gravitational/teleport/backend/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { TestingT(t) }

type EtcdSuite struct {
	bk         *bk
	suite      test.BackendSuite
	nodes      []string
	etcdPrefix string
	client     *etcd.Client
	changesC   chan interface{}
	key        string
	stopC      chan bool
}

var _ = Suite(&EtcdSuite{
	etcdPrefix: "/teleport_test",
})

func (s *EtcdSuite) SetUpSuite(c *C) {
	nodes_string := os.Getenv("TELEPORT_TEST_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}
	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry
	b, err := New(s.nodes, s.etcdPrefix)
	c.Assert(err, IsNil)
	s.bk = b.(*bk)
	s.client = s.bk.client

	s.changesC = make(chan interface{})
	s.stopC = make(chan bool)

	// Delete all values under the given prefix
	_, err = s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		if !notFound(err) {
			c.Assert(err, IsNil)
		}
	} else {
		_, err = s.bk.client.Delete(s.etcdPrefix, true)
		if !notFound(err) {
			c.Assert(err, IsNil)
		}
	}

	// Set up suite
	s.suite.ChangesC = s.changesC
	s.suite.B = b
}

func (s *EtcdSuite) TearDownTest(c *C) {
	close(s.stopC)
	c.Assert(s.bk.Close(), IsNil)
}

func (s *EtcdSuite) TestFromString(c *C) {
	b, err := FromString(fmt.Sprintf(`{"nodes": [%#v], "key": "%v"}`, s.nodes[0], s.etcdPrefix))
	c.Assert(err, IsNil)
	c.Assert(b, NotNil)
}

func (s *EtcdSuite) TestUserKeyCRUD(c *C) {
	s.suite.UserKeyCRUD(c)
}

func (s *EtcdSuite) TestUserCACRUD(c *C) {
	s.suite.UserCACRUD(c)
}

func (s *EtcdSuite) TestHostCACRUD(c *C) {
	s.suite.HostCACRUD(c)
}

func (s *EtcdSuite) TestServerCRUD(c *C) {
	s.suite.ServerCRUD(c)
}

func (s *EtcdSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *EtcdSuite) TestPasswordHashCRUD(c *C) {
	s.suite.PasswordHashCRUD(c)
}

func (s *EtcdSuite) TestWebSessionCRUD(c *C) {
	s.suite.WebSessionCRUD(c)
}

func (s *EtcdSuite) TestWebTunCRUD(c *C) {
	s.suite.WebTunCRUD(c)
}

func (s *EtcdSuite) TestLocking(c *C) {
	s.suite.Locking(c)
}

func (s *EtcdSuite) TestToken(c *C) {
	s.suite.TokenCRUD(c)
}
