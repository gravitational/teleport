package tun

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type Agent struct {
	addr        utils.NetAddr
	elog        lunk.EventLogger
	clt         *auth.TunClient
	signers     []ssh.Signer
	fqdn        string
	waitC       chan bool
	disconnectC chan bool
	conn        ssh.Conn
}

type AgentOption func(a *Agent) error

func SetEventLogger(e lunk.EventLogger) AgentOption {
	return func(s *Agent) error {
		s.elog = e
		return nil
	}
}

func NewAgent(addr utils.NetAddr, fqdn string, signers []ssh.Signer,
	clt *auth.TunClient, options ...AgentOption) (*Agent, error) {

	a := &Agent{
		clt:         clt,
		addr:        addr,
		fqdn:        fqdn,
		signers:     signers,
		waitC:       make(chan bool),
		disconnectC: make(chan bool, 10),
	}
	for _, o := range options {
		if err := o(a); err != nil {
			return nil, err
		}
	}
	if a.elog == nil {
		a.elog = utils.NullEventLogger
	}
	return a, nil
}

func (a *Agent) Start() error {
	if err := a.reconnect(); err != nil {
		return err
	}
	go a.handleDisconnect()
	return nil
}

func (a *Agent) handleDisconnect() {
	log.Infof("will handle disconnects")
	for {
		select {
		case <-a.disconnectC:
			log.Infof("detected disconnect, reconnecting")
			a.reconnect()
		}
	}
}

func (a *Agent) reconnect() error {
	var err error
	i := 0
	for {
		i += 1
		if err = a.connect(); err != nil {
			log.Infof("%v connect attempt %v: %v", a, i, err)
			time.Sleep(time.Duration(min(i, 10)) * time.Second)
			continue
		}
		return nil
	}
	return err
}

func (a *Agent) Wait() error {
	<-a.waitC
	return nil
}

func (a *Agent) String() string {
	return fmt.Sprintf("tunagent(remote=%v)", a.addr)
}

func (a *Agent) checkHostSignature(hostport string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		log.Infof("expected: %v", key)
		return fmt.Errorf("expected certificate")
	}
	hostname, _, err := net.SplitHostPort(hostport)
	if err != nil {
		log.Errorf("error spliting hostport(%v), err: %v", hostport, err)
		return err
	}
	certs, err := a.clt.GetRemoteCerts(services.HostCert, hostname)
	if err != nil {
		log.Errorf("failed to fetch remote certs: %v", err)
		return err
	}
	for _, c := range certs {
		log.Infof("checking key(id=%v) against host %v", c.ID, c.FQDN)
		pk, _, _, _, err := ssh.ParseAuthorizedKey(c.Value)
		if err != nil {
			log.Errorf("error parsing key: %v", err)
			return err
		}
		if sshutils.KeysEqual(pk, cert.SignatureKey) {
			log.Infof("matched key %v for %v", c.ID, c.FQDN)
			return nil
		}
	}
	return fmt.Errorf("no matching keys found")
}

func (a *Agent) connect() error {
	log.Infof("agent connect")
	c, err := ssh.Dial(a.addr.Network, a.addr.Addr, &ssh.ClientConfig{
		User:            a.fqdn,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(a.signers...)},
		HostKeyCallback: a.checkHostSignature,
	})
	if err != nil {
		log.Errorf("failed to dial: %v", err)
		return err
	}
	a.conn = c

	go a.startHeartbeat()
	go a.handleAccessPoint(c.HandleChannelOpen(chanAccessPoint))
	go a.handleTransport(c.HandleChannelOpen(chanTransport))

	log.Infof("%v connection established", a)
	return nil
}

func (a *Agent) handleAccessPoint(newC <-chan ssh.NewChannel) {
	for {
		nch := <-newC
		if nch == nil {
			log.Infof("connection closed, returning")
			return
		}
		log.Infof("got access point request: %v", nch.ChannelType())
		ch, req, err := nch.Accept()
		if err != nil {
			log.Errorf("failed to accept request: %v", err)
		}
		go a.proxyAccessPoint(ch, req)
	}
}

func (a *Agent) handleTransport(newC <-chan ssh.NewChannel) {
	for {
		nch := <-newC
		if nch == nil {
			log.Infof("connection closed, returing")
			return
		}
		log.Infof("got transport request: %v", nch.ChannelType())
		ch, req, err := nch.Accept()
		if err != nil {
			log.Errorf("failed to accept request: %v", err)
		}
		go a.proxyTransport(ch, req)
	}
}

func (a *Agent) proxyAccessPoint(ch ssh.Channel, req <-chan *ssh.Request) {
	defer ch.Close()

	conn, err := a.clt.GetDialer()()
	if err != nil {
		log.Errorf("%v error dialing: %v", a, err)
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
	case req = <-reqC:
		if req == nil {
			log.Infof("connection closed, returning")
			return
		}
	case <-time.After(10 * time.Second):
		log.Errorf("timeout waiting for dial")
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

	log.Infof("successfully dialed to %v, start proxying", server)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(conn, ch)
	}()

	go func() {
		defer wg.Done()
		io.Copy(ch, conn)
	}()

	wg.Wait()
}

func (a *Agent) startHeartbeat() {
	defer func() {
		a.disconnectC <- true
		log.Infof("sent disconnect message")
	}()

	hb, reqC, err := a.conn.OpenChannel(chanHeartbeat, nil)
	if err != nil {
		log.Errorf("failed to open channel: %v", err)
		return
	}

	closeC := make(chan bool)
	errC := make(chan error, 2)

	go func() {
		for {
			select {
			case <-closeC:
				log.Infof("asked to exit")
				return
			default:
			}
			_, err := hb.SendRequest("ping", false, nil)
			if err != nil {
				log.Errorf("failed to send heartbeat: %v", err)
				errC <- err
				return
			}
			time.Sleep(heartbeatPeriod)
		}
	}()

	go func() {
		for {
			select {
			case <-closeC:
				log.Infof("asked to exit")
				return
			case req := <-reqC:
				if req == nil {
					errC <- fmt.Errorf("heartbeat: connection closed")
					return
				}
				log.Infof("got out of band request: %v", req)
			}
		}
	}()

	log.Infof("got error: %v", <-errC)
	close(closeC)
}

const (
	chanHeartbeat   = "teleport-heartbeat"
	chanAccessPoint = "teleport-access-point"
	chanTransport   = "teleport-transport"

	chanTransportDialReq = "teleport-transport-dial"

	heartbeatPeriod = 5 * time.Second
)

const (
	RemoteSiteStatusOffline = "offline"
	RemoteSiteStatusOnline  = "online"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
