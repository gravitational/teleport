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


Keystore implements functions for saving and loading from hard disc
temporary teleport certificates
*/

package client

import (
	"net"

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
}

// NewLocalAgent loads all the saved teleport certificates and
// creates ssh agent with them
func NewLocalAgent(keyDir string) (a *LocalKeyAgent, err error) {
	keystore, err := NewFSLocalKeyStore(keyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a = &LocalKeyAgent{
		Agent:    agent.NewKeyring(),
		keyStore: keystore,
	}
	// add all stored keys into the agent:
	keys, err := a.GetKeys()
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

func (a *LocalKeyAgent) GetKeys() ([]agent.AddedKey, error) {
	keys, err := a.keyStore.GetKeys()
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
func (a *LocalKeyAgent) AddHostSignersToCache(hostSigners []services.CertAuthority) error {
	for _, hostSigner := range hostSigners {
		publicKeys, err := hostSigner.Checkers()
		if err != nil {
			log.Error(err)
			return trace.Wrap(err)
		}
		a.keyStore.AddKnownCA(hostSigner.DomainName, publicKeys)
	}
	return nil
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(hostId string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.Errorf("expected certificate")
	}
	keys, err := a.keyStore.GetKnownCAs()
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			log.Info("we can trust host %v", hostId)
			return nil
		}
	}
	err = trace.AccessDenied("remote host %v at %v cannot be trusted", hostId, remote)
	log.Error(err)
	return trace.Wrap(err)
}

func (a *LocalKeyAgent) AddKey(host string, key *Key) error {
	err := a.keyStore.AddKey(host, key)
	if err != nil {
		return trace.Wrap(err)
	}
	agentKey, err := key.AsAgentKey()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Agent.Add(*agentKey)
}
