package identityfile

import (
	"io/ioutil"

	"github.com/gravitational/teleport/lib/client"
	"gopkg.in/check.v1"
)

type IdentityfileTestSuite struct {
}

var _ = check.Suite(&IdentityfileTestSuite{})

func (s *IdentityfileTestSuite) TestWrite(c *check.C) {
	keyFilePath := c.MkDir() + "openssh"

	var key client.Key
	key.Cert = []byte("cert")
	key.Priv = []byte("priv")
	key.Pub = []byte("pub")

	// test OpenSSH-compatible identity file creation:
	_, err := Write(keyFilePath, &key, FormatOpenSSH, nil, "")
	c.Assert(err, check.IsNil)

	// key is OK:
	out, err := ioutil.ReadFile(keyFilePath)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), check.Equals, "priv")

	// cert is OK:
	out, err = ioutil.ReadFile(keyFilePath + "-cert.pub")
	c.Assert(err, check.IsNil)
	c.Assert(string(out), check.Equals, "cert")

	// test standard Teleport identity file creation:
	keyFilePath = c.MkDir() + "file"
	_, err = Write(keyFilePath, &key, FormatFile, nil, "")
	c.Assert(err, check.IsNil)

	// key+cert are OK:
	out, err = ioutil.ReadFile(keyFilePath)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), check.Equals, "privcert")
}
