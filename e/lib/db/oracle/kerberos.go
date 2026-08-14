package oracle

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/gssapi"
	"github.com/jcmturner/gokrb5/v8/spnego"

	"github.com/gravitational/teleport/e/lib/db/oracle/connection"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos"
)

func (e *Engine) useKerberosAuth() bool {
	return e.session.Database.GetAD().Domain != ""
}

type authenticateFunc func(username string, params protocol.KerberosAuthParams) ([]byte, error)

func (e *Engine) authenticateKerberos(username string, params protocol.KerberosAuthParams) ([]byte, error) {
	provider := kerberos.NewClientProvider(e.AuthClient, e.Log)

	kClient, err := provider.GetKerberosClient(e.Context, e.session.Database.GetAD(), username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.Log.DebugContext(e.Context, "Obtained Kerberos client")

	spn := params.ServiceClass + "/" + params.ServerInstance
	e.Log.DebugContext(e.Context, "Requesting Kerberos service ticket", "spn", spn)
	ticket, key, err := kClient.GetServiceTicket(spn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.Log.DebugContext(e.Context, "Creating Kerberos token")
	token, err := spnego.NewKRB5TokenAPREQ(kClient, ticket, key, []int{gssapi.ContextFlagMutual}, []int{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.Log.DebugContext(e.Context, "Successfully created Kerberos token")
	return token.APReq.Marshal()
}

func verifyExpectedKerberosServices(ctx context.Context, log *slog.Logger, incomingPacket *protocol.DataPacket) error {
	actualBytes, err := incomingPacket.DataPayload()
	if err != nil {
		return trace.Wrap(err)
	}

	switch {
	case matchServicesResponseKerberosExpected(actualBytes):
		return nil
	case incomingPacket.HasSecureNetworkServices() && strings.Contains(string(actualBytes), "KERBEROS5"):
		// this is a loose match; make a debug note about this fact.
		log.DebugContext(ctx, "Found loose Kerberos service match.")
		return nil
	default:
		// without KERBEROS5 in the message server will not accept Kerberos auth.
		log.DebugContext(ctx, "Kerberos service request: unexpected server reply")
		return trace.BadParameter("Kerberos service request: unexpected server reply")
	}
}

// performKerberosAuth performs the Kerberos authentication flow against Oracle server.
func performKerberosAuth(ctx context.Context, log *slog.Logger, username string, authenticate authenticateFunc, serverConn *connection.OracleConn) error {
	log.DebugContext(ctx, "Performing Kerberos auth.")
	// send initial packet, requesting Kerberos auth with other services disabled.
	err := writeDataPacket(serverConn, servicesRequestKerberos)
	if err != nil {
		return trace.Wrap(err)
	}

	// receive reply; we expect very specific response that acknowledges our choice of services:
	// - Kerberos enabled
	// - everything else disabled
	incomingPacket, err := readDataPacket(serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = verifyExpectedKerberosServices(ctx, log, incomingPacket)
	if err != nil {
		return trace.Wrap(err)
	}

	// ack services, continue to the next phase
	err = writeDataPacket(serverConn, acknowledgeServicesKerberos)
	if err != nil {
		return trace.Wrap(err)
	}

	// receive another reply; this will contain details of Kerberos ticket that we should request
	incomingPacket, err = readDataPacket(serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	ticketParams, err := protocol.ParseKerberosAuthParams(incomingPacket)
	if err != nil {
		return trace.Wrap(err)
	}
	token, err := authenticate(username, *ticketParams)
	if err != nil {
		return trace.Wrap(err)
	}
	payload, err := protocol.BuildKerberosTokenPayload(token)
	if err != nil {
		return trace.Wrap(err)
	}
	err = writeDataPacket(serverConn, payload)
	if err != nil {
		return trace.Wrap(err)
	}

	// final exchange; if everything is fine the server will return confirmation without an error.
	incomingPacket, err = readDataPacket(serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = protocol.VerifySNSPacket(incomingPacket)
	if err != nil {
		return trace.Wrap(err)
	}

	// we acknowledge it and the negotiation is finished.
	err = writeDataPacket(serverConn, acknowledgeFinalKerberos)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
