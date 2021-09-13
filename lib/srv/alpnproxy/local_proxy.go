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

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	appaws "github.com/gravitational/teleport/lib/srv/app/aws"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/agentconn"
)

// LocalProxy allows upgrading incoming connection to TLS where custom TLS values are set SNI ALPN and
// updated connection is forwarded to remote ALPN SNI teleport proxy service.
type LocalProxy struct {
	cfg     LocalProxyConfig
	context context.Context
	cancel  context.CancelFunc
}

// LocalProxyConfig is configuration for LocalProxy.
type LocalProxyConfig struct {
	// RemoteProxyAddr is the downstream destination address of remote ALPN proxy service.
	RemoteProxyAddr string
	// Protocol set for the upstream TLS connection.
	Protocol Protocol
	// Insecure turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool
	// Listener is listener running on local machine.
	Listener net.Listener
	// SNI is a ServerName value set for upstream TLS connection.
	SNI string
	// ParentContext is a parent context, used to signal global closure>
	ParentContext context.Context
	// SSHUser is a SSH user name.
	SSHUser string
	// SSHUserHost is user host requested by ssh subsystem.
	SSHUserHost string
	// SSHHostKeyCallback is the function type used for verifying server keys.
	SSHHostKeyCallback ssh.HostKeyCallback
	// SSHTrustedCluster allows to select trusted cluster ssh subsystem request.
	SSHTrustedCluster string
	// Certs are the client certificates used to connect to the remote Teleport Proxy.
	Certs []tls.Certificate
	// AWSCredentials are AWS Credentials used by LocalProxy for request's signature verification.
	AWSCredentials *credentials.Credentials
}

// CheckAndSetDefaults verifies the constraints for LocalProxyConfig.
func (cfg *LocalProxyConfig) CheckAndSetDefaults() error {
	if cfg.RemoteProxyAddr == "" {
		return trace.BadParameter("missing remote proxy address")
	}
	if cfg.Protocol == "" {
		return trace.BadParameter("missing protocol")
	}
	if cfg.ParentContext == nil {
		return trace.BadParameter("missing parent context")
	}
	return nil
}

// NewLocalProxy creates a new instance of LocalProxy.
func NewLocalProxy(cfg LocalProxyConfig) (*LocalProxy, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.ParentContext)
	return &LocalProxy{
		cfg:     cfg,
		context: ctx,
		cancel:  cancel,
	}, nil
}

// SSHProxy is equivalent of `ssh -o 'ForwardAgent yes' -p port  %r@host -s proxy:%h:%p` but established SSH
// connection to RemoteProxyAddr is wrapped with TLS protocol.
func (l *LocalProxy) SSHProxy() error {
	upstreamConn, err := tls.Dial("tcp", l.cfg.RemoteProxyAddr, &tls.Config{
		NextProtos:         []string{string(l.cfg.Protocol)},
		InsecureSkipVerify: l.cfg.InsecureSkipVerify,
		ServerName:         l.cfg.SNI,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer upstreamConn.Close()

	sshAgent, err := getAgent()
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := makeSSHClient(upstreamConn, l.cfg.RemoteProxyAddr, &ssh.ClientConfig{
		User: l.cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(sshAgent.Signers),
		},
		HostKeyCallback: l.cfg.SSHHostKeyCallback,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	defer sess.Close()

	err = agent.ForwardToAgent(client, sshAgent)
	if err != nil {
		return trace.Wrap(err)
	}
	err = agent.RequestAgentForwarding(sess)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = sess.RequestSubsystem(proxySubsystemName(l.cfg.SSHUserHost, l.cfg.SSHTrustedCluster)); err != nil {
		return trace.Wrap(err)
	}
	if err := proxySession(l.context, sess); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func proxySubsystemName(userHost, cluster string) string {
	subsystem := fmt.Sprintf("proxy:%s", userHost)
	if cluster != "" {
		subsystem = fmt.Sprintf("%s@%s", subsystem, cluster)
	}
	return subsystem
}

func makeSSHClient(conn *tls.Conn, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	cc, chs, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.NewClient(cc, chs, reqs), nil
}

func proxySession(ctx context.Context, sess *ssh.Session) error {
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	errC := make(chan error)
	go func() {
		defer sess.Close()
		_, err := io.Copy(os.Stdout, stdout)
		errC <- err
	}()
	go func() {
		defer sess.Close()
		_, err := io.Copy(stdin, os.Stdin)
		errC <- err
	}()
	go func() {
		defer sess.Close()
		_, err := io.Copy(os.Stderr, stderr)
		errC <- err
	}()
	var errs []error
	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errC:
			if err != nil && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
}

// Start starts the LocalProxy.
func (l *LocalProxy) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.cfg.Listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			log.WithError(err).Errorf("Faield to accept client connection.")
			return trace.Wrap(err)
		}
		go func() {
			if err := l.handleDownstreamConnection(ctx, conn, l.cfg.SNI); err != nil {
				log.WithError(err).Errorf("Failed to handle connection.")
			}
		}()
	}
}

// GetAddr returns the LocalProxy listener address.
func (l *LocalProxy) GetAddr() string {
	return l.cfg.Listener.Addr().String()
}

// handleDownstreamConnection proxies the downstreamConn (connection established to the local proxy) and forward the
// traffic to the upstreamConn (TLS connection to remote host).
func (l *LocalProxy) handleDownstreamConnection(ctx context.Context, downstreamConn net.Conn, serverName string) error {
	defer downstreamConn.Close()
	upstreamConn, err := tls.Dial("tcp", l.cfg.RemoteProxyAddr, &tls.Config{
		NextProtos:         []string{string(l.cfg.Protocol)},
		InsecureSkipVerify: l.cfg.InsecureSkipVerify,
		ServerName:         serverName,
		Certificates:       l.cfg.Certs,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer upstreamConn.Close()

	errC := make(chan error, 2)
	go func() {
		defer downstreamConn.Close()
		defer upstreamConn.Close()
		_, err := io.Copy(downstreamConn, upstreamConn)
		errC <- err
	}()
	go func() {
		defer downstreamConn.Close()
		defer upstreamConn.Close()
		_, err := io.Copy(upstreamConn, downstreamConn)
		errC <- err
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return trace.NewAggregate(append(errs, ctx.Err())...)
		case err := <-errC:
			if err != nil && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
}

func getAgent() (agent.ExtendedAgent, error) {
	agentSocket := os.Getenv(teleport.SSHAuthSock)
	if agentSocket == "" {
		return nil, trace.NotFound("failed to connect to SSH agent, %s env var not set", teleport.SSHAuthSock)
	}

	conn, err := agentconn.Dial(agentSocket)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent.NewClient(conn), nil
}

func (l *LocalProxy) Close() error {
	l.cancel()
	if l.cfg.Listener != nil {
		if err := l.cfg.Listener.Close(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// StartAWSAccessProxy starts the local AWS CLI proxy.
func (l *LocalProxy) StartAWSAccessProxy(ctx context.Context) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			NextProtos:         []string{string(l.cfg.Protocol)},
			InsecureSkipVerify: l.cfg.InsecureSkipVerify,
			ServerName:         l.cfg.SNI,
			Certificates:       l.cfg.Certs,
		},
	}
	proxy := &httputil.ReverseProxy{
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = "https"
			outReq.URL.Host = l.cfg.RemoteProxyAddr
		},
		Transport: tr,
	}
	err := http.Serve(l.cfg.Listener, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := l.checkAccess(req); err != nil {
			rw.WriteHeader(http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(rw, req)
	}))
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// checkAccess validates the request signature ensuring that the request originates from tsh aws command execution
// AWS CLI signs the request with random generated credentials that are passed to LocalProxy by
// the AWSCredentials LocalProxyConfig configuration.
func (l *LocalProxy) checkAccess(req *http.Request) error {
	sigV4, err := appaws.ParseSigV4(req.Header.Get("Authorization"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	// Read the request body and replace the body ready with a new reader that will allow reading the body again
	// by HTTP Transport.
	payload, err := appaws.GetAndReplaceReqBody(req)
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy := req.Clone(context.Background())

	// Remove all the headers that are not present in awsCred.SignedHeaders.
	filterSingedHeaders(reqCopy, sigV4.SignedHeaders)

	// Get the date that was used to create the signature of the original request
	// originated from AWS CLI and reuse it as a timestamp during request signing call.
	t, err := time.Parse(appaws.AmzDateTimeFormat, reqCopy.Header.Get(appaws.AmzDateHeader))
	if err != nil {
		return trace.BadParameter(err.Error())
	}

	signer := v4.NewSigner(l.cfg.AWSCredentials)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), sigV4.Service, sigV4.Region, t)
	if err != nil {
		return trace.Wrap(err)
	}

	localSigV4, err := appaws.ParseSigV4(reqCopy.Header.Get("Authorization"))
	if err != nil {
		return trace.Wrap(err)
	}

	// Compare the origin request AWS SigV4 signature with the signature calculated in LocalProxy based on
	// AWSCredentials taken from LocalProxyConfig.
	if sigV4.Signature != localSigV4.Signature {
		return trace.AccessDenied("signature verification failed")
	}
	return nil
}

// filterSingedHeaders removes request headers that are not in the signedHeaders list.
func filterSingedHeaders(r *http.Request, signedHeaders []string) {
	header := make(http.Header)
	for _, v := range signedHeaders {
		ck := textproto.CanonicalMIMEHeaderKey(v)
		val, ok := r.Header[ck]
		if ok {
			header[ck] = val
		}
	}
	r.Header = header
}
