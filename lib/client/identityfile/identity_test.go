package identityfile

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type IdentityfileTestSuite struct {
}

var _ = check.Suite(&IdentityfileTestSuite{})

func (s *IdentityfileTestSuite) TestWrite(c *check.C) {
	keyFilePath := c.MkDir() + "openssh"

	key := client.Key{
		Cert:        []byte("cert"),
		TLSCert:     []byte("tls-cert"),
		Priv:        []byte("priv"),
		Pub:         []byte("pub"),
		ClusterName: "foo",
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: [][]byte{[]byte("ca-cert")},
		}},
	}

	// test OpenSSH-compatible identity file creation:
	_, err := Write(keyFilePath, &key, FormatOpenSSH, "")
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
	_, err = Write(keyFilePath, &key, FormatFile, "")
	c.Assert(err, check.IsNil)

	// key+cert are OK:
	out, err = ioutil.ReadFile(keyFilePath)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), check.Equals, "priv\ncert\ntls-cert\nca-cert\n")

	// Test kubeconfig creation.
	kubeconfigPath := filepath.Join(c.MkDir(), "kubeconfig")
	_, err = Write(kubeconfigPath, &key, FormatKubernetes, "far.away.cluster")
	c.Assert(err, check.IsNil)

	// Check that kubeconfig is OK.
	kc, err := kubeconfig.Load(kubeconfigPath)
	c.Assert(err, check.IsNil)
	c.Assert(len(kc.AuthInfos), check.Equals, 1)
	c.Assert(len(kc.Clusters), check.Equals, 1)
	c.Assert(kc.Clusters[key.ClusterName].Server, check.Equals, "far.away.cluster")
	c.Assert(len(kc.Contexts), check.Equals, 1)
}
