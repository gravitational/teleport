/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sshutils defines several functions and types used across the
// Teleport API and other Teleport packages when working with SSH.
package sshutils

import (
	"bytes"
	"context"
	"crypto"
	"crypto/subtle"
	"errors"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/defaults"
)

// HandshakePayload structure is sent as a JSON blob by the teleport
// proxy to every SSH server who identifies itself as Teleport server
//
// It allows teleport proxies to communicate additional data to server
type HandshakePayload struct {
	// ClientAddr is the IP address of the remote client
	ClientAddr string `json:"clientAddr,omitempty"`
	// TracingContext contains tracing information so that spans can be correlated
	// across ssh boundaries
	TracingContext map[string]string `json:"tracingContext,omitempty"`
}

// ParseCertificate parses an SSH certificate from the authorized_keys format.
func ParseCertificate(buf []byte) (*ssh.Certificate, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not an SSH certificate")
	}

	return cert, nil
}

// ParseKnownHosts parses provided known_hosts entries into ssh.PublicKey list.
// If one or more hostnames are provided, only keys that have at least one match
// will be returned.
func ParseKnownHosts(knownHosts [][]byte, matchHostnames ...string) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range knownHosts {
		for {
			_, hosts, publicKey, _, bytes, err := ssh.ParseKnownHosts(line)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, trace.Wrap(err, "failed parsing known hosts: %v; raw line: %q", err, line)
			}

			if len(matchHostnames) == 0 || HostNameMatch(matchHostnames, hosts) {
				keys = append(keys, publicKey)
			}

			line = bytes
		}
	}
	return keys, nil
}

// HostNameMatch returns whether at least one of the given hosts matches one
// of the given matchHosts. If a host has a wildcard prefix "*.", it will be
// used to match. Ex: "*.example.com" will  match "proxy.example.com".
func HostNameMatch(matchHosts []string, hosts []string) bool {
	for _, matchHost := range matchHosts {
		for _, host := range hosts {
			if host == matchHost || matchesWildcard(matchHost, host) {
				return true
			}
		}
	}
	return false
}

// matchesWildcard ensures the given `hostname` matches the given `pattern`.
// The `pattern` should be prefixed with `*.` which will match exactly one domain
// segment, meaning `*.example.com` will match `foo.example.com` but not
// `foo.bar.example.com`.
func matchesWildcard(hostname, pattern string) bool {
	pattern = strings.TrimSpace(pattern)

	// Don't allow non-wildcard or empty patterns.
	if !strings.HasPrefix(pattern, "*.") || len(pattern) < 3 {
		return false
	}
	matchHost := pattern[2:]

	// Trim any trailing "." in case of an absolute domain.
	hostname = strings.TrimSuffix(hostname, ".")

	_, hostnameRoot, found := strings.Cut(hostname, ".")
	if !found {
		return false
	}

	return hostnameRoot == matchHost
}

// ParseAuthorizedKeys parses provided authorized_keys entries into ssh.PublicKey list.
func ParseAuthorizedKeys(authorizedKeys [][]byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range authorizedKeys {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			return nil, trace.Wrap(err, "failed parsing authorized keys: %v; raw line: %q", err, line)
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig from the given ssh.AuthMethod.
// If known_hosts are provided, they will be used in the config's HostKeyCallback.
//
// The config is set up to authenticate to proxy with the first available principal.
func ProxyClientSSHConfig(sshCert *ssh.Certificate, priv crypto.Signer, knownHosts ...[]byte) (*ssh.ClientConfig, error) {
	authMethod, err := AsAuthMethod(sshCert, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := &ssh.ClientConfig{
		Auth:    []ssh.AuthMethod{authMethod},
		Timeout: defaults.DefaultIOTimeout,
	}

	// The KeyId is not always a valid principal, so we use the first valid principal instead.
	cfg.User = sshCert.KeyId
	if len(sshCert.ValidPrincipals) > 0 {
		cfg.User = sshCert.ValidPrincipals[0]
	}

	if len(knownHosts) > 0 {
		trustedKeys, err := ParseKnownHosts(knownHosts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.HostKeyCallback, err = HostKeyCallback(trustedKeys, false)
		if err != nil {
			return nil, trace.Wrap(err, "failed to convert certificate authorities to HostKeyCallback")
		}
	}

	return cfg, nil
}

// SSHSigner returns an ssh.Signer from certificate and private key
func SSHSigner(sshCert *ssh.Certificate, signer crypto.Signer) (ssh.Signer, error) {
	sshSigner, err := ssh.NewSignerFromKey(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err = ssh.NewCertSigner(sshCert, sshSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshSigner, nil
}

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func AsAuthMethod(sshCert *ssh.Certificate, signer crypto.Signer) (ssh.AuthMethod, error) {
	sshSigner, err := SSHSigner(sshCert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.PublicKeys(sshSigner), nil
}

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against trusted host keys, usually associated with trusted CAs.
//
// If no trusted keys are provided, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func HostKeyCallback(trustedKeys []ssh.PublicKey, withHostKeyFallback bool) (ssh.HostKeyCallback, error) {
	// No trusted keys are provided, return a nil callback which will prompt the user for trust.
	if len(trustedKeys) == 0 {
		return nil, nil
	}

	callbackConfig := HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			return trustedKeys, nil
		},
	}

	if withHostKeyFallback {
		callbackConfig.HostKeyFallback = hostKeyFallbackFunc(trustedKeys)
	}

	callback, err := NewHostKeyCallback(callbackConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return callback, nil
}

func hostKeyFallbackFunc(knownHosts []ssh.PublicKey) func(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		for _, knownHost := range knownHosts {
			if KeysEqual(key, knownHost) {
				return nil
			}
		}
		return trace.AccessDenied("host %v presented a public key instead of a host certificate which isn't among known hosts", hostname)
	}
}

// KeysEqual is constant time compare of the keys to avoid timing attacks
func KeysEqual(ak, bk ssh.PublicKey) bool {
	a := ak.Marshal()
	b := bk.Marshal()
	return subtle.ConstantTimeCompare(a, b) == 1
}

// OpenSSH cert types look like "<key-type>-cert-v<version>@openssh.com".
var sshCertTypeRegex = regexp.MustCompile(`^[a-z0-9\-]+-cert-v[0-9]{2}@openssh\.com$`)

// IsSSHCertType checks if the given string looks like an ssh cert type.
// e.g. ssh-rsa-cert-v01@openssh.com.
func IsSSHCertType(val string) bool {
	return sshCertTypeRegex.MatchString(val)
}

type contextDialer func(ctx context.Context, network, addr string) (net.Conn, error)

type runSSHOpts struct {
	dialContext contextDialer
}

// RunSSHOption allows setting options as functional arguments to RunSSH.
type RunSSHOption func(*runSSHOpts)

// WithDialer connects to an SSH server with a custom dialer.
func WithDialer(dialer contextDialer) RunSSHOption {
	return func(opts *runSSHOpts) {
		opts.dialContext = dialer
	}
}

// RunSSH runs a command on an SSH server and returns the output.
func RunSSH(ctx context.Context, addr, command string, cfg *ssh.ClientConfig, opts ...RunSSHOption) ([]byte, []byte, error) {
	var options runSSHOpts
	for _, opt := range opts {
		opt(&options)
	}

	conn, err := options.dialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clientConn, newCh, requestsCh, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshClient := ssh.NewClient(clientConn, newCh, requestsCh)
	defer sshClient.Close()
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer session.Close()

	// Execute the command.
	var stdout bytes.Buffer
	session.Stdout = &stdout
	var stderr bytes.Buffer
	session.Stderr = &stderr
	err = session.Run(command)
	return stdout.Bytes(), stderr.Bytes(), trace.Wrap(err)
}

// ChannelReadWriter represents the data streams of an ssh.Channel-like object.
type ChannelReadWriter interface {
	io.ReadWriter
	Stderr() io.ReadWriter
}

// DiscardChannelData discards all data received from an ssh channel in the
// background.
func DiscardChannelData(ch ChannelReadWriter) {
	if ch == nil {
		return
	}
	go io.Copy(io.Discard, ch)
	go io.Copy(io.Discard, ch.Stderr())
}
