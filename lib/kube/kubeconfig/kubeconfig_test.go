package kubeconfig

import (
	"crypto/x509/pkix"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeconfig(t *testing.T) { check.TestingT(t) }

type KubeconfigSuite struct {
	kubeconfigPath string
	initialConfig  clientcmdapi.Config
}

var _ = check.Suite(&KubeconfigSuite{})

func (s *KubeconfigSuite) SetUpTest(c *check.C) {
	f, err := ioutil.TempFile("", "kubeconfig")
	if err != nil {
		c.Fatalf("failed to create temp kubeconfig file: %v", err)
	}
	defer f.Close()

	// Note: LocationOfOrigin and Extensions would be automatically added on
	// clientcmd.Write below. Set them explicitly so we can compare
	// s.initialConfig against loaded config.
	//
	// TODO: use a comparison library that can ignore individual fields.
	s.initialConfig = clientcmdapi.Config{
		CurrentContext: "dev",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster-1": {
				CertificateAuthority: "fake-ca-file",
				Server:               "https://1.2.3.4",
				LocationOfOrigin:     f.Name(),
				Extensions:           map[string]runtime.Object{},
			},
			"cluster-2": {
				InsecureSkipTLSVerify: true,
				Server:                "https://1.2.3.5",
				LocationOfOrigin:      f.Name(),
				Extensions:            map[string]runtime.Object{},
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"developer": {
				ClientCertificate: "fake-client-cert",
				ClientKey:         "fake-client-key",
				LocationOfOrigin:  f.Name(),
				Extensions:        map[string]runtime.Object{},
			},
			"admin": {
				Username:         "admin",
				Password:         "hunter1",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
			"support": {
				Exec: &clientcmdapi.ExecConfig{
					Command: "/bin/get_creds",
					Args:    []string{"--role=support"},
				},
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"dev": {
				Cluster:          "cluster-2",
				AuthInfo:         "developer",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
			"prod": {
				Cluster:          "cluster-1",
				AuthInfo:         "admin",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
		},
		Preferences: clientcmdapi.Preferences{
			Extensions: map[string]runtime.Object{},
		},
		Extensions: map[string]runtime.Object{},
	}

	initialContent, err := clientcmd.Write(s.initialConfig)
	c.Assert(err, check.IsNil)

	if _, err := f.Write(initialContent); err != nil {
		c.Fatalf("failed to write kubeconfig: %v", err)
	}

	s.kubeconfigPath = f.Name()
}

func (s *KubeconfigSuite) TearDownTest(c *check.C) {
	os.Remove(s.kubeconfigPath)
}

func (s *KubeconfigSuite) TestLoad(c *check.C) {
	config, err := Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	c.Assert(*config, check.DeepEquals, s.initialConfig)
}

func (s *KubeconfigSuite) TestSave(c *check.C) {
	cfg := clientcmdapi.Config{
		CurrentContext: "a",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				CertificateAuthority: "fake-ca-file",
				Server:               "https://1.2.3.4",
				LocationOfOrigin:     s.kubeconfigPath,
				Extensions:           map[string]runtime.Object{},
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				LocationOfOrigin: s.kubeconfigPath,
				Extensions:       map[string]runtime.Object{},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"a": {
				Cluster:          "cluster",
				AuthInfo:         "user",
				LocationOfOrigin: s.kubeconfigPath,
				Extensions:       map[string]runtime.Object{},
			},
		},
		Preferences: clientcmdapi.Preferences{
			Extensions: map[string]runtime.Object{},
		},
		Extensions: map[string]runtime.Object{},
	}

	err := Save(s.kubeconfigPath, cfg)
	c.Assert(err, check.IsNil)

	config, err := Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	c.Assert(*config, check.DeepEquals, cfg)
}

func (s *KubeconfigSuite) TestUpdate(c *check.C) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
	)
	creds, caCertPEM, err := s.genUserKey()
	c.Assert(err, check.IsNil)
	err = Update(s.kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	})
	c.Assert(err, check.IsNil)

	wantConfig := s.initialConfig.DeepCopy()
	wantConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   clusterAddr,
		CertificateAuthorityData: caCertPEM,
		LocationOfOrigin:         s.kubeconfigPath,
		Extensions:               map[string]runtime.Object{},
	}
	wantConfig.AuthInfos[clusterName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: creds.TLSCert,
		ClientKeyData:         creds.Priv,
		LocationOfOrigin:      s.kubeconfigPath,
		Extensions:            map[string]runtime.Object{},
	}
	wantConfig.Contexts[clusterName] = &clientcmdapi.Context{
		Cluster:          clusterName,
		AuthInfo:         clusterName,
		LocationOfOrigin: s.kubeconfigPath,
		Extensions:       map[string]runtime.Object{},
	}
	wantConfig.CurrentContext = clusterName

	config, err := Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	c.Assert(config, check.DeepEquals, wantConfig)
}

func (s *KubeconfigSuite) TestRemove(c *check.C) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
	)
	creds, _, err := s.genUserKey()
	c.Assert(err, check.IsNil)

	// Add teleport-generated entries to kubeconfig.
	err = Update(s.kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	})
	c.Assert(err, check.IsNil)

	// Remove those generated entries from kubeconfig.
	err = Remove(s.kubeconfigPath, clusterName)
	c.Assert(err, check.IsNil)

	// Verify that kubeconfig changed back to the initial state.
	wantConfig := s.initialConfig.DeepCopy()
	config, err := Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	// CurrentContext can end up as either of the remaining contexts, as long
	// as it's not the one we just removed.
	c.Assert(config.CurrentContext, check.Not(check.Equals), clusterName)
	wantConfig.CurrentContext = config.CurrentContext
	c.Assert(config, check.DeepEquals, wantConfig)

	// Add teleport-generated entries to kubeconfig again.
	err = Update(s.kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	})
	c.Assert(err, check.IsNil)

	// This time, explicitly switch CurrentContext to "prod".
	// Remove should preserve this CurrentContext!
	config, err = Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	config.CurrentContext = "prod"
	err = Save(s.kubeconfigPath, *config)
	c.Assert(err, check.IsNil)

	// Remove teleport-generated entries from kubeconfig.
	err = Remove(s.kubeconfigPath, clusterName)
	c.Assert(err, check.IsNil)

	wantConfig = s.initialConfig.DeepCopy()
	// CurrentContext should always end up as "prod" because we explicitly set
	// it above and Remove shouldn't touch it unless it matches the cluster
	// being removed.
	wantConfig.CurrentContext = "prod"
	config, err = Load(s.kubeconfigPath)
	c.Assert(err, check.IsNil)
	c.Assert(config, check.DeepEquals, wantConfig)
}

func (s *KubeconfigSuite) genUserKey() (*client.Key, []byte, error) {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ca, err := tlsca.FromKeys(caCert, caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	keygen := testauthority.New()
	priv, pub, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cryptoPub, err := sshutils.CryptoPublicKey(pub)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clock := clockwork.NewRealClock()
	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPub,
		Subject: pkix.Name{
			CommonName: "teleport-user",
		},
		NotAfter: clock.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &client.Key{
		Priv:    priv,
		Pub:     pub,
		TLSCert: tlsCert,
		TrustedCA: []server.TrustedCerts{{
			TLSCertificates: [][]byte{caCert},
		}},
	}, caCert, nil
}
