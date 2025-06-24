package oracle

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/e/lib/db/oracle/connection"
	"github.com/gravitational/teleport/e/lib/db/oracle/logging"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/e/lib/db/oracle/tns"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
	"github.com/gravitational/teleport/lib/utils"
)

func (e *Engine) readConnect(clientConn *connection.OracleConn) (*protocol.ConnectPacket, error) {
	pkt, err := clientConn.ReadPacket()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connect, ok := pkt.(*protocol.ConnectPacket)
	if !ok {
		return nil, trace.BadParameter("expected connect packet, got %T", pkt)
	}

	// connect is a bit special: its size is limited by the protocol, but the connection string it contains is often above that limit.
	// if that happens, the client will follow it up with a DATA packet containing the connection string in question.
	err = connect.MaybeReadMoreData(clientConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connString, err := connect.GetConnectionString()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceName, err := connect.GetServiceName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.Log.InfoContext(e.Context, "Received connection string", "conn_string", connString, "service_name", serviceName)

	err = e.checkExpectedServiceName(serviceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.serviceName = serviceName

	if e.onConnectPacketRead != nil {
		e.onConnectPacketRead(connect)
	}

	return connect, nil
}

func (e *Engine) dialServerAndForward(ctx context.Context, sessionCtx *common.Session) error {
	packetLogger, err := logging.NewPacketLogger(ctx, sessionCtx, e.Log)
	if err != nil {
		return trace.Wrap(err)
	}
	defer packetLogger.Close()

	clientConn, err := connection.NewConn(e.clientConn,
		connection.WithOnReadHeader(func(header protocol.PacketHeader) { packetLogger.LogHeader(packetcapture.ClientToTeleport, header) }),
		connection.WithOnReadPacket(func(packet protocol.Packet) { packetLogger.LogPacket(packetcapture.ClientToTeleport, packet) }),
		connection.WithOnWritePacket(func(packet protocol.Packet) { packetLogger.LogPacket(packetcapture.TeleportToClient, packet) }),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	connectPacket, err := e.readConnect(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tweakConnectPacket(e.Context, e.Log, connectPacket)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note that client connection is already closed by the `e.clientConn.Close()` call in HandleConnection function.
	serverConn, err := e.openServerConnection(ctx, packetLogger, sessionCtx, clientConn, connectPacket)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	// perform auth. client and server flows are independent, so we can run them at the same time.
	errCh := make(chan error, 2)
	go func() {
		errClient := e.secureNetworkServicesClient(clientConn)
		if errClient != nil {
			e.Log.ErrorContext(e.Context, "Client negotiation failure", "error", errClient)
		}
		errCh <- errClient
	}()

	go func() {
		errServer := e.secureNetworkServicesServer(e.Context, serverConn)
		if errServer != nil {
			e.Log.ErrorContext(e.Context, "Server negotiation failure", "error", errServer)
		}
		errCh <- errServer
	}()

	err = trace.NewAggregate(<-errCh, <-errCh)
	if err != nil {
		return trace.Wrap(err)
	}

	err = e.forwardLoop(ctx, clientConn, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// tweakConnectPacket modifies connectPacket to constrain the requested protocol version and options to the supported subset.
func tweakConnectPacket(ctx context.Context, logger *slog.Logger, connectPacket *protocol.ConnectPacket) error {
	requestedProtocolVersion, err := connectPacket.GetProtocolVersion()
	if err == nil {
		logger.DebugContext(ctx, "Protocol version", "requested_protocol_version", requestedProtocolVersion)
	} else {
		logger.DebugContext(ctx, "Failed to get protocol version", "error", err)
	}

	const maxProtocolVersion = 317
	if requestedProtocolVersion > maxProtocolVersion {
		logger.DebugContext(ctx, "Lowering protocol version", "new_protocol_version", maxProtocolVersion, "requested_protocol_version", requestedProtocolVersion)
		err = connectPacket.SetProtocolVersion(maxProtocolVersion)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	opts, err := connectPacket.GetServiceOptions()
	if err != nil {
		return trace.Wrap(err)
	}

	// Disable "full duplex" service option, as having it enabled may cause differences in login flows that we don't want.
	// This corresponds to `DISABLE_OOB=off` (beware double negative) flag in client/server config.
	// There is no harm in disabling it:
	// - the feature is known to be problematic and is disabled by default in various configurations, including RDS Oracle.
	// - even if enabled on both client and server side, plenty of networks won't work with it anyway.
	if opts.HasFlag(protocol.ServiceOptionFullDuplex) {
		logger.DebugContext(ctx, "Found service option full duplex, disabling", "options", opts)
		opts = opts.WithFlagUnset(protocol.ServiceOptionFullDuplex)
		err = connectPacket.SetServiceOptions(opts)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// openServerConnection handles the first phase of the connection.
// The client declares the database it wishes to connect to and server accepts or refuses.
// Server may also request TLS renegotiation.
// Protocol version is negotiated, which impacts the binary message layout.
func (e *Engine) openServerConnection(ctx context.Context, packetLogger logging.PacketLogger, sessionCtx *common.Session, clientConn *connection.OracleConn, connectPacket *protocol.ConnectPacket) (*connection.OracleConn, error) {
	var serverTcpConn net.Conn
	closeServerTcpConn := true
	defer func() {
		if closeServerTcpConn && serverTcpConn != nil {
			_ = serverTcpConn.Close()
		}
	}()

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// server address to dial; initially set from database spec, may be overridden by data from redirect packet.
	serverDialAddr := getURI(sessionCtx.Database)

	// we allow maximum of 3 dial attempts.
	// - in regular, direct connection we will observe a single dial attempt.
	// - in SCAN, a single redirect to the direct server will happen, resulting in total two dial attempts.
	// - we will accept yet another redirect, although there are no known configurations that would behave like this.
	const maxDialAttempts = 3

	// handling the redirect packet requires continue against the outer loop.
dial:
	for dialAttempt := 1; ; dialAttempt++ {
		e.Log.DebugContext(e.Context, "Opening server connection", "attempt", dialAttempt, "address", serverDialAddr)
		if dialAttempt > maxDialAttempts {
			return nil, trace.BadParameter("exceeded max dial attempts")
		}

		serverTcpConn, err = net.DialTimeout("tcp", serverDialAddr, defaults.DefaultIOTimeout)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		e.Log.DebugContext(e.Context, "Connection opened.", "local_addr", serverTcpConn.LocalAddr(), "remote_addr", serverTcpConn.RemoteAddr())

		// CONNECT -> ACCEPT loop; may need to restart TLS and retry.
		// Typical happy flow:
		// - send CONNECT
		// - receive RESEND
		// - send CONNECT
		// - receive ACCEPT
		// We allow for more RESEND packets because handling that isn't hard and the protocol technically allows for that.
		const maxResendAttempts = 3
		for attempt := 1; ; attempt++ {
			if attempt > maxResendAttempts {
				return nil, trace.BadParameter("exceeded max resend attempts")
			}

			connectionOptions := []connection.ConnOption{
				connection.WithTLS(ctx, tlsConfig),
				connection.WithOnReadHeader(func(header protocol.PacketHeader) { packetLogger.LogHeader(packetcapture.ServerToTeleport, header) }),
				connection.WithOnReadPacket(func(packet protocol.Packet) { packetLogger.LogPacket(packetcapture.ServerToTeleport, packet) }),
				connection.WithOnWritePacket(func(packet protocol.Packet) { packetLogger.LogPacket(packetcapture.TeleportToServer, packet) }),
			}

			serverConn, err := connection.NewConn(serverTcpConn, connectionOptions...)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// pass the CONNECT (and optional DATA packet) down to the server.
			e.Log.InfoContext(e.Context, "Sending connect packet", "attempt", attempt)
			err = serverConn.WritePacket(connectPacket)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if connectPacket.DataPacket != nil {
				err = serverConn.WritePacket(connectPacket.DataPacket)
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}

			// read the response from server. expecting either RESEND or ACCEPT.
			pkt, err := serverConn.ReadPacket()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			switch pktT := pkt.(type) {
			case *protocol.AcceptPacket:
				accept := pktT
				e.Log.InfoContext(e.Context, "Received accept packet", "protocol_version", accept.ProtocolVersion)

				// update negotiated protocol version.
				clientConn.SetProtocolVersion(accept.ProtocolVersion)
				serverConn.SetProtocolVersion(accept.ProtocolVersion)

				// forward the accept packet to the client.
				err = clientConn.WritePacket(accept)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				e.Log.InfoContext(e.Context, "Protocol handshake complete.")

				closeServerTcpConn = false

				return serverConn, nil
			case *protocol.RefusePacket:
				e.Log.WarnContext(e.Context, "Received refuse packet.", "message", pktT.Message)

				// forward refuse packet to the client
				err = clientConn.WritePacket(pktT)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return nil, trace.AccessDenied("server refused connection: %s", pktT.Message)
			case *protocol.ResendPacket:
				e.Log.DebugContext(e.Context, "RESEND received, trying again.")
				continue
			case *protocol.RedirectPacket:
				e.Log.DebugContext(e.Context, "Received REDIRECT packet, processing.")
				redirect := pktT

				// similar to connect packet, redirect packet often needs more data which is sent in a follow-up DATA packet.
				err = redirect.MaybeReadMoreData(serverConn)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				redirectAddr, err := redirect.RedirectAddress()
				if err != nil {
					return nil, trace.Wrap(err, "failed to get redirect address")
				}
				e.Log.DebugContext(e.Context, "Redirect address", "addr", redirectAddr)

				redirectHost, redirectPort, err := parseRedirectAddress(e.Context, e.Log, redirectAddr)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				serverDialAddr = net.JoinHostPort(redirectHost, strconv.Itoa(redirectPort))

				redirectConnStr, err := redirect.RedirectConnectionString()
				if err != nil {
					return nil, trace.Wrap(err, "failed to get redirect connection string")
				}
				e.Log.DebugContext(e.Context, "Redirect connection string", "conn_str", redirectConnStr)

				// We need to update connect packet dropping old connection string in favor of the new one provided by the server.
				// However, the client shouldn't be bothered to provide an updated connect packet: it is unaware of the redirect happening at all.
				// We also cannot make the fresh packet from scratch, we need to keep the flags provided by the client intact.
				// We construct the new packet by updating the connection string of the original packet.
				newConnectPacket, err := connectPacket.WithConnectionString(redirectConnStr)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				connectPacket = newConnectPacket

				// close old TCP connection; ignore potential errors.
				_ = serverTcpConn.Close()

				// continue to new dial (outer loop)
				continue dial
			}

			e.Log.DebugContext(e.Context, "Received unexpected packet.", "type", fmt.Sprintf("%T", pkt))

			return nil, trace.BadParameter("received unexpected packet type: %T", pkt)
		}
	}
}

// parseRedirectAddress extracts connection address and port from redirect address, such as:
//
//	(ADDRESS=(PROTOCOL=TCPS)(HOST=10.0.0.52)(PORT=2452))
//
// or with an additional DESCRIPTION node:
//
//	(DESCRIPTION=(ADDRESS=(PROTOCOL=TCPS)(HOST=10.0.0.52)(PORT=2452)))
//
// TCPS protocol is enforced.
func parseRedirectAddress(ctx context.Context, logger *slog.Logger, addr string) (string, int, error) {
	// parse the address
	nodes, err := tns.ParseNodes(addr)
	if err != nil {
		return "", 0, trace.Wrap(err, "failed to parse redirect address %q", addr)
	}
	if len(nodes) > 1 {
		logger.DebugContext(ctx, "Parsed redirect address with more than one node", "addr", addr)
	}

	tree := &tns.Node{Children: nodes}

	const (
		descriptionKey = "DESCRIPTION"
		addressKey     = "ADDRESS"
		hostKey        = "HOST"
		portKey        = "PORT"
		protocolKey    = "PROTOCOL"
	)

	hostNode := tree.Path(addressKey, hostKey)
	port := tree.Path(addressKey, portKey).GetValue()
	proto := tree.Path(addressKey, protocolKey).GetValue()

	// if HOST node is missing, check within the DESCRIPTION node.
	if hostNode == nil {
		hostNode = tree.Path(descriptionKey, addressKey, hostKey)
		if hostNode == nil {
			return "", 0, trace.BadParameter("redirect address %q is missing a host key", addr)
		}
		port = tree.Path(descriptionKey, addressKey, portKey).GetValue()
		proto = tree.Path(descriptionKey, addressKey, protocolKey).GetValue()
	}
	host := hostNode.GetValue()

	logger.DebugContext(ctx, "Parsed redirect address", "host", host, "port", port, "protocol", proto)

	// validate
	if host == "" {
		return "", 0, trace.BadParameter("empty host value")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, trace.BadParameter("failed to parse port number: %q", port)
	}
	if !strings.EqualFold(proto, "tcps") {
		return "", 0, trace.BadParameter("expected TCPS protocol, got %q", proto)
	}

	return host, portNum, nil
}

// secureNetworkServicesClient performs SNS negotiation with the client.
// SNS can be used for advanced auth, native protocol encryption, checksums and more.
// We don't want any of that from the client: the protocol is wrapped in authenticated TLS, so all of those features are redundant.
// We ignore whatever the client requests and respond with "no services enabled".
func (e *Engine) secureNetworkServicesClient(clientConn *connection.OracleConn) error {
	snsPacket, err := readDataPacket(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	if !snsPacket.HasSecureNetworkServices() {
		return trace.BadParameter("expected SNS packet, got different one instead.")
	}

	// TODO: parse the SNS data packet and verify the fields.
	e.Log.DebugContext(e.Context, "SNS packet received.")

	err = writeDataPacket(clientConn, servicesResponseNoServices)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// secureNetworkServicesServer performs SNS negotiation with the server.
// Out of all "services" available in SNS, we only want auth.
// Depending on configuration, we either use Kerberos or TCPS (mTLS).
func (e *Engine) secureNetworkServicesServer(ctx context.Context, serverConn *connection.OracleConn) error {
	if e.useKerberosAuth() {
		return trace.Wrap(performKerberosAuth(ctx, e.Log, e.session.DatabaseUser, e.authenticateKerberos, serverConn))
	}
	return trace.Wrap(e.performTCPSAuth(serverConn))
}

func (e *Engine) performTCPSAuth(serverConn *connection.OracleConn) error {
	e.Log.DebugContext(e.Context, "Performing TCPS (mTLS) auth.")
	err := writeDataPacket(serverConn, servicesRequestTCPS)
	if err != nil {
		return trace.Wrap(err)
	}
	serverSNSResponse, err := readDataPacket(serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO: actually parse server response in full, instead of merely checking the flag.
	if !serverSNSResponse.HasSecureNetworkServices() {
		e.Log.WarnContext(e.Context, "Unexpected response to SNS packet. Upcoming protocol failure likely.")
	}
	return nil
}

// forwardLoop is a general proxying method, where majority of packets are processed.
// Mostly we don't care about their contents and simply pass everything as is,
// except for the initial handful of server packets which we check for session ID, required to start audit puller.
func (e *Engine) forwardLoop(ctx context.Context, clientConn, serverConn *connection.OracleConn) error {
	e.Log.DebugContext(e.Context, "Starting async proxying.")

	// connections are fully initialized. from now on, just forward all traffic between db client and server.
	errC := make(chan error, 2)
	go func() {
		log := e.Log.With("thread", "client -> server")

		for {
			p, err := clientConn.ReadPacket()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					log.DebugContext(e.Context, "error reading packet", "error", err)
				}
				errC <- trace.Wrap(err)
				return
			}

			err = serverConn.WritePacket(p)
			if err != nil {
				errC <- trace.Wrap(err)
				return
			}
		}
	}()

	// read data from server, pass it to the client.
	go func() {
		log := e.Log.With("thread", "server -> client")

		// expect to find session id in within handful of packets, abort the session if that doesn't happen.
		// based on the tests with real clients the quota could be as low as 5, but it is better to leave some breathing room.
		dataPacketQuota := 20

		needAuditPuller := e.startAuditPuller != nil

		for {
			p, err := serverConn.ReadPacket()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					log.DebugContext(e.Context, "error reading packet", "error", err)
				}
				errC <- trace.Wrap(err)
				return
			}

			if needAuditPuller {
				dataPacketQuota--
				if dataPacketQuota < 0 {
					err = trace.BadParameter("Unable to find session id in the initial packet exchange")
					e.Log.ErrorContext(e.Context, "Failed to locate valid session parameters.", "error", err)
					errC <- trace.Wrap(err)
				}

				dataPacket, ok := p.(*protocol.DataPacket)
				if ok && dataPacket.HasAuthParameters() {
					err = e.tryStartAuditPuller(dataPacket, dataPacketQuota)
					if err != nil {
						e.Log.ErrorContext(e.Context, "Failed to locate valid session parameters.", "error", err)
						errC <- trace.Wrap(err)
						return
					}
					needAuditPuller = false
				}
			}

			err = clientConn.WritePacket(p)
			if err != nil {
				errC <- trace.Wrap(err)
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		e.Log.DebugContext(e.Context, "Context done, stopping forwarding.")
		return nil
	case err := <-errC:
		if err == nil {
			e.Log.DebugContext(e.Context, "Exiting without error.")
		} else {
			if utils.IsOKNetworkError(err) {
				e.Log.DebugContext(e.Context, "Closed connection.")
			} else {
				e.Log.WarnContext(e.Context, "Closing connection due to an error.", "error", err)
			}
		}
		return trace.Wrap(err)
	}
}

func readDataPacket(conn *connection.OracleConn) (*protocol.DataPacket, error) {
	pkt, err := conn.ReadPacket()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dataPacket, ok := pkt.(*protocol.DataPacket)
	if !ok {
		return nil, trace.BadParameter("expected DataPacket, got %T", pkt)
	}

	return dataPacket, nil
}

func writeDataPacket(conn *connection.OracleConn, bytes []byte) error {
	dataPacket, err := protocol.DataPacketFromPayload(conn.LargeSDU(), bytes)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(conn.WritePacket(dataPacket))
}
