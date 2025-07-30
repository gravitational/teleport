// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dns

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/sync/errgroup"
	"gvisor.dev/gvisor/pkg/tcpip"

	"github.com/gravitational/teleport"
)

const (
	// This is the recommended EDNS maximum payload size in https://www.rfc-editor.org/rfc/rfc6891.txt
	// While this server doesn't directly support EDNS yet for queries that actually resolve to Teleport apps,
	// upstream nameservers may, and we shouldn't drop valid requests to those upstream servers.
	// This is not an absolute maximum for EDNS, but it is the usual maximum in practice, and the maximum
	// supported by bind https://github.com/isc-projects/bind9/blob/9357019498d57aef95fff94d408198e21dcc93c9/lib/dns/resolver.c#L238
	maxUDPDNSMessageSize = 4096

	// https://www.rfc-editor.org/rfc/rfc1123#page-77 recommends 5 seconds as a minimum, and this seems to be
	// common in practice.
	forwardRequestTimeout = 5 * time.Second
)

// Resolver represents an entity that can resolve DNS requests.
type Resolver interface {
	// ResolveA should return a Result for an A record question. If an empty Result is returned with no error,
	// the question will be forwarded upstream.
	ResolveA(ctx context.Context, domain string) (Result, error)

	// ResolveAAAA should return a Result for an AAAA record question. If an empty Result is returned with no
	// error, the question will be forwarded upstream.
	ResolveAAAA(ctx context.Context, domain string) (Result, error)
}

// Result holds the result of DNS resolution.
type Result struct {
	// A is an A record.
	A [4]byte
	// AAAA is an AAAA record.
	AAAA [16]byte
	// NXDomain indicates that the requested domain is invalid or unassigned, this is an authoritative answer.
	NXDomain bool
	// NoRecord indicates the domain exists but the requested record type doesn't, this is an authoritative
	// answer.
	NoRecord bool
}

// UpstreamNameserverSource provides the current set of upstream nameservers.
type UpstreamNameserverSource interface {
	// UpstreamNameservers should return the current set of upstream nameservers, requests that cannot be
	// resolved will be forwarded to these addresses.
	UpstreamNameservers(context.Context) ([]string, error)
}

// Server is a DNS server.
type Server struct {
	resolver                 Resolver
	upstreamNameserverSource UpstreamNameserverSource
	messageBuffers           sync.Pool
	slog                     *slog.Logger
}

// NewServer returns a DNS server that handles the details of the DNS protocol and asks [resolver] to answer
// DNS questions. If [resolver] has no answer, requests will be forwarded to the upstream nameservers provided
// by [upstreamNameserverSource].
func NewServer(resolver Resolver, upstreamNameserverSource UpstreamNameserverSource) (*Server, error) {
	return &Server{
		resolver:                 resolver,
		upstreamNameserverSource: upstreamNameserverSource,
		messageBuffers: sync.Pool{
			New: func() any {
				buf := make([]byte, maxUDPDNSMessageSize)
				return &buf
			},
		},
		slog: slog.With(teleport.ComponentKey, teleport.Component("vnet", "dns")),
	}, nil
}

// getMessageBuffer returns a buffer of size [maxUDPMessageSize]. Call [returnBuf] to return the buffer to the
// shared pool. Use this to avoid large allocations on all DNS messages.
func (s *Server) getMessageBuffer() (buf []byte, returnBuf func()) {
	buf = *s.messageBuffers.Get().(*[]byte)
	buf = buf[:cap(buf)]
	return buf, func() {
		s.messageBuffers.Put(&buf)
	}
}

// HandleUDP reads and handles a single UDP message from [conn] and writes the response back to [conn].
// This will be called by VNet code.
func (s *Server) HandleUDP(ctx context.Context, conn net.Conn) error {
	buf, returnBuf := s.getMessageBuffer()
	defer returnBuf()

	n, err := conn.Read(buf)
	if err != nil {
		return trace.Wrap(err, "failed to read from UDP conn")
	}
	if n >= maxUDPDNSMessageSize {
		return trace.BadParameter("Dropping UDP message that is too large")
	}
	buf = buf[:n]

	return trace.Wrap(s.handleDNSMessage(ctx, conn.RemoteAddr().String(), buf, conn))
}

// ListendAndServeUDP reads all incoming UDP messages from [conn], handles DNS questions, and writes the
// responses back to [conn].
// This is not called by VNet code and basically exists so we can test the resolver outside of VNet.
func (s *Server) ListenAndServeUDP(ctx context.Context, conn *net.UDPConn) error {
	buf, returnBuf := s.getMessageBuffer()
	defer returnBuf()

	for {
		buf = buf[:cap(buf)]
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return trace.Wrap(err, "failed to read from UDP conn")
		}
		if n >= maxUDPDNSMessageSize {
			return trace.BadParameter("Dropping UDP message that is too large")
		}
		buf = buf[:n]

		responseWriter := &udpWriter{
			conn:       conn,
			remoteAddr: remoteAddr,
		}
		if err := s.handleDNSMessage(ctx, remoteAddr.String(), buf, responseWriter); err != nil {
			s.slog.DebugContext(ctx, "Error handling DNS message.", "error", err)
		}
	}
}

type udpWriter struct {
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
}

func (u *udpWriter) Write(b []byte) (int, error) {
	n, _, err := u.conn.WriteMsgUDP(b, nil /*oob*/, u.remoteAddr)
	return n, err
}

// handleDNSMessage handles the DNS message held in [buf] and writes the answer to [responseWriter].
// This could handle DNS messages arriving over UDP or TCP.
func (s *Server) handleDNSMessage(ctx context.Context, remoteAddr string, buf []byte, responseWriter io.Writer) error {
	slog := s.slog.With("remote_addr", remoteAddr)
	slog.DebugContext(ctx, "Handling DNS message.")
	defer slog.DebugContext(ctx, "Done handling DNS message.")

	var parser dnsmessage.Parser
	requestHeader, err := parser.Start(buf)
	if err != nil {
		return trace.Wrap(err, "parsing DNS message")
	}
	if requestHeader.OpCode != 0 {
		slog.DebugContext(ctx, "OpCode is not QUERY (0), forwarding.", "opcode", requestHeader.OpCode)
		return trace.Wrap(s.forward(ctx, slog, buf, responseWriter), "forwarding non-Query DNS message")
	}
	question, err := parser.Question()
	if err != nil {
		return trace.Wrap(err, "parsing DNS question")
	}
	fqdn := question.Name.String()
	slog = slog.With("fqdn", fqdn, "type", question.Type.String())
	slog.DebugContext(ctx, "Received DNS question.", "question", question)
	if question.Class != dnsmessage.ClassINET {
		slog.DebugContext(ctx, "Query class is not INET, forwarding.", "class", question.Class)
		return trace.Wrap(s.forward(ctx, slog, buf, responseWriter), "forwarding non-INET DNS query")
	}

	var result Result
	switch question.Type {
	case dnsmessage.TypeA:
		result, err = s.resolver.ResolveA(ctx, fqdn)
		if err != nil {
			return trace.Wrap(err, "resolving A request for %q", fqdn)
		}
	case dnsmessage.TypeAAAA:
		result, err = s.resolver.ResolveAAAA(ctx, fqdn)
		if err != nil {
			return trace.Wrap(err, "resolving AAAA request for %q", fqdn)
		}
	default:
		slog.DebugContext(ctx, "Question type is not A or AAAA, forwarding.", "type", question.Type)
		return trace.Wrap(s.forward(ctx, slog, buf, responseWriter), "forwarding %s DNS query", question.Type)
	}

	var response []byte
	switch {
	case result.NXDomain:
		slog.DebugContext(ctx, "No match for name, responding with authoritative name error.")
		response, err = buildNXDomainResponse(buf, &requestHeader, &question)
	case result.NoRecord:
		slog.DebugContext(ctx, "Name matched but no record, responding with authoritative non-answer.")
		response, err = buildEmptyResponse(buf, &requestHeader, &question)
	case question.Type == dnsmessage.TypeA && result.A != ([4]byte{}):
		slog.DebugContext(ctx, "Matched DNS A.", "a", tcpip.AddrFrom4(result.A))
		response, err = buildAResponse(buf, &requestHeader, &question, result.A)
	case question.Type == dnsmessage.TypeAAAA && result.AAAA != ([16]byte{}):
		slog.DebugContext(ctx, "Matched DNS AAAA.", "aaaa", tcpip.AddrFrom16(result.AAAA))
		response, err = buildAAAAResponse(buf, &requestHeader, &question, result.AAAA)
	default:
		slog.DebugContext(ctx, "Forwarding unmatched query.")
		return trace.Wrap(s.forward(ctx, slog, buf, responseWriter), "forwarding unmatched DNS query")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = responseWriter.Write(response)
	return trace.Wrap(err, "writing DNS response")
}

// forward forwards a raw DNS message to all upstream nameservers and writes the first response to
// [responseWriter]. If there are no upstream nameservers, or none of them responds within the timeout, an
// error is returned. This doesn't do any retries because the downstream resolver is likely to do its own
// retries.
func (s *Server) forward(ctx context.Context, slog *slog.Logger, buf []byte, responseWriter io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, forwardRequestTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	upstreamNameservers, err := s.upstreamNameserverSource.UpstreamNameservers(ctx)
	if err != nil {
		return trace.Wrap(err, "getting host default nameservers")
	}
	if len(upstreamNameservers) == 0 {
		return trace.Errorf("no upstream nameservers")
	}

	// Forward the message to each upstream nameserver concurrently, the first to answer wins.
	// Each goroutine will write a single error or a single response to the appropriate channel.
	// Each goroutine should quickly exit after the context is canceled.
	responses := make(chan []byte, len(upstreamNameservers))
	errs := make(chan error, len(upstreamNameservers))
	g, ctx := errgroup.WithContext(ctx)
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	for _, nameserver := range upstreamNameservers {
		responseBuf, returnResponseBuf := s.getMessageBuffer()
		defer returnResponseBuf()

		nameserver := nameserver
		g.Go(func() error {
			slog := slog.With("nameserver", nameserver)
			slog.DebugContext(ctx, "Forwarding request to upstream nameserver.")

			upstreamConn, err := net.Dial("udp", nameserver)
			if err != nil {
				errs <- trace.Wrap(err, "dialing upstream nameserver")
				return nil
			}

			// Immediately close the upstream conn after the context is canceled to unblock any i/o. This
			// function will not return any answer until all errgroup goroutines have terminated.
			go func() {
				<-ctx.Done()
				upstreamConn.Close()
			}()

			upstreamConn.SetDeadline(deadline)
			_, err = upstreamConn.Write(buf)
			if err != nil {
				errs <- trace.Wrap(err, "writing message to upstream")
				return nil
			}
			n, err := upstreamConn.Read(responseBuf)
			if err != nil {
				errs <- trace.Wrap(err, "reading forwarded DNS response")
				return nil
			}
			if n == len(responseBuf) {
				errs <- fmt.Errorf("DNS response too large")
				return nil
			}
			// Cancel all other goroutines
			cancel()
			responses <- responseBuf[:n]
			return nil
		})
	}

	// Not using the errgroup err, errors were written to channel.
	_ = g.Wait()

	select {
	case firstResponse := <-responses:
		slog.DebugContext(ctx, "Got response to forwarded DNS query, responding to client.")
		_, err := responseWriter.Write(firstResponse)
		return trace.Wrap(err, "writing DNS response")
	default:
	}

	close(errs)
	return trace.Wrap(trace.NewAggregateFromChannel(errs, context.Background()), "no upstream answers")
}

func buildEmptyResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeSuccess)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	// TODO(nklaassen): TTL in SOA record?
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildNXDomainResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeNameError)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	// TODO(nklaassen): TTL in SOA record?
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildAResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question, addr [4]byte) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeSuccess)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	if err := responseBuilder.StartAnswers(); err != nil {
		return buf, trace.Wrap(err, "starting answers section of DNS response")
	}
	if err := responseBuilder.AResource(dnsmessage.ResourceHeader{
		Name:  question.Name,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
		TTL:   10,
	}, dnsmessage.AResource{A: addr}); err != nil {
		return buf, trace.Wrap(err, "adding AResource to DNS response")
	}
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildAAAAResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question, addr [16]byte) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeSuccess)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	if err := responseBuilder.StartAnswers(); err != nil {
		return buf, trace.Wrap(err, "starting answers section of DNS response")
	}
	if err := responseBuilder.AAAAResource(dnsmessage.ResourceHeader{
		Name:  question.Name,
		Type:  dnsmessage.TypeAAAA,
		Class: dnsmessage.ClassINET,
		TTL:   10,
	}, dnsmessage.AAAAResource{AAAA: addr}); err != nil {
		return buf, trace.Wrap(err, "adding AAAAResource to DNS response")
	}
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func prepDNSResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question, rcode dnsmessage.RCode) (*dnsmessage.Builder, error) {
	buf = buf[:0]
	responseBuilder := dnsmessage.NewBuilder(buf, dnsmessage.Header{
		ID:            requestHeader.ID,
		Response:      true,
		Authoritative: true,
		RCode:         dnsmessage.RCodeSuccess,
	})
	responseBuilder.EnableCompression()
	if err := responseBuilder.StartQuestions(); err != nil {
		return nil, trace.Wrap(err, "starting questions section of DNS response")
	}
	if err := responseBuilder.Question(*question); err != nil {
		return nil, trace.Wrap(err, "adding question to DNS response")
	}
	return &responseBuilder, nil
}
