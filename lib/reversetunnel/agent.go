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

// Package reversetunnel sets up persistent reverse tunnel
// between remote site and teleport proxy, when site agents
// dial to teleport proxy's socket and teleport proxy can connect
// to any server through this tunnel.
package reversetunnel

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Agent is a reverse tunnel agent running as a part of teleport Proxies
// to establish outbound reverse tunnels to remote proxies
type Agent struct {
	log             *log.Entry
	addr            utils.NetAddr
	clt             *auth.TunClient
	domainName      string
	broadcastClose  *utils.CloseBroadcaster
	disconnectC     chan bool
	hostKeyCallback utils.HostKeyCallback
	authMethods     []ssh.AuthMethod
}

// AgentOption specifies parameter that could be passed to Agents
type AgentOption func(a *Agent) error

// NewAgent returns a new reverse tunnel agent
func NewAgent(
	addr utils.NetAddr,
	domainName string,
	signers []ssh.Signer,
	clt *auth.TunClient) (*Agent, error) {

	a := &Agent{
		log: log.WithFields(log.Fields{
			teleport.Component: teleport.ComponentReverseTunnel,
			teleport.ComponentFields: map[string]interface{}{
				"side":   "agent",
				"remote": addr.String(),
				"mode":   "agent",
			},
		}),
		clt:            clt,
		addr:           addr,
		domainName:     domainName,
		broadcastClose: utils.NewCloseBroadcaster(),
		disconnectC:    make(chan bool, 10),
		authMethods:    []ssh.AuthMethod{ssh.PublicKeys(signers...)},
	}
	a.hostKeyCallback = a.checkHostSignature
	return a, nil
}

// Close signals to close all connections
func (a *Agent) Close() error {
	return a.broadcastClose.Close()
}

// Start starts agent that attempts to connect to remote server part
func (a *Agent) Start() error {
	conn, err := a.connect()
	// start heartbeat even if error happend, it will reconnect
	go a.runHeartbeat(conn)
	return err
}

// Wait waits until all outstanding operations are completed
func (a *Agent) Wait() error {
	return nil
}

// String returns debug-friendly
func (a *Agent) String() string {
	return fmt.Sprintf("tunagent(remote=%v)", a.addr)
}

func (a *Agent) checkHostSignature(hostport string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.Errorf("expected certificate")
	}
	cas, err := a.clt.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return trace.Wrap(err, "failed to fetch remote certs")
	}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return trace.BadParameter("error parsing key: %v", err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(checker, cert.SignatureKey) {
				a.log.Infof("matched key %v for %v", ca.ID(), hostport)
				return nil
			}
		}
	}
	return trace.NotFound(
		"no matching keys found when checking server's host signature")
}

func (a *Agent) connect() (conn *ssh.Client, err error) {
	if a.addr.IsEmpty() {
		return nil, trace.BadParameter("reverse tunnel cannot be created: target address is empty")
	}
	for _, authMethod := range a.authMethods {
		conn, err = ssh.Dial(a.addr.AddrNetwork, a.addr.Addr, &ssh.ClientConfig{
			User:            a.domainName,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: a.hostKeyCallback,
		})
		if conn != nil {
			break
		}
	}
	return conn, err
}

func (a *Agent) proxyAccessPoint(ch ssh.Channel, req <-chan *ssh.Request) {
	defer ch.Close()

	conn, err := a.clt.GetDialer()()
	if err != nil {
		a.log.Errorf("error dialing: %v", err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(conn, ch)
	}()

	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(ch, conn)
	}()

	wg.Wait()
}

func (a *Agent) proxyTransport(ch ssh.Channel, reqC <-chan *ssh.Request) {
	defer ch.Close()

	var req *ssh.Request
	select {
	case <-a.broadcastClose.C:
		a.log.Infof("is closed, returning")
		return
	case req = <-reqC:
		if req == nil {
			a.log.Infof("connection closed, returning")
			return
		}
	case <-time.After(teleport.DefaultTimeout):
		a.log.Errorf("timeout waiting for dial")
		return
	}

	server := string(req.Payload)
	log.Infof("got out of band request %v", server)

	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Errorf("failed to dial: %v, err: %v", server, err)
		return
	}
	req.Reply(true, []byte("connected"))

	a.log.Infof("successfully dialed to %v, start proxying", server)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		// make sure that we close the client connection on a channel
		// close, otherwise the other goroutine would never know
		// as it will block on read from the connection
		defer conn.Close()
		io.Copy(conn, ch)
	}()

	go func() {
		defer wg.Done()
		io.Copy(ch, conn)
	}()

	wg.Wait()
}

// runHeartbeat is a blocking function which runs in a loop sending heartbeats
// to the given SSH connection.
//
func (a *Agent) runHeartbeat(conn *ssh.Client) {
	heartbeatLoop := func() error {
		if conn == nil {
			return trace.Errorf("heartbeat cannot ping: need to reconnect")
		}
		defer conn.Close()
		hb, reqC, err := conn.OpenChannel(chanHeartbeat, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		newAccesspointC := conn.HandleChannelOpen(chanAccessPoint)
		newTransportC := conn.HandleChannelOpen(chanTransport)

		// send first ping right away, then start a ping timer:
		hb.SendRequest("ping", false, nil)
		ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
		defer ticker.Stop()

		for {
			select {
			// need to exit:
			case <-a.broadcastClose.C:
				return nil
			// time to ping:
			case <-ticker.C:
				_, err := hb.SendRequest("ping", false, nil)
				if err != nil {
					return trace.Wrap(err)
				}
			// ssh channel closed:
			case req := <-reqC:
				if req == nil {
					return trace.Errorf("heartbeat: connection closed")
				}
			// new access point request:
			case nch := <-newAccesspointC:
				if nch == nil {
					continue
				}
				a.log.Infof("reverseTunnel.Agent: access point request: %v", nch.ChannelType())
				ch, req, err := nch.Accept()
				if err != nil {
					a.log.Errorf("failed to accept request: %v", err)
					continue
				}
				go a.proxyAccessPoint(ch, req)
			// new transport request:
			case nch := <-newTransportC:
				if nch == nil {
					continue
				}
				a.log.Infof("reverseTunnel.Agent: transport request: %v", nch.ChannelType())
				ch, req, err := nch.Accept()
				if err != nil {
					a.log.Errorf("failed to accept request: %v", err)
					continue
				}
				go a.proxyTransport(ch, req)
			}
		}
	}

	err := heartbeatLoop()
	if err != nil || conn == nil {
		time.Sleep(defaults.ReverseTunnelAgentHeartbeatPeriod)
		a.Start()
	}
}

const (
	chanHeartbeat        = "teleport-heartbeat"
	chanAccessPoint      = "teleport-access-point"
	chanTransport        = "teleport-transport"
	chanTransportDialReq = "teleport-transport-dial"
)

const (
	// RemoteSiteStatusOffline indicates that site is considered as
	// offline, since it has missed a series of heartbeats
	RemoteSiteStatusOffline = "offline"
	// RemoteSiteStatusOnline indicates that site is sending heartbeats
	// at expected interval
	RemoteSiteStatusOnline = "online"
)
