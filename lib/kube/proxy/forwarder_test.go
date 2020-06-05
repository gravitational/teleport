package proxy

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"

	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type ForwarderSuite struct{}

var _ = check.Suite(ForwarderSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

func (s ForwarderSuite) TestRequestCertificate(c *check.C) {
	cl, err := newMockCSRClient()
	c.Assert(err, check.IsNil)
	f := &Forwarder{
		ForwarderConfig: ForwarderConfig{
			Keygen: testauthority.New(),
			Client: cl,
		},
		Entry: logrus.NewEntry(logrus.New()),
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
	c.Assert(b.Certificates[0].Certificate[0], check.DeepEquals, cl.lastCert.Raw)
	c.Assert(len(b.RootCAs.Subjects()), check.Equals, 1)

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
	c.Assert(*idFromCSR, check.DeepEquals, ctx.Identity)
}

func (s ForwarderSuite) TestGetClusterSession(c *check.C) {
	clusterSessions, err := ttlmap.New(defaults.ClientCacheSize)
	c.Assert(err, check.IsNil)
	f := &Forwarder{
		clusterSessions: clusterSessions,
		Entry:           logrus.NewEntry(logrus.New()),
	}

	user, err := services.NewUser("bob")
	c.Assert(err, check.IsNil)
	remote := &mockRemoteSite{name: "site a"}
	ctx := authContext{
		cluster: cluster{
			isRemote:   true,
			RemoteSite: remote,
		},
		AuthContext: auth.AuthContext{
			User: user,
		},
	}
	sess := &clusterSession{authContext: ctx}

	// Initial clusterSessions is empty, no session should be found.
	c.Assert(f.getClusterSession(ctx), check.IsNil)

	// Add a session to clusterSessions, getClusterSession should find it.
	clusterSessions.Set(ctx.key(), sess, time.Hour)
	c.Assert(f.getClusterSession(ctx), check.Equals, sess)

	// Close the RemoteSite out-of-band (like when a remote cluster got removed
	// via tctl), getClusterSession should notice this and discard the
	// clusterSession.
	remote.closed = true
	c.Assert(f.getClusterSession(ctx), check.IsNil)
	_, ok := f.clusterSessions.Get(ctx.key())
	c.Assert(ok, check.Equals, false)
}

func (s ForwarderSuite) TestAuthenticate(c *check.C) {
	cc, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		ClientIdleTimeout:     services.NewDuration(time.Hour),
		DisconnectExpiredCert: true,
	})
	c.Assert(err, check.IsNil)
	ap := mockAccessPoint{clusterConfig: cc}

	user, err := services.NewUser("user-a")
	c.Assert(err, check.IsNil)

	tun := mockRevTunnel{
		sites: map[string]reversetunnel.RemoteSite{
			"remote": mockRemoteSite{name: "remote"},
			"local":  mockRemoteSite{name: "local"},
		},
	}

	f := &Forwarder{
		Entry: logrus.NewEntry(logrus.New()),
		ForwarderConfig: ForwarderConfig{
			ClusterName: "local",
			Tunnel:      tun,
			AccessPoint: ap,
		},
		creds: kubeCreds{targetAddr: "k8s.example.com"},
	}

	tests := []struct {
		desc           string
		user           auth.IdentityGetter
		authzErr       bool
		roleKubeUsers  []string
		roleKubeGroups []string
		routeToCluster string

		wantKubeUsers  []string
		wantKubeGroups []string
		wantRemote     bool
		wantErr        bool
	}{
		{
			desc:           "local user and cluster",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",

			wantKubeUsers:  []string{"user-a"},
			wantKubeGroups: []string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated},
		},
		{
			desc:           "remote user and local cluster",
			user:           auth.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",

			wantKubeUsers:  []string{"user-a"},
			wantKubeGroups: []string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated},
		},
		{
			desc:           "local user and remote cluster",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",

			wantKubeUsers:  []string{"user-a"},
			wantKubeGroups: []string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated},
			wantRemote:     true,
		},
		{
			desc:           "remote user and remote cluster",
			user:           auth.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",

			wantErr: true,
		},
		{
			desc:           "kube users passed in request",
			user:           auth.LocalUser{},
			roleKubeUsers:  []string{"kube-user-a", "kube-user-b"},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",

			wantKubeUsers:  []string{"kube-user-a", "kube-user-b"},
			wantKubeGroups: []string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated},
		},
		{
			desc:     "authorization failure",
			user:     auth.LocalUser{},
			authzErr: true,

			wantErr: true,
		},
		{
			desc: "unsupported user type",
			user: auth.BuiltinRole{},

			wantErr: true,
		},
	}
	for _, tt := range tests {
		c.Log(tt.desc)

		roles, err := services.FromSpec("ops", services.RoleSpecV3{
			Allow: services.RoleConditions{
				KubeUsers:  tt.roleKubeUsers,
				KubeGroups: tt.roleKubeGroups,
			},
		})
		c.Assert(err, check.IsNil)
		authCtx := auth.AuthContext{
			User:     user,
			Checker:  roles,
			Identity: tlsca.Identity{RouteToCluster: tt.routeToCluster},
		}
		authz := mockAuthorizer{ctx: &authCtx}
		if tt.authzErr {
			authz.err = trace.AccessDenied("denied!")
		}
		f.Auth = authz

		req := &http.Request{
			Host:       "example.com",
			RemoteAddr: "user.example.com",
			TLS: &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{
					{NotAfter: time.Now().Add(time.Hour)},
				},
			},
		}
		ctx := context.WithValue(context.Background(), auth.ContextUser, tt.user)
		req = req.WithContext(ctx)

		gotCtx, err := f.authenticate(req)
		if tt.wantErr {
			c.Assert(err, check.NotNil)
			c.Assert(trace.IsAccessDenied(err), check.Equals, true)
			continue
		} else {
			c.Assert(err, check.IsNil)
		}

		c.Assert(gotCtx.kubeUsers, check.DeepEquals, utils.StringsSet(tt.wantKubeUsers))
		c.Assert(gotCtx.kubeGroups, check.DeepEquals, utils.StringsSet(tt.wantKubeGroups))
		c.Assert(gotCtx.cluster.isRemote, check.Equals, tt.wantRemote)
		c.Assert(gotCtx.cluster.targetAddr, check.Equals, f.creds.targetAddr)
		c.Assert(gotCtx.cluster.RemoteSite.GetName(), check.Equals, tt.routeToCluster)
		c.Assert(gotCtx.cluster.remoteAddr.String(), check.Equals, req.RemoteAddr)
		c.Assert(gotCtx.disconnectExpiredCert, check.DeepEquals, req.TLS.PeerCertificates[0].NotAfter)
	}
}

func (s ForwarderSuite) TestExec(c *check.C) {
	c.Fatal("TODO")
}

func (s ForwarderSuite) TestPortForward(c *check.C) {
	c.Fatal("TODO")
}

func (s ForwarderSuite) TestSetupImpersonationHeaders(c *check.C) {
	c.Fatal("TODO")
}

func (s ForwarderSuite) TestNewClusterSession(c *check.C) {
	c.Fatal("TODO")
}

// mockCSRClient to intercept ProcessKubeCSR requests, record them and return a
// stub response.
type mockCSRClient struct {
	auth.ClientI

	ca       *tlsca.CertAuthority
	gotCSR   auth.KubeCSR
	lastCert *x509.Certificate
}

func newMockCSRClient() (*mockCSRClient, error) {
	ca, err := tlsca.New([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	if err != nil {
		return nil, err
	}
	return &mockCSRClient{ca: ca}, nil
}

func (c *mockCSRClient) ProcessKubeCSR(csr auth.KubeCSR) (*auth.KubeCSRResponse, error) {
	c.gotCSR = csr

	x509CSR, err := tlsca.ParseCertificateRequestPEM(csr.CSR)
	if err != nil {
		return nil, err
	}
	caCSR := tlsca.CertificateRequest{
		Clock:     clockwork.NewFakeClock(),
		PublicKey: x509CSR.PublicKey.(crypto.PublicKey),
		Subject:   x509CSR.Subject,
		NotAfter:  time.Now().Add(time.Minute),
		DNSNames:  x509CSR.DNSNames,
	}
	cert, err := c.ca.GenerateCertificate(caCSR)
	if err != nil {
		return nil, err
	}
	c.lastCert, err = tlsca.ParseCertificatePEM(cert)
	if err != nil {
		return nil, err
	}
	return &auth.KubeCSRResponse{
		Cert:            cert,
		CertAuthorities: [][]byte{[]byte(fixtures.SigningCertPEM)},
		TargetAddr:      "mock addr",
	}, nil
}

// mockRemoteSite is a reversetunnel.RemoteSite implementation with hardcoded
// name, because there's no easy way to construct a real
// reversetunnel.RemoteSite.
type mockRemoteSite struct {
	reversetunnel.RemoteSite
	name   string
	closed bool
}

func (s mockRemoteSite) GetName() string { return s.name }
func (s mockRemoteSite) IsClosed() bool  { return s.closed }

type mockAccessPoint struct {
	auth.AccessPoint

	clusterConfig services.ClusterConfig
}

func (ap mockAccessPoint) GetClusterConfig(...services.MarshalOption) (services.ClusterConfig, error) {
	return ap.clusterConfig, nil
}

type mockRevTunnel struct {
	reversetunnel.Server

	sites map[string]reversetunnel.RemoteSite
}

func (t mockRevTunnel) GetSite(name string) (reversetunnel.RemoteSite, error) {
	s, ok := t.sites[name]
	if !ok {
		return nil, trace.NotFound("remote site %q not found", name)
	}
	return s, nil
}

func (t mockRevTunnel) GetSites() []reversetunnel.RemoteSite {
	var sites []reversetunnel.RemoteSite
	for _, s := range t.sites {
		sites = append(sites, s)
	}
	return sites
}

type mockAuthorizer struct {
	ctx *auth.AuthContext
	err error
}

func (a mockAuthorizer) Authorize(context.Context) (*auth.AuthContext, error) {
	return a.ctx, a.err
}
