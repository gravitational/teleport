package reversetunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func newDirectSite(domainName string, client auth.ClientI) *directSite {
	return &directSite{
		client:     client,
		domainName: domainName,
		log: log.WithFields(log.Fields{
			teleport.Component: teleport.ComponentReverseTunnel,
			teleport.ComponentFields: map[string]string{
				"domainName": domainName,
				"side":       "server",
				"type":       "localSite",
			},
		}),
	}
}

// directSite allows to directly access the remote servers
// not using any tunnel, and using standard SSH
type directSite struct {
	sync.Mutex
	client auth.ClientI

	authServer  string
	log         *log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	lastActive  time.Time
	srv         *server
}

func (s *directSite) GetClient() (auth.ClientI, error) {
	return s.client, nil
}

func (s *directSite) String() string {
	return fmt.Sprintf("localSite(%v)", s.domainName)
}

func (s *directSite) GetStatus() string {
	return RemoteSiteStatusOnline
}

func (s *directSite) GetName() string {
	return s.domainName
}

func (s *directSite) GetLastConnected() time.Time {
	return time.Now()
}

func (s *directSite) ConnectToServer(server, user string, auth []ssh.AuthMethod) (*ssh.Client, error) {
	s.log.Infof("ConnectToServer(server=%v, user=%v)", server, user)

	client, err := ssh.Dial(
		"tcp",
		server,
		&ssh.ClientConfig{
			User: user,
			Auth: auth,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

func (s *directSite) Dial(network string, addr string) (net.Conn, error) {
	s.log.Debugf("Dial(addr=%v)", addr)
	return net.Dial(network, addr)
}

func (s *directSite) DialServer(addr string) (net.Conn, error) {
	s.log.Debugf("DialServer(addr=%v)", addr)
	return s.Dial("tcp", addr)
}

func findServer(addr string, servers []services.Server) (*services.Server, error) {
	for i := range servers {
		srv := &servers[i]
		_, port, err := net.SplitHostPort(srv.Addr)
		if err != nil {
			log.Warningf("server %v(%v) has incorrect address format (%v)",
				srv.Addr, srv.Hostname, err.Error())
		} else {
			if (len(srv.Hostname) != 0) && (len(port) != 0) && (addr == srv.Hostname+":"+port || addr == srv.Addr) {
				return srv, nil
			}
		}
	}
	return nil, trace.NotFound("server %v is unknown", addr)
}
