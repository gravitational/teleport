package joincerts

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

// HostCertsParams is the set of parameters used to generate host certificates
// when a host joins the cluster.
type HostCertsParams struct {
	// HostID is the unique ID of the host.
	HostID string
	// HostName is a user-friendly host name.
	HostName string
	// SystemRole is the main system role requested, e.g. Instance, Node, Proxy, etc.
	SystemRole types.SystemRole
	// AuthenticatedSystemRoles is a set of system roles that the Instance
	// identity currently re-joining has authenticated.
	AuthenticatedSystemRoles types.SystemRoles
	// PublicTLSKey is the requested TLS public key in PEM-encoded PKIX DER format.
	PublicTLSKey []byte
	// PublicSSHKey is the requested SSH public key in SSH authorized keys format.
	PublicSSHKey []byte
	// AdditionalPrincipals is a list of additional principals
	// to include in OpenSSH and X509 certificates
	AdditionalPrincipals []string
	// DNSNames is a list of DNS names to include in x509 certificates.
	DNSNames []string
	// RemoteAddr is the remote address of the host requesting a host certificate.
	RemoteAddr string
	// RawJoinClaims are raw claims asserted by specific join methods.
	RawJoinClaims any
}

// BotCertsParams is the set of parameters used to generate bot certificates
// when a bot joins the cluster.
type BotCertsParams struct {
	// PublicTLSKey is the requested TLS public key in PEM-encoded PKIX DER format.
	PublicTLSKey []byte
	// PublicSSHKey is the requested SSH public key in SSH authorized keys format.
	PublicSSHKey []byte
	// BotInstanceID is a trusted instance identifier for a Machine ID bot,
	// provided to Auth by the Join Service when bots rejoin via a client
	// certificate extension.
	BotInstanceID string
	// PreviousBotInstanceID is a trusted previous instance identifier for a
	// Machine ID bot.
	PreviousBotInstanceID string
	// BotGeneration is a trusted generation counter value for Machine ID bots,
	// provided to Auth by the Join Service when bots rejoin via a client
	// certificate extension.
	BotGeneration int32
	// Expires is a desired time of the expiry of user certificates. This only
	// applies to bot joining, and will be ignored by node joining.
	Expires *time.Time
	// RemoteAddr is the remote address of the bot requesting a bot certificate.
	RemoteAddr string
	// RawJoinClaims are raw claims asserted by specific join methods.
	RawJoinClaims any
	// Attrs is a collection of attributes that result from the join process.
	Attrs *workloadidentityv1pb.JoinAttrs
}

// MakeHostCertsParams returns [HostCertsParams] populated by the ClientInit
// message and context of the request.
func MakeHostCertsParams(
	ctx context.Context,
	diag *diagnostic.Diagnostic,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	joinMethod types.JoinMethod,
) (*HostCertsParams, error) {
	// GenerateHostCertsForJoin requires the TLS key to be PEM-encoded.
	tlsPub, err := x509.ParsePKIXPublicKey(clientInit.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPubPEM, err := keys.MarshalPublicKey(crypto.PublicKey(tlsPub))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// GenerateHostCertsForJoin requires the SSH key to be in authorized keys format.
	sshPub, err := ssh.ParsePublicKey(clientInit.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshAuthorizedKey := ssh.MarshalAuthorizedKey(sshPub)

	params := &HostCertsParams{
		SystemRole:   types.SystemRole(clientInit.SystemRole),
		PublicTLSKey: tlsPubPEM,
		PublicSSHKey: sshAuthorizedKey,
	}

	if hostParams := clientInit.HostParams; hostParams != nil {
		params.HostName = hostParams.HostName
		params.AdditionalPrincipals = hostParams.AdditionalPrincipals
		params.DNSNames = hostParams.DNSNames
	}

	if authCtx.IsInstance {
		// Only authenticated Instance certs are allowed to re-join and
		// maintain their existing host ID and authenticate additional system
		// roles.
		params.HostID = authCtx.HostID
		params.AuthenticatedSystemRoles = authCtx.SystemRoles
	} else {
		// Generate a new host ID to assign to the client.
		hostID, err := hostid.Generate(ctx, joinMethod)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		params.HostID = hostID
	}

	// Trust the remote address as forwarded by the proxy, or else use the one
	// we get from the connection context.
	if authCtx.IsForwardedByProxy && clientInit.ProxySuppliedParams != nil {
		params.RemoteAddr = clientInit.ProxySuppliedParams.RemoteAddr
	} else {
		// This gets set on the diagnostic by the gRPC layer.
		params.RemoteAddr = diag.Get().RemoteAddr
	}

	return params, nil
}

// MakeHostCertsParams returns [BotCertsParams] populated by the ClientInit
// message and context of the request.
func MakeBotCertsParams(
	diag *diagnostic.Diagnostic,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
) (*BotCertsParams, error) {
	// GenerateBotCertsForJoin requires the TLS key to be PEM-encoded.
	tlsPub, err := x509.ParsePKIXPublicKey(clientInit.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPubPEM, err := keys.MarshalPublicKey(crypto.PublicKey(tlsPub))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// GenerateBotCertsForJoin requires the SSH key to be in authorized keys format.
	sshPub, err := ssh.ParsePublicKey(clientInit.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshAuthorizedKey := ssh.MarshalAuthorizedKey(sshPub)

	params := &BotCertsParams{
		PublicTLSKey:  tlsPubPEM,
		PublicSSHKey:  sshAuthorizedKey,
		BotInstanceID: authCtx.BotInstanceID,
	}

	if botParams := clientInit.BotParams; botParams != nil {
		params.BotGeneration = int32(authCtx.BotGeneration)
		params.BotInstanceID = authCtx.BotInstanceID
		params.Expires = botParams.Expires
	}

	// Trust the remote address as forwarded by the proxy, or else use the one
	// we get from the connection context.
	if authCtx.IsForwardedByProxy && clientInit.ProxySuppliedParams != nil {
		params.RemoteAddr = clientInit.ProxySuppliedParams.RemoteAddr
	} else {
		// This gets set on the diagnostic by the gRPC layer.
		params.RemoteAddr = diag.Get().RemoteAddr
	}

	return params, nil
}

// MakeResultMessage returns a [*messages.Result] populated from [*proto.Certs]
// with the certs converted into the proper wire format.
func MakeResultMessage(certs *proto.Certs, hostID *string) (*messages.Result, error) {
	sshCert, err := rawSSHCert(certs.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// certs.SSHCACerts is a misnomer, SSH CAs are just public keys, not certificates.
	sshCAKeys, err := rawSSHPublicKeys(certs.SSHCACerts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.Result{
		TLSCert:    rawTLSCert(certs.TLS),
		TLSCACerts: rawTLSCerts(certs.TLSCACerts),
		SSHCert:    sshCert,
		SSHCAKeys:  sshCAKeys,
		HostID:     hostID,
	}, nil
}

// rawTLSCerts converts a slice of PEM-encoded TLS certificates to the raw ASN.1
// DER form as required by [Result].
func rawTLSCerts(pemBytes [][]byte) [][]byte {
	out := make([][]byte, len(pemBytes))
	for i, bytes := range pemBytes {
		out[i] = rawTLSCert(bytes)
	}
	return out
}

// rawTLSCert converts a PEM-encoded TLS certificate to the raw ASN.1 DER form
// as required by [Result].
func rawTLSCert(pemBytes []byte) []byte {
	pemBlock, _ := pem.Decode(pemBytes)
	return pemBlock.Bytes
}

// rawSSHCert converts an SSH certificate or public key in SSH authorized_keys
// format to the SSH wire format as required by [Result].
func rawSSHCert(authorizedKey []byte) ([]byte, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pub.Marshal(), nil
}

// rawSSHPublicKeys converts a slices of SSH public keys in SSH authorized_keys
// format to the SSH wire format as required by [Result].
func rawSSHPublicKeys(authorizedKeys [][]byte) ([][]byte, error) {
	out := make([][]byte, len(authorizedKeys))
	for i, authorizedKey := range authorizedKeys {
		var err error
		out[i], err = rawSSHCert(authorizedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return out, nil
}
