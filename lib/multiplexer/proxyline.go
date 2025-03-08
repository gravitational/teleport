/**
 *  Copyright 2013 Rackspace
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 *  Note: original copyright is preserved on purpose
 */

package multiplexer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"unsafe"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
)

// PP2Type is the PROXY protocol v2 TLV type
type PP2Type byte

type PP2TeleportSubtype PP2Type

const (
	// TCP4 is TCP over IPv4
	TCP4 = "TCP4"
	// TCP6 is tCP over IPv6
	TCP6 = "TCP6"
	// Unknown is unsupported or unknown protocol
	UNKNOWN = "UNKNOWN"

	PP2TypeNOOP PP2Type = 0x04 // No-op used for padding

	// Known custom types, spec allows to use 0xE0 - 0xEF for custom types
	PP2TypeGCP   PP2Type = 0xE0 // https://cloud.google.com/vpc/docs/configure-private-service-connect-producer
	PP2TypeAWS   PP2Type = 0xEA // https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-target-groups.html
	PP2TypeAzure PP2Type = 0xEE // https://learn.microsoft.com/en-us/azure/private-link/private-link-service-overview

	PP2TypeTeleport PP2Type = 0xE4 // Teleport own type for transferring our custom data such as connection metadata

	PP2TeleportSubtypeSigningCert  PP2TeleportSubtype = 0x01 // Certificate used to sign JWT
	PP2TeleportSubtypeJWT          PP2TeleportSubtype = 0x02 // JWT used to verify information sent in plain PROXY header
	PP2TeleportSubtypeOriginalAddr PP2TeleportSubtype = 0x03 // Original IPv6 source address when downgrading to IPv4
)

var (
	proxyCRLF = "\r\n"
	proxySep  = " "

	// ErrTruncatedTLV is returned when there's no enough bytes to read full TLV
	ErrTruncatedTLV = errors.New("TLV value was truncated")

	// ErrNoSignature is returned when proxy line doesn't have full required data (JWT and cert) for verification
	ErrNoSignature = errors.New("could not find signature data on the proxy line")
	// ErrBadCACert is returned when a HostCA cert could not successfully be added to roots for signing certificate verification
	ErrBadCACert = errors.New("could not add host CA to roots for verification")
	// ErrIncorrectRole is returned when signing cert doesn't have required system role (Proxy)
	ErrIncorrectRole = errors.New("could not find required system role on the signing certificate")
	// ErrNonLocalCluster is returned when we received signed PROXY header, which signing certificate is from remote cluster.
	ErrNonLocalCluster = errors.New("signing certificate is not signed by local cluster CA")
	// ErrNoHostCA is returned when CAGetter could not get host CA, for example if auth server is not available
	ErrNoHostCA          = errors.New("could not get specified host CA to verify signed PROXY header")
	ErrInvalidPseudoIPv4 = errors.New("mismatched pseudo IPv4 source and original IPv6 in proxy line")
)

// ProxyLine implements PROXY protocol version 1 and 2
// Spec: https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
// Original implementation here: https://github.com/racker/go-proxy-protocol
// TLV: https://github.com/pires/go-proxyproto
type ProxyLine struct {
	Protocol    string
	Source      net.TCPAddr
	Destination net.TCPAddr
	TLVs        []TLV // PROXY protocol extensions
	IsVerified  bool
}

// TLV (Type-Length-Value) is an extension mechanism in PROXY protocol v2, see end of section 2.2
type TLV struct {
	Type  PP2Type
	Value []byte
}

// String returns on-the wire string representation of the proxy line
func (p *ProxyLine) String() string {
	return fmt.Sprintf("PROXY %s %s %s %d %d\r\n", p.Protocol, p.Source.IP.String(), p.Destination.IP.String(), p.Source.Port, p.Destination.Port)
}

// Bytes returns on-the wire bytes representation of proxy line conforming to the proxy v2 protocol
func (p *ProxyLine) Bytes() ([]byte, error) {
	b := &bytes.Buffer{}
	header := proxyV2Header{VersionCommand: (Version2 << 4) | ProxyCommand}
	copy(header.Signature[:], ProxyV2Prefix)
	var addr interface{}
	if p.Source.Port < 0 || p.Destination.Port < 0 ||
		p.Source.Port > math.MaxUint16 || p.Destination.Port > math.MaxUint16 {
		return nil, trace.BadParameter("source or destination port (%q,%q) is out of range 0-65535", p.Source.Port, p.Destination.Port)
	}
	switch p.Protocol {
	case TCP4:
		header.Protocol = ProtocolTCP4
		addr4 := proxyV2Address4{
			SourcePort:      uint16(p.Source.Port),
			DestinationPort: uint16(p.Destination.Port),
		}
		sourceIPv4 := p.Source.IP.To4()
		if sourceIPv4 == nil {
			return nil, trace.BadParameter("could not get source IPv4 address representation from %q", p.Source.IP.String())
		}
		copy(addr4.Source[:], sourceIPv4)
		destIPv4 := p.Destination.IP.To4()
		if destIPv4 == nil {
			return nil, trace.BadParameter("could not get destination IPv4 address representation from %q", p.Destination.IP.String())
		}
		copy(addr4.Destination[:], destIPv4)
		addr = addr4
	case TCP6:
		header.Protocol = ProtocolTCP6
		addr6 := proxyV2Address6{
			SourcePort:      uint16(p.Source.Port),
			DestinationPort: uint16(p.Destination.Port),
		}
		sourceIPv6 := p.Source.IP.To16()
		if sourceIPv6 == nil {
			return nil, trace.BadParameter("could not get source IPv6 address representation from %q", p.Source.IP.String())
		}
		copy(addr6.Source[:], sourceIPv6)
		destIPv6 := p.Destination.IP.To16()
		if destIPv6 == nil {
			return nil, trace.BadParameter("could not get destination IPv6 address representation from %q", p.Destination.IP.String())
		}
		copy(addr6.Destination[:], destIPv6)
		addr = addr6
	default:
		return nil, trace.BadParameter("unsupported protocol %q", p.Protocol)
	}
	tlvsBytes, err := MarshalTLVs(p.TLVs)
	if err != nil {
		return nil, trace.Errorf("could not marshal TLVs for the proxy line: %w", err)
	}
	if binary.Size(addr)+binary.Size(tlvsBytes) > math.MaxUint16 {
		return nil, trace.LimitExceeded("size of PROXY payload is too large")
	}
	header.Length = uint16(binary.Size(addr) + binary.Size(tlvsBytes))
	binary.Write(b, binary.BigEndian, header)
	binary.Write(b, binary.BigEndian, addr)
	binary.Write(b, binary.BigEndian, tlvsBytes)

	return b.Bytes(), nil
}

// ReadProxyLine reads proxy line protocol from the reader
func ReadProxyLine(reader *bufio.Reader) (*ProxyLine, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !strings.HasSuffix(line, proxyCRLF) {
		return nil, trace.BadParameter("expected CRLF in proxy protocol, got something else")
	}
	tokens := strings.Split(line[:len(line)-2], proxySep)
	ret := ProxyLine{}
	if len(tokens) < 6 {
		return nil, trace.BadParameter("malformed PROXY line protocol string")
	}
	switch tokens[1] {
	case TCP4:
		ret.Protocol = TCP4
	case TCP6:
		ret.Protocol = TCP6
	default:
		ret.Protocol = UNKNOWN
	}
	sourceIP, err := parseIP(ret.Protocol, tokens[2])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	destIP, err := parseIP(ret.Protocol, tokens[3])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sourcePort, err := parsePortNumber(tokens[4])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	destPort, err := parsePortNumber(tokens[5])
	if err != nil {
		return nil, err
	}
	ret.Source = net.TCPAddr{IP: sourceIP, Port: sourcePort}
	ret.Destination = net.TCPAddr{IP: destIP, Port: destPort}
	return &ret, nil
}

func parsePortNumber(portString string) (int, error) {
	port, err := strconv.Atoi(portString)
	if err != nil {
		return -1, trace.BadParameter("bad port %q: %v", port, err)
	}
	if port < 0 || port > 65535 {
		return -1, trace.BadParameter("port %q not in supported range [0...65535]", portString)
	}
	return port, nil
}

func parseIP(protocol string, addrString string) (net.IP, error) {
	addr := net.ParseIP(addrString)
	switch {
	case len(addr) == 0:
		return nil, trace.BadParameter("failed to parse address")
	case addr.To4() != nil && protocol != TCP4:
		return nil, trace.BadParameter("got IPV4 address %q for IPV6 proto %q", addr.String(), protocol)
	case addr.To4() == nil && protocol == TCP6:
		return nil, trace.BadParameter("got IPV6 address %v %q for IPV4 proto %q", len(addr), addr.String(), protocol)
	}
	return addr, nil
}

type proxyV2Header struct {
	Signature      [12]uint8
	VersionCommand uint8
	Protocol       uint8
	Length         uint16
}

type proxyV2Address4 struct {
	Source          [4]uint8
	Destination     [4]uint8
	SourcePort      uint16
	DestinationPort uint16
}

type proxyV2Address6 struct {
	Source          [16]uint8
	Destination     [16]uint8
	SourcePort      uint16
	DestinationPort uint16
}

const (
	Version2     = 2
	ProxyCommand = 1
	LocalCommand = 0
	ProtocolTCP4 = 0x11
	ProtocolTCP6 = 0x21
)

// ReadProxyLineV2 reads PROXY protocol v2 line from the reader
func ReadProxyLineV2(reader *bufio.Reader) (*ProxyLine, error) {
	var header proxyV2Header
	var ret ProxyLine
	if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
		return nil, trace.Wrap(err)
	}
	if !bytes.Equal(header.Signature[:], ProxyV2Prefix) {
		return nil, trace.BadParameter("unrecognized signature %s", hex.EncodeToString(header.Signature[:]))
	}
	cmd, ver := header.VersionCommand&0xF, header.VersionCommand>>4
	if ver != Version2 {
		return nil, trace.BadParameter("unsupported version %d", ver)
	}
	if cmd == LocalCommand {
		// LOCAL command, just skip address information and keep original addresses (no proxy line)
		if header.Length > 0 {
			_, err := io.CopyN(io.Discard, reader, int64(header.Length))
			return nil, trace.Wrap(err)
		}
		return nil, nil
	}
	if cmd != ProxyCommand {
		return nil, trace.BadParameter("unsupported command %d", cmd)
	}
	var size uint16
	switch header.Protocol {
	case ProtocolTCP4:
		var addr proxyV2Address4
		size = uint16(unsafe.Sizeof(addr))
		if err := binary.Read(reader, binary.BigEndian, &addr); err != nil {
			return nil, trace.Wrap(err)
		}
		ret.Protocol = TCP4
		ret.Source = net.TCPAddr{IP: addr.Source[:], Port: int(addr.SourcePort)}
		ret.Destination = net.TCPAddr{IP: addr.Destination[:], Port: int(addr.DestinationPort)}
	case ProtocolTCP6:
		var addr proxyV2Address6
		size = uint16(unsafe.Sizeof(addr))
		if err := binary.Read(reader, binary.BigEndian, &addr); err != nil {
			return nil, trace.Wrap(err)
		}
		ret.Protocol = TCP6
		ret.Source = net.TCPAddr{IP: addr.Source[:], Port: int(addr.SourcePort)}
		ret.Destination = net.TCPAddr{IP: addr.Destination[:], Port: int(addr.DestinationPort)}
	default:
		return nil, trace.BadParameter("unsupported protocol %x", header.Protocol)
	}

	// If there are more bytes left it means we've got TLVs
	if header.Length > size {
		tlvsBytes := &bytes.Buffer{}

		if _, err := io.CopyN(tlvsBytes, reader, int64(header.Length-size)); err != nil {
			return nil, trace.Wrap(err)
		}

		tlvs, err := UnmarshalTLVs(tlvsBytes.Bytes())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.TLVs = tlvs
	}

	return &ret, nil
}

// UnmarshalTLVs parses provided bytes slice into slice of TLVs
func UnmarshalTLVs(rawBytes []byte) ([]TLV, error) {
	var tlvs []TLV
	for len(rawBytes) > 0 {
		if len(rawBytes) < 3 {
			return nil, ErrTruncatedTLV
		}

		tlv := TLV{
			Type: PP2Type(rawBytes[0]), // First byte is TLV type
		}

		// Next two bytes are TLV's value length
		lenStart := 1
		lenEnd := lenStart + 2
		tlvLen := int(binary.BigEndian.Uint16(rawBytes[lenStart:lenEnd]))
		rawBytes = rawBytes[3:] // Move by 3 bytes to skip TLV header
		if tlvLen > len(rawBytes) {
			return nil, ErrTruncatedTLV
		}

		// Ignore no-op padding
		if tlv.Type != PP2TypeNOOP {
			tlv.Value = make([]byte, tlvLen)
			copy(tlv.Value, rawBytes[:tlvLen])
		}

		rawBytes = rawBytes[tlvLen:]
		tlvs = append(tlvs, tlv)
	}
	return tlvs, nil
}

// MarshalTLVs marshals provided slice of TLVs into slice of bytes.
func MarshalTLVs(tlvs []TLV) ([]byte, error) {
	var raw []byte
	for _, tlv := range tlvs {
		if len(tlv.Value) > math.MaxUint16 {
			return nil, trace.LimitExceeded("can not marshal TLV with type %v, length %d exceeds the limit of 65kb", tlv.Type, len(tlv.Value))
		}
		var length [2]byte
		binary.BigEndian.PutUint16(length[:], uint16(len(tlv.Value)))
		raw = append(raw, byte(tlv.Type))
		raw = append(raw, length[:]...)
		raw = append(raw, tlv.Value...)
	}
	return raw, nil
}

// AddTeleportTLVs adds the provided signature, cert, and an optional original address to the proxy line,
// marshaling it into appropriate TLV structure.
func (p *ProxyLine) AddTeleportTLVs(signature, signingCert []byte, originalAddr *net.TCPAddr) error {
	if len(signature) == 0 {
		return trace.BadParameter("missing signature")
	}
	if len(signingCert) == 0 {
		return trace.BadParameter("missing signing certificate")
	}

	teleportTLVs := []TLV{
		{
			Type:  PP2Type(PP2TeleportSubtypeSigningCert),
			Value: signingCert,
		},
		{
			Type:  PP2Type(PP2TeleportSubtypeJWT),
			Value: signature,
		},
	}

	if originalAddr != nil {
		teleportTLVs = append(teleportTLVs, TLV{
			Type:  PP2Type(PP2TeleportSubtypeOriginalAddr),
			Value: []byte(originalAddr.String()),
		})
	}

	teleportTLVBytes, err := MarshalTLVs(teleportTLVs)
	if err != nil {
		return err
	}

	// If there's already signature among TLVs, we replace it
	for i := range p.TLVs {
		if p.TLVs[i].Type == PP2TypeTeleport {
			p.TLVs[i].Value = teleportTLVBytes
			return nil
		}
	}

	// Otherwise we append it
	p.TLVs = append(p.TLVs, TLV{Type: PP2TypeTeleport, Value: teleportTLVBytes})

	return nil
}

// IsSigned returns true if proxy line's TLV contains signature.
// Does not take into account if signature is valid or not, just the presence of it.
func (p *ProxyLine) IsSigned() bool {
	tlvs, _ := p.getTeleportTLVs()
	return len(tlvs.token) > 0 || tlvs.proxyCert != nil
}

type teleportTLVs struct {
	token           string
	proxyCert       []byte
	originalAddress *net.TCPAddr
}

// getTeleportTLVs returns custom teleport TLVs present in the ProxyLine, if any
func (p *ProxyLine) getTeleportTLVs() (teleportTLVs, error) {
	var tlvs teleportTLVs
	for _, tlv := range p.TLVs {
		if tlv.Type == PP2TypeTeleport {
			teleportSubTLVs, err := UnmarshalTLVs(tlv.Value)
			if err != nil {
				return tlvs, trace.Wrap(err)
			}

			for _, subTLV := range teleportSubTLVs {
				switch PP2TeleportSubtype(subTLV.Type) {
				case PP2TeleportSubtypeSigningCert:
					tlvs.proxyCert = subTLV.Value
				case PP2TeleportSubtypeJWT:
					tlvs.token = string(subTLV.Value)
				case PP2TeleportSubtypeOriginalAddr:
					addr, err := net.ResolveTCPAddr("tcp", string(subTLV.Value))
					if err != nil {
						return tlvs, trace.Wrap(err)
					}
					tlvs.originalAddress = addr
				}
			}
			break
		}
	}
	return tlvs, nil
}

// VerifySignature checks that signature contained in the proxy line is securely signed.
func (p *ProxyLine) VerifySignature(ctx context.Context, caGetter CertAuthorityGetter, localClusterName string, clock clockwork.Clock) error {
	// If there's no TLVs it can't be verified
	if len(p.TLVs) == 0 {
		return trace.Wrap(ErrNoSignature)
	}

	tlvs, err := p.getTeleportTLVs()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(tlvs.token) == 0 || tlvs.proxyCert == nil {
		return trace.Wrap(ErrNoSignature)
	}

	signingCert, err := x509.ParseCertificate(tlvs.proxyCert)
	if err != nil {
		return trace.Wrap(err)
	}

	identity, err := tlsca.FromSubject(signingCert.Subject, signingCert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}
	if identity.TeleportCluster != localClusterName {
		return trace.Wrap(ErrNonLocalCluster, "signing certificate cluster name: %s, local cluster name: %s",
			identity.TeleportCluster, localClusterName)
	}

	hostCA, err := caGetter(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: localClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(ErrNoHostCA, "CA cluster name: %s", localClusterName)
	}
	hostCACerts := getTLSCerts(hostCA)

	roots := x509.NewCertPool()
	for _, cert := range hostCACerts {
		ok := roots.AppendCertsFromPEM(cert)
		if !ok {
			return trace.Wrap(ErrBadCACert)
		}
	}

	// Make sure that transmitted proxy cert is signed by appropriate host CA
	_, err = signingCert.Verify(x509.VerifyOptions{Roots: roots})
	if err != nil {
		return trace.Wrap(err)
	}

	foundRole := checkForSystemRole(identity, types.RoleProxy)
	if !foundRole {
		return trace.Wrap(ErrIncorrectRole)
	}

	// Check JWT using proxy cert's public key
	jwtVerifier, err := jwt.New(&jwt.Config{
		Clock:       clock,
		PublicKey:   signingCert.PublicKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: localClusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Determine if a pseudo IPv4 was used and validate
	sAddr := p.Source.String()
	if tlvs.originalAddress != nil {
		expectedPLSource, err := getPseudoIPV4(*tlvs.originalAddress)
		if err != nil {
			return trace.Wrap(err)
		}

		if !expectedPLSource.IP.Equal(p.Source.IP) {
			return trace.Wrap(ErrInvalidPseudoIPv4)
		}

		sAddr = tlvs.originalAddress.String()
	}
	_, err = jwtVerifier.VerifyPROXY(jwt.PROXYVerifyParams{
		ClusterName:        localClusterName,
		SourceAddress:      sAddr,
		DestinationAddress: p.Destination.String(),
		RawToken:           tlvs.token,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	p.IsVerified = true
	return nil
}

// ResolveSource returns the source IP address associated with a ProxyLine. If the Source is a class E address
// then we need to return the IPv6 stored in the teleport TLVs instead.
func (p *ProxyLine) ResolveSource() net.Addr {
	// check if class E address
	if []byte(p.Source.IP)[0] < classEPrefix {
		return &p.Source
	}

	tlvs, err := p.getTeleportTLVs()
	if err != nil {
		return &p.Source
	}

	return tlvs.originalAddress
}

func getTLSCerts(ca types.CertAuthority) [][]byte {
	pairs := ca.GetTrustedTLSKeyPairs()
	out := make([][]byte, len(pairs))
	for i, pair := range pairs {
		out[i] = append([]byte{}, pair.Cert...)
	}
	return out
}

func checkForSystemRole(identity *tlsca.Identity, roleToFind types.SystemRole) bool {
	findRole := func(roles []string) bool {
		for _, role := range roles {
			if roleToFind == types.SystemRole(role) {
				return true
			}
		}

		return false
	}
	return findRole(identity.Groups) || findRole(identity.SystemRoles)
}
