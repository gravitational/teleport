package common

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/services"
)

func TestAuthSignKubeconfig(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "auth_command_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	ca := services.NewCertAuthority(
		services.HostCA,
		"example.com",
		nil,
		[][]byte{[]byte("SSH CA cert")},
		nil,
		services.CertAuthoritySpecV2_RSA_SHA2_512,
	)
	ca.SetTLSKeyPairs([]services.TLSKeyPair{{Cert: []byte("TLS CA cert")}})

	client := mockClient{
		clusterName: clusterName,
		userCerts: &proto.Certs{
			SSH: []byte("SSH cert"),
			TLS: []byte("TLS cert"),
		},
		cas: []services.CertAuthority{ca},
	}
	ac := &AuthCommand{
		output:       filepath.Join(tmpDir, "kubeconfig"),
		outputFormat: identityfile.FormatKubernetes,
		proxyAddr:    "proxy.example.com",
	}

	// Generate kubeconfig.
	if err = ac.generateUserKeys(client); err != nil {
		t.Fatalf("generating kubeconfig: %v", err)
	}

	// Validate kubeconfig contents.
	kc, err := kubeconfig.Load(ac.output)
	if err != nil {
		t.Fatalf("loading generated kubeconfig: %v", err)
	}
	gotCert := kc.AuthInfos[kc.CurrentContext].ClientCertificateData
	if !bytes.Equal(gotCert, client.userCerts.TLS) {
		t.Errorf("got client cert: %q, want %q", gotCert, client.userCerts.TLS)
	}
	gotCA := kc.Clusters[kc.CurrentContext].CertificateAuthorityData
	wantCA := ca.GetTLSKeyPairs()[0].Cert
	if !bytes.Equal(gotCA, wantCA) {
		t.Errorf("got CA cert: %q, want %q", gotCA, wantCA)
	}
	gotServerAddr := kc.Clusters[kc.CurrentContext].Server
	if gotServerAddr != ac.proxyAddr {
		t.Errorf("got server address: %q, want %q", gotServerAddr, ac.proxyAddr)
	}
}

type mockClient struct {
	auth.ClientI

	clusterName services.ClusterName
	userCerts   *proto.Certs
	cas         []services.CertAuthority
}

func (c mockClient) GetClusterName(...services.MarshalOption) (services.ClusterName, error) {
	return c.clusterName, nil
}
func (c mockClient) GenerateUserCerts(context.Context, proto.UserCertsRequest) (*proto.Certs, error) {
	return c.userCerts, nil
}
func (c mockClient) GetCertAuthorities(services.CertAuthType, bool, ...services.MarshalOption) ([]services.CertAuthority, error) {
	return c.cas, nil
}
