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
	"io"
	"log/slog"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"golang.org/x/net/dns/dnsmessage"
	"gvisor.dev/gvisor/pkg/tcpip"
)

const (
	maxMessageSize       = 512 // RFC1035
	maxConcurrentQueries = 4
)

type Server struct {
	slog           *slog.Logger
	messageBuffers chan []byte
	resolver       Resolver
}

func NewServer(slog *slog.Logger, resolver Resolver) *Server {
	messageBuffers := make(chan []byte, maxConcurrentQueries)
	for i := 0; i < maxConcurrentQueries; i++ {
		messageBuffers <- []byte{}
	}
	return &Server{
		messageBuffers: messageBuffers,
		resolver:       resolver,
		slog:           slog.With(trace.Component, "VNet.DNS"),
	}
}

func (s *Server) HandleUDPConn(ctx context.Context, conn io.ReadWriteCloser) error {
	// TODO: IPv6
	s.slog.Debug("Handling DNS.")
	defer conn.Close()

	buf := <-s.messageBuffers
	defer func() { s.messageBuffers <- buf }()
	if cap(buf) < maxMessageSize {
		buf = make([]byte, maxMessageSize)
	} else {
		buf = buf[:cap(buf)]
	}

	n, err := conn.Read(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if n >= maxMessageSize {
		return trace.BadParameter("message too large")
	}

	// debugDNS(buf)

	var parser dnsmessage.Parser
	requestHeader, err := parser.Start(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	question, err := parser.Question()
	s.slog.Debug("Received DNS question.", "question", question)
	if question.Class != dnsmessage.ClassINET {
		s.slog.Debug("Query class is not INET, not responding.", "class", question.Class)
		return nil
	}

	// Reset buf to use it for the response.
	buf = buf[:0]

	fqdn := question.Name.String()
	result, err := s.resolver.Resolve(ctx, fqdn)
	if err != nil {
		return trace.Wrap(err, "resolving DNS request for %s", fqdn)
	}

	if result.NXDomain {
		s.slog.Debug("No match for name, responding with authoritative name error.", "fqdn", fqdn)
		buf, err := buildDNSNXDomainResponse(buf, requestHeader, question)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = conn.Write(buf)
		return trace.Wrap(err, "writing DNS NXDOMAIN response")
	}

	if result.ForwardTo != (tcpip.Address{}) {
		// TODO: support custom domains with DNS forwarding
		return trace.NotImplemented("DNS forwarding not implemented")
	}

	// TODO: Support AAAA
	aRecord := &dnsmessage.AResource{result.IP.As4()}
	s.slog.Debug("Matched DNS question.", "fqdn", fqdn, "addr", aRecord.A)
	buf, err = buildDNSResponseWithAnswer(buf, requestHeader, question, aRecord)
	if err != nil {
		return trace.Wrap(err)
	}

	// debugDNS(buf)

	n, err = conn.Write(buf)
	if err != nil {
		return trace.Wrap(err, "writing DNS response")
	}
	return nil
}

func debugDNS(buf []byte) {
	cp := make([]byte, len(buf))
	copy(cp, buf)
	var parser dnsmessage.Parser
	header, err := parser.Start(cp)
	if err != nil {
		slog.Warn("Error parsing message header.", "err", err)
	}
	questions, err := parser.AllQuestions()
	if err != nil {
		slog.Warn("Error parsing message questions.", "err", err)
	}
	answers, err := parser.AllAnswers()
	if err != nil {
		slog.Warn("Error parsing message answers.", "err", err)
	}
	authorities, err := parser.AllAuthorities()
	if err != nil {
		slog.Warn("Error parsing message authorities.", "err", err)
	}
	additionals, err := parser.AllAdditionals()
	if err != nil {
		slog.Warn("Error parsing message additionals.", "err", err)
	}
	spew.Dump(dnsmessage.Message{
		Header:      header,
		Questions:   questions,
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	})
}

func buildDNSResponseWithoutAnswer(buf []byte, requestHeader dnsmessage.Header, question dnsmessage.Question) ([]byte, error) {
	responseBuilder := dnsmessage.NewBuilder(buf, dnsmessage.Header{
		ID:                 requestHeader.ID,
		OpCode:             requestHeader.OpCode,
		Response:           true,
		Authoritative:      true,
		Truncated:          false,
		RecursionDesired:   false,
		RecursionAvailable: false,
		RCode:              dnsmessage.RCodeSuccess,
	})
	responseBuilder.EnableCompression()
	if err := responseBuilder.StartQuestions(); err != nil {
		return buf, trace.Wrap(err, "starting questions section of DNS response")
	}
	if err := responseBuilder.Question(question); err != nil {
		return buf, trace.Wrap(err, "adding question to DNS response")
	}
	// TODO: TTL in SOA record?
	buf, err := responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildDNSNXDomainResponse(buf []byte, requestHeader dnsmessage.Header, question dnsmessage.Question) ([]byte, error) {
	responseBuilder := dnsmessage.NewBuilder(buf, dnsmessage.Header{
		ID:            requestHeader.ID,
		OpCode:        requestHeader.OpCode,
		Response:      true,
		Authoritative: true,
		RCode:         dnsmessage.RCodeNameError,
	})
	responseBuilder.EnableCompression()
	if err := responseBuilder.StartQuestions(); err != nil {
		return buf, trace.Wrap(err, "starting questions section of DNS response")
	}
	if err := responseBuilder.Question(question); err != nil {
		return buf, trace.Wrap(err, "adding question to DNS response")
	}
	// TODO: TTL in SOA record?
	buf, err := responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildDNSResponseWithAnswer(buf []byte, requestHeader dnsmessage.Header, question dnsmessage.Question, aRecord *dnsmessage.AResource) ([]byte, error) {
	responseBuilder := dnsmessage.NewBuilder(buf, dnsmessage.Header{
		ID:                 requestHeader.ID,
		OpCode:             requestHeader.OpCode,
		Response:           true,
		Authoritative:      true,
		Truncated:          false,
		RecursionDesired:   false,
		RecursionAvailable: false,
		RCode:              dnsmessage.RCodeSuccess,
	})
	responseBuilder.EnableCompression()
	if err := responseBuilder.StartQuestions(); err != nil {
		return buf, trace.Wrap(err, "starting questions section of DNS response")
	}
	if err := responseBuilder.Question(question); err != nil {
		return buf, trace.Wrap(err, "adding question to DNS response")
	}
	if err := responseBuilder.StartAnswers(); err != nil {
		return buf, trace.Wrap(err, "starting answers section of DNS response")
	}
	// TODO: IPv6
	if err := responseBuilder.AResource(dnsmessage.ResourceHeader{
		Name:  question.Name,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
		TTL:   60,
	}, *aRecord); err != nil {
		return buf, trace.Wrap(err, "adding AResource to DNS response")
	}
	buf, err := responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}
