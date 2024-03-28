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
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	maxMessageSize = 1500 // MTU
)

type Server struct {
	hostConfFile   string
	slog           *slog.Logger
	messageBuffers sync.Pool
	resolver       Resolver
	ttlCache       *utils.FnCache
}

func NewServer(slog *slog.Logger, resolver Resolver) (*Server, error) {
	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 10 * time.Second,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Server{
		hostConfFile: "/etc/resolv.conf",
		slog:         slog.With(teleport.ComponentKey, "VNet.DNS"),
		messageBuffers: sync.Pool{
			New: func() any {
				return make([]byte, maxMessageSize)
			},
		},
		resolver: resolver,
		ttlCache: ttlCache,
	}, nil
}

func (s *Server) getMessageBuffer() ([]byte, func()) {
	buf := s.messageBuffers.Get().([]byte)
	buf = buf[:cap(buf)]
	return buf, func() {
		s.messageBuffers.Put(buf)
	}
}

func (s *Server) HandleUDPConn(ctx context.Context, conn io.ReadWriteCloser) error {
	s.slog.Debug("Handling DNS.")
	defer conn.Close()

	buf, returnBuf := s.getMessageBuffer()
	defer returnBuf()

	n, err := conn.Read(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if n >= maxMessageSize {
		return trace.BadParameter("DNS message too large")
	}
	buf = buf[:n]

	// debugDNS(buf)

	var parser dnsmessage.Parser
	requestHeader, err := parser.Start(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if requestHeader.OpCode != 0 {
		s.slog.Debug("OpCode is not QUERY (0), forwarding.", "OpCode", requestHeader.OpCode)
		return trace.Wrap(s.forward(ctx, s.slog, conn, buf), "forwarding non-QUERY DNS message")
	}
	question, err := parser.Question()
	if err != nil {
		return trace.Wrap(err, "parsing DNS question")
	}
	fqdn := question.Name.String()
	slog := s.slog.With("fqdn", fqdn, "type", question.Type.String())
	slog.Info("Received DNS question.", "question", question)
	if question.Class != dnsmessage.ClassINET {
		slog.Debug("Query class is not INET, forwarding.", "class", question.Class)
		return trace.Wrap(s.forward(ctx, slog, conn, buf), "forwarding non-INET DNS query")
		return nil
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
	}

	var response []byte
	switch {
	case result.NXDomain:
		slog.Info("No match for name, responding with authoritative name error.")
		response, err = buildNXDomainResponse(buf, &requestHeader, &question)
	case result.NoRecord:
		slog.Info("Name matched but no record, responding with authoritative non-answer.")
		response, err = buildEmptyResponse(buf, &requestHeader, &question)
	case result.A != ([4]byte{}):
		slog.Info("Matched DNS A.", "A", result.A)
		response, err = buildAResponse(buf, &requestHeader, &question, result.A)
	case result.AAAA != ([16]byte{}):
		slog.Info("Matched DNS AAAA.", "AAAA", result.AAAA)
		response, err = buildAAAAResponse(buf, &requestHeader, &question, result.AAAA)
	default:
		slog.Info("Forwarding unmatched query.")
		return trace.Wrap(s.forward(ctx, slog, conn, buf), "forwarding unmatched DNS query")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// debugDNS(buf)
	_, err = conn.Write(response)
	return trace.Wrap(err, "writing DNS response")
}

func (s *Server) forward(ctx context.Context, logger *slog.Logger, downstreamConn io.ReadWriteCloser, buf []byte) error {
	deadline := time.Now().Add(5 * time.Second)
	dialer := net.Dialer{
		Deadline: deadline,
	}

	forwardingNameservers, err := s.forwardingNameservers(ctx)
	if err != nil {
		return trace.Wrap(err, "getting host default nameservers")
	}
	responses := make(chan []byte, len(forwardingNameservers))
	errs := make(chan error, len(forwardingNameservers))
	for _, addr := range forwardingNameservers {
		addr := addr
		responseBuf, returnResponseBuf := s.getMessageBuffer()
		defer returnResponseBuf()
		go func() {
			logger.With("addr", addr).Debug("Forwarding request to nameserver.")
			upstreamConn, err := dialer.DialContext(ctx, "udp", addr+":53")
			if err != nil {
				errs <- err
				return
			}
			upstreamConn.SetWriteDeadline(deadline)
			upstreamConn.SetReadDeadline(deadline)
			_, err = upstreamConn.Write(buf)
			if err != nil {
				errs <- err
				return
			}
			n, err := upstreamConn.Read(responseBuf)
			if err != nil {
				errs <- trace.Wrap(err, "Reading forwarded DNS response")
				return
			}
			if n == cap(responseBuf) {
				errs <- fmt.Errorf("DNS response too large")
				return
			}
			responses <- responseBuf
		}()
	}

	var allErrs []error
	for i := 0; i < len(forwardingNameservers); i++ {
		select {
		case err := <-errs:
			allErrs = append(allErrs, err)
		case resp := <-responses:
			logger.Debug("Writing response from forwarded request back to client.")
			_, err := downstreamConn.Write(resp)
			return trace.Wrap(err, "writing forwarded DNS response")
		}
	}
	return trace.NewAggregate(allErrs...)
}

func (s *Server) forwardingNameservers(ctx context.Context) ([]string, error) {
	return utils.FnCacheGet(ctx, s.ttlCache, "ns", func(ctx context.Context) ([]string, error) {
		f, err := os.Open(s.hostConfFile)
		if err != nil {
			return nil, trace.Wrap(err, "opening %s", s.hostConfFile)
		}
		defer f.Close()

		var nameservers []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "nameserver") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			nameservers = append(nameservers, fields[1])
		}
		s.slog.Debug("Loaded host default nameservers.", "nameservers", nameservers)
		return nameservers, nil
	})
}

var writeLock sync.Mutex

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
	writeLock.Lock()
	defer writeLock.Unlock()
	spew.Dump(dnsmessage.Message{
		Header:      header,
		Questions:   questions,
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	})
}

func buildEmptyResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeSuccess)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	// TODO: TTL in SOA record?
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func buildNXDomainResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question) ([]byte, error) {
	responseBuilder, err := prepDNSResponse(buf, requestHeader, question, dnsmessage.RCodeNameError)
	if err != nil {
		return buf, trace.Wrap(err)
	}
	// TODO: TTL in SOA record?
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
	}, dnsmessage.AResource{addr}); err != nil {
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
	}, dnsmessage.AAAAResource{addr}); err != nil {
		return buf, trace.Wrap(err, "adding AAAAResource to DNS response")
	}
	buf, err = responseBuilder.Finish()
	return buf, trace.Wrap(err, "serializing DNS response")
}

func prepDNSResponse(buf []byte, requestHeader *dnsmessage.Header, question *dnsmessage.Question, rcode dnsmessage.RCode) (*dnsmessage.Builder, error) {
	buf = buf[:0]
	responseBuilder := dnsmessage.NewBuilder(buf, dnsmessage.Header{
		ID:                 requestHeader.ID,
		Response:           true,
		Authoritative:      true,
		RecursionAvailable: true,
		RCode:              dnsmessage.RCodeSuccess,
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
