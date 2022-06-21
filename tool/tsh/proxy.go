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

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// onProxyCommandSSH creates a local ssh proxy.
// In cases of TLS Routing the connection is established to the WebProxy with teleport-proxy-ssh ALPN protocol.
// and all ssh traffic is forwarded through the local ssh proxy.
//
// If proxy doesn't support TLS Routing the onProxyCommandSSH is used as ProxyCommand to remove proxy/site prefixes
// from destination node address to support multiple platform where 'cut -d' command is not provided.
// For more details please look at: Generate Windows-compatible OpenSSH config https://github.com/gravitational/teleport/pull/7848
func onProxyCommandSSH(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyParams, err := getSSHProxyParams(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(tc.JumpHosts) > 0 {
		err := setupJumpHost(cf, tc, *proxyParams)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if proxyParams.tlsRouting {
		return trace.Wrap(sshProxyWithTLSRouting(cf, tc, *proxyParams))
	}

	return trace.Wrap(sshProxy(tc, *proxyParams))
}

// getSSHProxyParams prepares parameters for establishing an SSH proxy.
func getSSHProxyParams(cf *CLIConf, tc *libclient.TeleportClient) (*sshProxyParams, error) {
	targetHost, targetPort, err := net.SplitHostPort(tc.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Without jump hosts, we will be connecting to the current Teleport client
	// proxy the user is logged into.
	if len(tc.JumpHosts) == 0 {
		proxyHost, proxyPort := tc.SSHProxyHostPort()
		if tc.TLSRoutingEnabled {
			proxyHost, proxyPort = tc.WebProxyHostPort()
		}
		return &sshProxyParams{
			proxyHost:   proxyHost,
			proxyPort:   strconv.Itoa(proxyPort),
			targetHost:  cleanTargetHost(targetHost, tc.WebProxyHost(), tc.SiteName),
			targetPort:  targetPort,
			clusterName: tc.SiteName,
			tlsRouting:  tc.TLSRoutingEnabled,
		}, nil
	}
	// When jump host is specified, we will be connecting to the jump host's
	// proxy directly. Call its ping endpoint to figure out the cluster details
	// such as cluster name, SSH proxy address, etc.
	ping, err := webclient.Find(&webclient.Config{
		Context:   cf.Context,
		ProxyAddr: tc.JumpHosts[0].Addr.Addr,
		Insecure:  tc.InsecureSkipVerify,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshProxyHost, sshProxyPort, err := ping.Proxy.SSHProxyHostPort()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sshProxyParams{
		proxyHost:   sshProxyHost,
		proxyPort:   sshProxyPort,
		targetHost:  targetHost,
		targetPort:  targetPort,
		clusterName: ping.ClusterName,
		tlsRouting:  ping.Proxy.TLSRoutingEnabled,
	}, nil
}

// setupJumpHost configures the client for connecting to the jump host's proxy.
func setupJumpHost(cf *CLIConf, tc *libclient.TeleportClient, sp sshProxyParams) error {
	return tc.WithoutJumpHosts(func(tc *libclient.TeleportClient) error {
		// Fetch certificate for the leaf cluster. This allows users to log
		// in once into the root cluster and let the proxy handle fetching
		// certificates for leaf clusters automatically.
		err := tc.LoadKeyForClusterWithReissue(cf.Context, sp.clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		// Update known_hosts with the leaf proxy's CA certificate, otherwise
		// users will be prompted to manually accept the key.
		err = tc.UpdateKnownHosts(cf.Context, sp.proxyHost, sp.clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		// We'll be connecting directly to the leaf cluster so make sure agent
		// loads correct host CA.
		tc.LocalAgent().UpdateCluster(sp.clusterName)
		return nil
	})
}

// sshProxyParams combines parameters for establishing an SSH proxy used
// as a ProxyCommand for SSH clients.
type sshProxyParams struct {
	// proxyHost is the Teleport proxy host name.
	proxyHost string
	// proxyPort is the Teleport proxy port.
	proxyPort string
	// targetHost is the target SSH node host name.
	targetHost string
	// targetPort is the target SSH node port.
	targetPort string
	// clusterName is the cluster where the SSH node resides.
	clusterName string
	// tlsRouting is true if the Teleport proxy has TLS routing enabled.
	tlsRouting bool
}

// cleanTargetHost cleans the targetHost and remote site and proxy suffixes.
// Before the `cut -d` command was used for this purpose but to support multi-platform OpenSSH clients the logic
// it was moved tsh proxy ssh command.
// For more details please look at: Generate Windows-compatible OpenSSH config https://github.com/gravitational/teleport/pull/7848
func cleanTargetHost(targetHost, proxyHost, siteName string) string {
	targetHost = strings.TrimSuffix(targetHost, "."+proxyHost)
	targetHost = strings.TrimSuffix(targetHost, "."+siteName)
	return targetHost
}

func sshProxyWithTLSRouting(cf *CLIConf, tc *libclient.TeleportClient, params sshProxyParams) error {
	pool, err := tc.LocalAgent().ClientCertPool(params.clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig := &tls.Config{
		RootCAs: pool,
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    net.JoinHostPort(params.proxyHost, params.proxyPort),
		Protocols:          []alpncommon.Protocol{alpncommon.ProtocolProxySSH},
		InsecureSkipVerify: cf.InsecureSkipVerify,
		ParentContext:      cf.Context,
		SNI:                params.proxyHost,
		SSHUser:            tc.HostLogin,
		SSHUserHost:        fmt.Sprintf("%s:%s", params.targetHost, params.targetPort),
		SSHHostKeyCallback: tc.HostKeyCallback,
		SSHTrustedCluster:  params.clusterName,
		ClientTLSConfig:    tlsConfig,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer lp.Close()
	if err := lp.SSHProxy(cf.Context, tc.LocalAgent()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func sshProxy(tc *libclient.TeleportClient, params sshProxyParams) error {
	sshPath, err := getSSHPath()
	if err != nil {
		return trace.Wrap(err)
	}
	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	knownHostsPath := keypaths.KnownHostsPath(keysDir)

	args := []string{
		"-A",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-p", params.proxyPort,
		params.proxyHost,
		"-s",
		fmt.Sprintf("proxy:%s:%s@%s", params.targetHost, params.targetPort, params.clusterName),
	}

	if tc.HostLogin != "" {
		args = append([]string{"-l", tc.HostLogin}, args...)
	}

	log.Debugf("Executing proxy command: %v %v.", sshPath, strings.Join(args, " "))

	child := exec.Command(sshPath, args...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	return trace.Wrap(child.Run())
}

func onProxyCommandDB(cf *CLIConf) error {
	client, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	routeToDatabase, err := pickActiveDatabase(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if routeToDatabase.Protocol == defaults.ProtocolSnowflake && !cf.LocalProxyTunnel {
		return trace.BadParameter("Snowflake proxy works only in the tunnel mode. Please add --tunnel flag to enable it")
	}

	rootCluster, err := client.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := libclient.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	addr := "localhost:0"
	if cf.LocalProxyPort != "" {
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.WithError(err).Warnf("Failed to close listener.")
		}
	}()

	proxyOpts, err := prepareLocalProxyOptions(&localProxyConfig{
		cliConf:          cf,
		teleportClient:   client,
		profile:          profile,
		routeToDatabase:  routeToDatabase,
		listener:         listener,
		localProxyTunnel: cf.LocalProxyTunnel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := mkLocalProxy(cf.Context, proxyOpts)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-cf.Context.Done()
		lp.Close()
	}()

	if cf.LocalProxyTunnel {
		addr, err := utils.ParseAddr(lp.GetAddr())
		if err != nil {
			return trace.Wrap(err)
		}
		cmd, err := dbcmd.NewCmdBuilder(client, profile, routeToDatabase, cf.SiteName,
			dbcmd.WithLocalProxy("localhost", addr.Port(0), ""),
			dbcmd.WithNoTLS(),
			dbcmd.WithLogger(log),
		).GetConnectCommand()
		if err != nil {
			return trace.Wrap(err)
		}
		err = dbProxyAuthTpl.Execute(os.Stdout, map[string]string{
			"database": routeToDatabase.ServiceName,
			"type":     dbProtocolToText(routeToDatabase.Protocol),
			"cluster":  profile.Cluster,
			"command":  fmt.Sprintf("%s %s", strings.Join(cmd.Env, " "), cmd.String()),
			"address":  listener.Addr().String(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err = dbProxyTpl.Execute(os.Stdout, map[string]string{
			"database": routeToDatabase.ServiceName,
			"address":  listener.Addr().String(),
			"ca":       profile.CACertPathForCluster(rootCluster),
			"cert":     profile.DatabaseCertPathForCluster(cf.SiteName, routeToDatabase.ServiceName),
			"key":      profile.KeyPath(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	defer lp.Close()
	if err := lp.Start(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type localProxyOpts struct {
	proxyAddr string
	listener  net.Listener
	protocols []alpncommon.Protocol
	insecure  bool
	certFile  string
	keyFile   string
}

// protocol returns the first protocol or string if configuration doesn't contain any protocols.
func (l *localProxyOpts) protocol() string {
	if len(l.protocols) == 0 {
		return ""
	}
	return string(l.protocols[0])
}

func mkLocalProxy(ctx context.Context, opts localProxyOpts) (*alpnproxy.LocalProxy, error) {
	alpnProtocol, err := alpncommon.ToALPNProtocol(opts.protocol())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	address, err := utils.ParseAddr(opts.proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := mkLocalProxyCerts(opts.certFile, opts.keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify: opts.insecure,
		RemoteProxyAddr:    opts.proxyAddr,
		Protocols:          append([]alpncommon.Protocol{alpnProtocol}, opts.protocols...),
		Listener:           opts.listener,
		ParentContext:      ctx,
		SNI:                address.Host(),
		Certs:              certs,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lp, nil
}

func mkLocalProxyCerts(certFile, keyFile string) ([]tls.Certificate, error) {
	if certFile == "" && keyFile == "" {
		return []tls.Certificate{}, nil
	}
	if (certFile == "" && keyFile != "") || (certFile != "" && keyFile == "") {
		return nil, trace.BadParameter("both --cert-file and --key-file are required")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []tls.Certificate{cert}, nil
}

func onProxyCommandApp(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	appCerts, err := loadAppCertificate(tc, cf.AppName)
	if err != nil {
		return trace.Wrap(err)
	}

	address, err := utils.ParseAddr(tc.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	addr := "localhost:0"
	if cf.LocalProxyPort != "" {
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		Listener:           listener,
		RemoteProxyAddr:    tc.WebProxyAddr,
		Protocols:          []alpncommon.Protocol{alpncommon.ProtocolHTTP},
		InsecureSkipVerify: cf.InsecureSkipVerify,
		ParentContext:      cf.Context,
		SNI:                address.Host(),
		Certs:              []tls.Certificate{appCerts},
	})
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	fmt.Printf("Proxying connections to %s on %v\n", cf.AppName, lp.GetAddr())

	go func() {
		<-cf.Context.Done()
		lp.Close()
	}()

	defer lp.Close()
	if err = lp.Start(cf.Context); err != nil {
		log.WithError(err).Errorf("Failed to start local proxy.")
	}

	return nil
}

// onProxyCommandAWS creates local proxes for AWS apps.
func onProxyCommandAWS(cf *CLIConf) error {
	awsApp, err := pickActiveAWSApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = awsApp.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := awsApp.Close(); err != nil {
			log.WithError(err).Error("Failed to close AWS app.")
		}
	}()

	envVars, err := awsApp.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}

	templateData := map[string]interface{}{
		"envVars":     envVars,
		"address":     awsApp.GetForwardProxyAddr(),
		"endpointURL": awsApp.GetEndpointURL(),
	}

	template := awsHTTPSProxyTemplate
	if cf.AWSEndpointURLMode {
		template = awsEndpointURLProxyTemplate
	}

	if err = template.Execute(os.Stdout, templateData); err != nil {
		return trace.Wrap(err)
	}

	<-cf.Context.Done()
	return nil
}

// loadAppCertificate loads the app certificate for the provided app.
func loadAppCertificate(tc *libclient.TeleportClient, appName string) (tls.Certificate, error) {
	key, err := tc.LocalAgent().GetKey(tc.SiteName, libclient.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cc, ok := key.AppTLSCerts[appName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("please login into the application first. 'tsh app login'")
	}
	cert, err := tls.X509KeyPair(cc, key.Priv)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	expiresAt, err := getTLSCertExpireTime(cert)
	if err != nil {
		return tls.Certificate{}, trace.WrapWithMessage(err, "invalid certificate - please login to the application again. 'tsh app login'")
	}
	if time.Until(expiresAt) < 5*time.Second {
		return tls.Certificate{}, trace.BadParameter(
			"application %s certificate has expired, please re-login to the app using 'tsh app login'",
			appName)
	}
	return cert, nil
}

// getTLSCertExpireTime returns the certificate NotAfter time.
func getTLSCertExpireTime(cert tls.Certificate) (time.Time, error) {
	if len(cert.Certificate) < 1 {
		return time.Time{}, trace.NotFound("invalid certificate length")
	}
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	return x509cert.NotAfter, nil
}

// dbProxyTpl is the message that gets printed to a user when a database proxy is started.
var dbProxyTpl = template.Must(template.New("").Parse(`Started DB proxy on {{.address}}

Use following credentials to connect to the {{.database}} proxy:
  ca_file={{.ca}}
  cert_file={{.cert}}
  key_file={{.key}}
`))

func dbProtocolToText(protocol string) string {
	switch protocol {
	case defaults.ProtocolPostgres:
		return "PostgreSQL"
	case defaults.ProtocolCockroachDB:
		return "CockroachDB"
	case defaults.ProtocolMySQL:
		return "MySQL"
	case defaults.ProtocolMongoDB:
		return "MongoDB"
	case defaults.ProtocolRedis:
		return "Redis"
	case defaults.ProtocolSQLServer:
		return "SQL Server"
	}
	return ""
}

// dbProxyAuthTpl is the message that's printed for an authenticated db proxy.
var dbProxyAuthTpl = template.Must(template.New("").Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.

Use the following command to connect to the database:
  $ {{.command}}
`))

// awsHTTPSProxyTemplate is the message that gets printed to a user when an
// HTTPS proxy is started.
var awsHTTPSProxyTemplate = template.Must(template.New("").Parse(
	`Started AWS proxy on {{.envVars.HTTPS_PROXY}}.

Use the following credentials and HTTPS proxy setting to connect to the proxy:
  AWS_ACCESS_KEY_ID={{.envVars.AWS_ACCESS_KEY_ID}}
  AWS_SECRET_ACCESS_KEY={{.envVars.AWS_SECRET_ACCESS_KEY}}
  AWS_CA_BUNDLE={{.envVars.AWS_CA_BUNDLE}}
  HTTPS_PROXY={{.envVars.HTTPS_PROXY}}
`))

// awsEndpointURLProxyTemplate is the message that gets printed to a user when an
// AWS endpoint URL proxy is started.
var awsEndpointURLProxyTemplate = template.Must(template.New("").Parse(
	`Started AWS proxy which serves as an AWS endpoint URL at {{.endpointURL}}.

In addition to the endpoint URL, use the following credentials to connect to the proxy:
  AWS_ACCESS_KEY_ID={{.envVars.AWS_ACCESS_KEY_ID}}
  AWS_SECRET_ACCESS_KEY={{.envVars.AWS_SECRET_ACCESS_KEY}}
  AWS_CA_BUNDLE={{.envVars.AWS_CA_BUNDLE}}
`))
