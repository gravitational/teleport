/*
Copyright 2015 Gravitational, Inc.

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

package client

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type LocalKeyAgent struct {
	// implements ssh agent.Agent interface
	agent.Agent
	keyStore LocalKeyStore

	// sshAgent is the external SSH agent
	sshAgent agent.Agent
}

// NewLocalAgent loads all the saved teleport certificates and
// creates ssh agent with them
func NewLocalAgent(keyDir, username string) (a *LocalKeyAgent, err error) {
	keystore, err := NewFSLocalKeyStore(keyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a = &LocalKeyAgent{
		Agent:    agent.NewKeyring(),
		keyStore: keystore,
		sshAgent: connectToSSHAgent(),
	}
	// load all stored keys from disk (~/.tsh usually) and pass them into the agent:
	keys, err := a.LoadKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range keys {
		if err := a.Agent.Add(keys[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a, nil
}

// loadKeys return the list of keys for the given user
// from the local keystore (files in ~/.tsh)
func (a *LocalKeyAgent) LoadKeys(username string) ([]agent.AddedKey, error) {
	keys, err := a.keyStore.GetKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retval := make([]agent.AddedKey, len(keys))
	for i, key := range keys {
		ak, err := key.AsAgentKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		retval[i] = *ak
	}
	return retval, nil
}

// AddHostSignersToCache takes a list of CAs whom we trust. This list is added to a database
// of "seen" CAs.
//
// Every time we connect to a new host, we'll request its certificaate to be signed by one
// of these trusted CAs.
//
// Why do we trust these CAs? Because we received them from a trusted Teleport Proxy.
// Why do we trust the proxy? Because we've connected to it via HTTPS + username + Password + HOTP.
func (a *LocalKeyAgent) AddHostSignersToCache(hostSigners []services.CertAuthorityV1) error {
	for _, hostSigner := range hostSigners {
		publicKeys, err := hostSigner.V2().Checkers()
		if err != nil {
			log.Error(err)
			return trace.Wrap(err)
		}
		log.Debugf("[KEY AGENT] adding CA key for %s", hostSigner.DomainName)
		err = a.keyStore.AddKnownHostKeys(hostSigner.DomainName, publicKeys)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(hostId string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		// not a signed cert? perhaps we're given a host public key (happens when the host is running
		// sshd instead of teleport daemon
		keys, _ := a.keyStore.GetKnownHostKeys(hostId)
		if len(keys) > 0 && sshutils.KeysEqual(key, keys[0]) {
			log.Debugf("[KEY AGENT] verified host %s", hostId)
			return nil
		}
		// ask the user if they want to trust this host
		fmt.Printf("The authenticity of host '%s' can't be established. "+
			"Its public key is:\n%s\nAre you sure you want to continue (yes/no)? ",
			hostId, ssh.MarshalAuthorizedKey(key))

		bytes := make([]byte, 12)
		os.Stdin.Read(bytes)
		if strings.TrimSpace(strings.ToLower(string(bytes)))[0] != 'y' {
			err := trace.AccessDenied("untrusted host %v", hostId)
			log.Error(err)
			return err
		}
		// remember the host key (put it into 'known_hosts')
		if err := a.keyStore.AddKnownHostKeys(hostId, []ssh.PublicKey{key}); err != nil {
			log.Warnf("error saving the host key: %v", err)
		}
		// success
		return nil
	}

	// we are given a certificate. see if it was signed by any of the known_host keys:
	keys, err := a.keyStore.GetKnownHostKeys("")
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	log.Debugf("[KEY AGENT] got %d known hosts", len(keys))
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			log.Debugf("[KEY AGENT] verified host %s", hostId)
			return nil
		}
	}
	err = trace.AccessDenied("untrusted host %v", hostId)
	log.Error(err)
	return err
}

// AddKey stores a new signed session key for future use.
//
// It returns an implementation of ssh.Authmethod which can be passed to ssh.Config
// to make new SSH connections authenticated by this key.
//
func (a *LocalKeyAgent) AddKey(host string, username string, key *Key) (*CertAuthMethod, error) {
	// save it to disk (usually into ~/.tsh)
	err := a.keyStore.AddKey(host, username, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	agentKey, err := key.AsAgentKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// add it to the external SSH agent:
	if a.sshAgent != nil {
		if err = a.sshAgent.Add(*agentKey); err != nil {
			log.Warn(err)
		}
	}
	// add it to our own in-memory key agent:
	if err = a.Agent.Add(*agentKey); err != nil {
		return nil, trace.Wrap(err)
	}
	// generate SSH auth method based on the given signed key and return
	// it to the caller:
	signer, err := ssh.NewSignerFromKey(agentKey.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer, err = ssh.NewCertSigner(agentKey.Certificate, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return methodForCert(signer), nil
}

// DeleteKey deletes the user key stored for a given proxy server
func (a *LocalKeyAgent) DeleteKey(proxyHost string, username string) error {
	localKey, err := a.keyStore.GetKey(proxyHost, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// ... then remove it from the local storage:
	err = trace.Wrap(a.keyStore.DeleteKey(proxyHost, username))

	// FIRST, we need to remove the same key from the SSH Agent:
	if a.sshAgent == nil {
		return err
	}
	pubKey, _, _, _, _ := ssh.ParseAuthorizedKey(localKey.Cert)
	if pubKey == nil {
		return err
	}
	agentKeys, _ := a.sshAgent.List()
	for _, agentKey := range agentKeys {
		if bytes.Contains(pubKey.Marshal(), agentKey.Blob) {
			a.sshAgent.Remove(agentKey)
			break
		}
	}
	return err
}

// AuthMethods returns the list of differnt authentication methods this agent supports
// It returns two:
//	  1. First to try is the external SSH agent
//    2. Itself (disk-based local agent)
func (a *LocalKeyAgent) AuthMethods() (m []ssh.AuthMethod) {
	// combine our certificates with external SSH agent's:
	var certs []ssh.Signer
	if ourCerts, _ := a.Signers(); ourCerts != nil {
		certs = append(certs, ourCerts...)
	}
	if a.sshAgent != nil {
		if sshAgentCerts, _ := a.sshAgent.Signers(); sshAgentCerts != nil {
			certs = append(certs, sshAgentCerts...)
		}
	}
	// for every certificate create a new "auth method" and return them
	m = make([]ssh.AuthMethod, len(certs))
	for i := range certs {
		m[i] = methodForCert(certs[i])
	}
	return m
}

// CertAuthMethod is a wrapper around ssh.Signer (certificate signer) object.
// CertAuthMethod then implements ssh.Authmethod interface around this one certificate signer.
//
// We need this wrapper because Golang's SSH library's unfortunate API design. It uses
// callbacks with 'authMethod' interfaces and without this wrapper it is impossible to
// tell which certificate an 'authMethod' passed via a callback had succeeded authenticating with.
type CertAuthMethod struct {
	ssh.AuthMethod
	Cert ssh.Signer
}

func methodForCert(cert ssh.Signer) *CertAuthMethod {
	return &CertAuthMethod{
		Cert: cert,
		AuthMethod: ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return []ssh.Signer{cert}, nil
		}),
	}
}
