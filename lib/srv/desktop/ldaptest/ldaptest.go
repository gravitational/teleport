/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package ldaptest contains a fake LDAP server implementing just enough
// LDAP functionality in order to test desktop discovery scenarios.
package ldaptest

import (
	"crypto/tls"
	"net"
	"sync"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

// Server is a fake LDAP server that can respond to search requests.
type Server struct {
	Addr string

	mu          sync.Mutex
	resultsByDN map[string]result
	ln          net.Listener
}

// NewServer creates a new LDAP server and starts it.
func NewServer(tlsConfig *tls.Config) (*Server, error) {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, trace.Wrap(err, "opening listener")
	}

	tlsListener := tls.NewListener(listener, tlsConfig)

	s := &Server{
		Addr:        listener.Addr().String(),
		ln:          tlsListener,
		resultsByDN: make(map[string]result),
	}

	go func() {
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				return
			}
			go s.serveConn(conn)
		}
	}()

	return s, nil
}

// Close shuts down the server.
func (s *Server) Close() error {
	return s.ln.Close()
}

// SetEntries configures the server to report valid LDAP entries
// in response to a search query for a particular DN.
func (s *Server) SetEntries(baseDN string, entries ...*ldap.Entry) {
	s.mu.Lock()
	s.resultsByDN[baseDN] = result{entries: entries}
	s.mu.Unlock()
}

// SetErrorCode configures the server to report an LDAP result
// (typically an error) in response to a search query for a particular DN.
func (s *Server) SetErrorCode(baseDN string, code uint16, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := s.resultsByDN[baseDN]
	result.code = code
	result.message = message
	result.entries = nil
	s.resultsByDN[baseDN] = result
}

func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()

	for {
		p, err := ber.ReadPacket(conn)
		if err != nil {
			return
		}
		// We expect at least 2 children: a message ID and an operation.
		if len(p.Children) < 2 {
			return
		}

		messageID := p.Children[0].Value
		op := p.Children[1]

		// We only support LDAP searches.
		if op.Tag != ldap.ApplicationSearchRequest {
			return
		}

		baseDN, _ := op.Children[0].Value.(string)
		result := s.search(baseDN)
		if err := writeSearchResponse(conn, messageID, result); err != nil {
			return
		}
	}
}

func (s *Server) search(baseDN string) result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resultsByDN[baseDN]
}

func writeLDAPMessage(conn net.Conn, messageID any, payload *ber.Packet) error {
	msg := ber.NewSequence("LDAP Message")
	msg.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, messageID, "Message ID"))
	msg.AppendChild(payload)
	_, err := conn.Write(msg.Bytes())
	return err
}

func searchResultEntry(entry *ldap.Entry) *ber.Packet {
	packet := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ber.Tag(ldap.ApplicationSearchResultEntry), nil, "Search Result Entry")
	packet.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, entry.DN, "Object Name"))

	attrs := ber.NewSequence("Attributes")
	for _, attr := range entry.Attributes {
		partial := ber.NewSequence("Partial Attribute")
		partial.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attr.Name, "Attribute Name"))
		values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "Attribute Values")
		for _, value := range attr.Values {
			values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, value, "Attribute Value"))
		}
		partial.AppendChild(values)
		attrs.AppendChild(partial)
	}
	packet.AppendChild(attrs)
	return packet
}

func writeSearchResponse(conn net.Conn, messageID any, result result) error {
	for _, entry := range result.entries {
		if err := writeLDAPMessage(conn, messageID, searchResultEntry(entry)); err != nil {
			return trace.Wrap(err)
		}
	}

	// once all entries are written we must write a search result done
	done := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ber.Tag(ldap.ApplicationSearchResultDone), nil, "Search Result Done")
	done.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, result.code, "Result Code"))
	done.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "Matched DN"))
	done.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, result.message, "Diagnostic Message"))
	if err := writeLDAPMessage(conn, messageID, done); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type result struct {
	entries []*ldap.Entry
	code    uint16
	message string
}

// NewComputerEntry creates an LDAP entry simulating a computer.
func NewComputerEntry(name, dnSuffix string) *ldap.Entry {
	fullDN := "CN=" + name + "," + dnSuffix

	return ldap.NewEntry(fullDN, map[string][]string{
		"cn":                     {name},
		"description":            {"test host " + name},
		"distinguishedName":      {fullDN},
		"dNSHostName":            {name},
		"name":                   {name},
		"objectGUID":             {uuid.NewString()},
		"operatingSystem":        {"Windows Server 2025 Standard"},
		"operatingSystemVersion": {"10.0"},
		"primaryGroupID":         {"515"},
	})
}
