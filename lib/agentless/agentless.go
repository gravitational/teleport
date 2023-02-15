package agentless

import (
	"context"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/trace"
)

type SiteClientGetter interface {
	GetSiteClient(ctx context.Context, clusterName string) (auth.ClientI, error)
}

func ConfigureAgent(ctx context.Context, username, clusterName string, clientGetter SiteClientGetter, agentGetter teleagent.Getter) (teleagent.Agent, error) {
	// generate a new key pair
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sign new public key with OpenSSH CA
	client, err := clientGetter.GetSiteClient(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certBytes, err := client.GenerateOpenSSHCert(ctx, auth.OpenSSHCertRequest{
		Username:  username,
		PublicKey: priv.MarshalSSHPublicKey(),
		TTL:       time.Hour,
		Cluster:   clusterName,
		// TODO: ClientIP?
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// parse returned certificate bytes
	k, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not an SSH certificate")
	}

	// ensure this is the only key added to the agent
	tagent, err := agentGetter()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := tagent.RemoveAll(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := tagent.Add(agent.AddedKey{
		PrivateKey:  priv.Signer,
		Certificate: cert,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return tagent, nil
}
