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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
)

const (
	awsCLIBinaryName = "aws"
)

func onAWS(cf *CLIConf) error {
	// create self-signed local cert AWS LocalProxy listener cert
	// and pass CA to AWS CLI by --ca-bundle flag to enforce HTTPS
	// protocol communication between AWS CLI <-> LocalProxy internal.
	tmpCert, err := newTempSelfSignedLocalCert()
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := tmpCert.Clean(); err != nil {
			log.WithError(err).Errorf(
				"Failed to clean temporary self-signed local proxy cert %q.", tmpCert.getCAPath())
		}
	}()

	generatedAWSCred, err := generateAWSCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := createLocalAWSCLIProxy(cf, tc, generatedAWSCred, tmpCert.getCert())
	if err != nil {
		return trace.Wrap(err)
	}
	defer lp.Close()
	go func() {
		if err := lp.StartAWSAccessProxy(cf.Context); err != nil {
			log.WithError(err).Errorf("Failed to start local proxy.")
		}
	}()

	// Setup the command to run.
	cmd := exec.Command(awsCLIBinaryName, cf.AWSCommandArgs...)

	credValues, err := generatedAWSCred.Get()
	if err != nil {
		return trace.Wrap(err)
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", credValues.AccessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", credValues.SecretAccessKey),
		fmt.Sprintf("AWS_CA_BUNDLE=%s", tmpCert.getCAPath()),
		fmt.Sprintf("HTTPS_PROXY=%s", "http://"+lp.GetForwardProxyAddr()),
		fmt.Sprintf("https_proxy=%s", "http://"+lp.GetForwardProxyAddr()),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// generateAWSCredentials generates and returns fake AWS credential that are used
// for signing an AWS request during aws CLI call and verified on local AWS proxy side.
func generateAWSCredentials() (*credentials.Credentials, error) {
	id := uuid.New().String()
	secret := uuid.New().String()
	return credentials.NewStaticCredentials(id, secret, ""), nil
}

func createLocalAWSCLIProxy(cf *CLIConf, tc *client.TeleportClient, cred *credentials.Credentials, ca tls.Certificate) (*alpnproxy.LocalProxy, error) {
	awsApp, err := pickActiveAWSApp(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appCerts, err := loadAWSAppCertificate(tc, awsApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	address, err := utils.ParseAddr(tc.WebProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	forwardProxyListenAddr := "localhost:0"
	listenAddr := "localhost:0"
	if cf.LocalProxyPort != "" {
		port, err := strconv.Atoi(cf.LocalProxyPort)
		if err != nil {
			return nil, trace.BadParameter("invalid local proxy port %s", cf.LocalProxyPort)
		}

		forwardProxyListenAddr = fmt.Sprintf("localhost:%d", port)
		listenAddr = fmt.Sprintf("localhost:%d", port+1)
	}

	// Note that the created forward proxy serves HTTP instead of HTTPS, to
	// eliminate the need to install temporary CA on the system.
	//
	// Normally a HTTP connection is considered insecure as transport is not
	// encrypted. Thus it is very important that the forward proxy is only used
	// for proxying HTTPS requests, so the tunneled data is still encrypted
	// between client and receiver. The initial "CONNECT" request before
	// tunneling is not encrypted but it contains minimal information like host
	// and user agent. In addition, the forward proxy is localhost only so the
	// transport does not leave the local network.
	forwardProxyListener, err := net.Listen("tcp", forwardProxyListenAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create listener for receiving AWS request tunneled from forward proxy.
	listener, err := alpnproxy.NewHTTPSListenerReceiver(alpnproxy.HTTPSListenerReceiverConfig{
		ListenAddr: listenAddr,
		CA:         ca,
		Want:       alpnproxy.WantAWSRequests,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		Listener:             listener,
		ForwardProxyListener: forwardProxyListener,
		RemoteProxyAddr:      tc.WebProxyAddr,
		Protocol:             alpncommon.ProtocolHTTP,
		InsecureSkipVerify:   cf.InsecureSkipVerify,
		ParentContext:        cf.Context,
		SNI:                  address.Host(),
		AWSCredentials:       cred,
		Certs:                []tls.Certificate{appCerts},
	})
	if err != nil {
		return nil, trace.NewAggregate(
			err,
			listener.Close(),
			forwardProxyListener.Close(),
		)
	}
	return lp, nil
}

func loadAWSAppCertificate(tc *client.TeleportClient, appName string) (tls.Certificate, error) {
	key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cc, ok := key.AppTLSCerts[appName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("please login into AWS Console App 'tsh app login' first")
	}
	cert, err := tls.X509KeyPair(cc, key.Priv)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if len(cert.Certificate) < 1 {
		return tls.Certificate{}, trace.NotFound("invalid certificate length")
	}
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if time.Until(x509cert.NotAfter) < 5*time.Second {
		return tls.Certificate{}, trace.BadParameter(
			"AWS application %s certificate has expired, please re-login to the app using 'tsh app login'",
			appName)
	}
	return cert, nil
}

func printArrayAs(arr []string, columnName string) {
	sort.Strings(arr)
	if len(arr) == 0 {
		return
	}
	t := asciitable.MakeTable([]string{columnName})
	for _, v := range arr {
		t.AddRow([]string{v})
	}
	fmt.Println(t.AsBuffer().String())

}

func getARNFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	if cf.AWSRole == "" {
		printArrayAs(profile.AWSRolesARNs, "Available Role ARNs")
		return "", trace.BadParameter("--aws-role flag is required")
	}
	for _, v := range profile.AWSRolesARNs {
		if v == cf.AWSRole {
			return v, nil
		}
	}

	roleNameToARN := make(map[string]string)
	for _, v := range profile.AWSRolesARNs {
		arn, err := awsarn.Parse(v)
		if err != nil {
			return "", trace.Wrap(err)
		}
		// Example of the ANR Resource: 'role/EC2FullAccess' or 'role/path/to/customrole'
		parts := strings.Split(arn.Resource, "/")
		if len(parts) < 1 || parts[0] != "role" {
			continue
		}
		roleName := strings.Join(parts[1:], "/")

		if val, ok := roleNameToARN[roleName]; ok && cf.AWSRole == roleName {
			return "", trace.BadParameter(
				"provided role name %q is ambiguous between %q and %q ARNs, please specify full role ARN",
				cf.AWSRole, val, arn.String())
		}
		roleNameToARN[roleName] = arn.String()
	}

	roleARN, ok := roleNameToARN[cf.AWSRole]
	if !ok {
		printArrayAs(profile.AWSRolesARNs, "Available Role ARNs")
		printArrayAs(mapKeysToSlice(roleNameToARN), "Available Role Names")
		inputType := "ARN"
		if !awsarn.IsARN(cf.AWSRole) {
			inputType = "name"
		}
		return "", trace.NotFound("failed to find the %q role %s", cf.AWSRole, inputType)
	}
	return roleARN, nil
}

func mapKeysToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type tempSelfSignedLocalCert struct {
	cert   tls.Certificate
	caFile *os.File
}

func newTempSelfSignedLocalCert() (*tempSelfSignedLocalCert, error) {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"Teleport"},
	}, []string{"localhost"}, defaults.CATTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f, err := os.CreateTemp("", "*_aws_local_proxy_cert.pem")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := io.Copy(f, bytes.NewReader(caCert)); err != nil {
		return nil, trace.Wrap(err)
	}
	return &tempSelfSignedLocalCert{
		cert:   cert,
		caFile: f,
	}, nil
}

func (t *tempSelfSignedLocalCert) getCAPath() string {
	return t.caFile.Name()
}

func (t *tempSelfSignedLocalCert) getCert() tls.Certificate {
	return t.cert
}

func (t tempSelfSignedLocalCert) Clean() error {
	if err := t.caFile.Close(); err != nil {
		return trace.Wrap(err)
	}
	if err := os.Remove(t.caFile.Name()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func pickActiveAWSApp(cf *CLIConf) (string, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return "", trace.NotFound("Please login to AWS app using 'tsh app login' first")
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return "", trace.NotFound("Please login to AWS app using 'tsh app login' first")
			}
			return "", trace.Wrap(err)
		}
		if app.AWSRoleARN == "" {
			return "", trace.BadParameter(
				"Selected app %q is not an AWS application", name,
			)
		}
		return name, nil
	}

	awsApps := getAWSAppsName(profile.Apps)
	if len(awsApps) == 0 {
		return "", trace.NotFound("Please login to AWS App using 'tsh app login' first")
	}
	if len(awsApps) > 1 {
		names := strings.Join(awsApps, ", ")
		return "", trace.BadParameter(
			"Multiple AWS apps are available (%v), please specify one using --app CLI argument", names,
		)
	}
	return awsApps[0], nil
}

func findApp(apps []tlsca.RouteToApp, name string) (*tlsca.RouteToApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, trace.NotFound("failed to find app with %q name", name)
}

func getAWSAppsName(apps []tlsca.RouteToApp) []string {
	var out []string
	for _, app := range apps {
		if app.AWSRoleARN != "" {
			out = append(out, app.Name)
		}
	}
	return out
}
