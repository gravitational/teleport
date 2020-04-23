package proxy

import (
	"crypto/x509"
	"encoding/pem"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"gopkg.in/check.v1"
)

type ForwarderSuite struct{}

var _ = check.Suite(ForwarderSuite{})

func (s ForwarderSuite) TestRequestCertificate(c *check.C) {
	cl := &mockClient{
		csrResp: auth.KubeCSRResponse{
			Cert:            []byte("mock cert"),
			CertAuthorities: [][]byte{[]byte("mock CA")},
			TargetAddr:      "mock addr",
		},
	}
	f := &Forwarder{
		ForwarderConfig: ForwarderConfig{
			Keygen: testauthority.New(),
			Client: cl,
		},
	}
	user, err := services.NewUser("bob")
	c.Assert(err, check.IsNil)
	ctx := authContext{
		cluster: cluster{
			RemoteSite: mockRemoteSite{name: "site a"},
		},
		AuthContext: auth.AuthContext{
			User: user,
			Identity: tlsca.Identity{
				Username:         "bob",
				Groups:           []string{"group a", "group b"},
				Usage:            []string{"usage a", "usage b"},
				Principals:       []string{"principal a", "principal b"},
				KubernetesGroups: []string{"k8s group a", "k8s group b"},
				Traits:           map[string][]string{"trait a": []string{"b", "c"}},
			},
		},
	}

	b, err := f.requestCertificate(ctx)
	c.Assert(err, check.IsNil)
	// All fields except b.key are predictable.
	c.Assert(b.cert, check.DeepEquals, cl.csrResp.Cert)
	c.Assert(b.certAuthorities, check.DeepEquals, cl.csrResp.CertAuthorities)
	c.Assert(b.targetAddr, check.DeepEquals, cl.csrResp.TargetAddr)

	// Check the KubeCSR fields.
	c.Assert(cl.gotCSR.Username, check.DeepEquals, ctx.User.GetName())
	c.Assert(cl.gotCSR.ClusterName, check.DeepEquals, ctx.cluster.GetName())

	// Parse x509 CSR and check the subject.
	csrBlock, _ := pem.Decode(cl.gotCSR.CSR)
	c.Assert(csrBlock, check.NotNil)
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	c.Assert(err, check.IsNil)
	idFromCSR, err := tlsca.FromSubject(csr.Subject, time.Time{})
	c.Assert(err, check.IsNil)
	c.Assert(idFromCSR, check.DeepEquals, ctx.Identity)
}

// mockClient to intercept ProcessKubeCSR requests, record them and return a
// stub response.
type mockClient struct {
	auth.ClientI

	csrResp auth.KubeCSRResponse
	gotCSR  auth.KubeCSR
}

func (c *mockClient) ProcessKubeCSR(csr auth.KubeCSR) (*auth.KubeCSRResponse, error) {
	c.gotCSR = csr
	return &c.csrResp, nil
}

// mockRemoteSite is a reversetunnel.RemoteSite implementation with hardcoded
// name, because there's no easy way to construct a real
// reversetunnel.RemoteSite.
type mockRemoteSite struct {
	reversetunnel.RemoteSite
	name string
}

func (s mockRemoteSite) GetName() string {
	return s.name
}
