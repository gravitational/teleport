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
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
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

	// ENV AWS credentials need to be set in order to enforce AWS CLI to
	// sign the request and provide Authorization Header where service-name and region-name are encoded.
	// When endpoint-url AWS CLI flag provides the destination AWS API address is override by endpoint-url value.
	// Teleport AWS Signing APP will resolve aws-service and aws-region to the proper Amazon API URL.
	generatedAWSCred, err := genAndSetAWSCredentials()
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

	addr, err := utils.ParseAddr(lp.GetAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	url := url.URL{
		Path:   "/",
		Host:   fmt.Sprintf("%s:%d", "localhost", addr.Port(0)),
		Scheme: "https",
	}

	endpointFlag := fmt.Sprintf("--endpoint-url=%s", url.String())
	bundleFlag := fmt.Sprintf("--ca-bundle=%s", tmpCert.getCAPath())

	args := append([]string{}, cf.AWSCommandArgs...)
	args = append(args, endpointFlag)
	args = append(args, bundleFlag)
	cmd := exec.Command(awsCLIBinaryName, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// genAndSetAWSCredentials generates and returns fake AWS credential that are used
// for signing an AWS request during aws CLI call and verified on local AWS proxy side.
func genAndSetAWSCredentials() (*credentials.Credentials, error) {
	id := uuid.NewUUID().String()
	secret := uuid.NewUUID().String()
	if err := setFakeAWSEnvCredentials(id, secret); err != nil {
		return nil, trace.Wrap(err)
	}
	return credentials.NewStaticCredentials(id, secret, ""), nil
}

func createLocalAWSCLIProxy(cf *CLIConf, tc *client.TeleportClient, cred *credentials.Credentials, localCerts tls.Certificate) (*alpnproxy.LocalProxy, error) {
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
	listener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		Certificates: []tls.Certificate{
			localCerts,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		Listener:           listener,
		RemoteProxyAddr:    tc.WebProxyAddr,
		Protocol:           alpncommon.ProtocolHTTP,
		InsecureSkipVerify: cf.InsecureSkipVerify,
		ParentContext:      cf.Context,
		SNI:                address.Host(),
		AWSCredentials:     cred,
		Certs:              []tls.Certificate{appCerts},
	})
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return nil, trace.NewAggregate(err, cerr)
		}
		return nil, trace.Wrap(err)
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

func printAWSRoles(roles awsutils.Roles) {
	if len(roles) == 0 {
		return
	}

	roles.Sort()

	t := asciitable.MakeTable([]string{"Role Name", "Role ARN"})
	for _, role := range roles {
		// Use role.Display for role names to match what AWS web console shows.
		t.AddRow([]string{role.Display, role.ARN})
	}
	fmt.Println(t.AsBuffer().String())

}

func setFakeAWSEnvCredentials(accessKeyID, secretKey string) error {
	if err := os.Setenv("AWS_ACCESS_KEY_ID", accessKeyID); err != nil {
		return trace.Wrap(err)
	}
	if err := os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getARNFromFlags(cf *CLIConf, profile *client.ProfileStatus, app types.Application) (string, error) {
	// Filter AWS roles by AWS account ID. If AWS account ID is empty, all
	// roles are returned.
	roles := awsutils.FilterAWSRoles(profile.AWSRolesARNs, app.GetAWSAccountID())

	if cf.AWSRole == "" {
		if len(roles) == 1 {
			log.Infof("AWS Role %v is selected by default as it is the only role configured for this AWS app.", roles[0].Display)
			return roles[0].ARN, nil
		}

		printAWSRoles(roles)
		return "", trace.BadParameter("--aws-role flag is required")
	}

	// Match by role ARN.
	if awsarn.IsARN(cf.AWSRole) {
		if role, found := roles.FindRoleByARN(cf.AWSRole); found {
			return role.ARN, nil
		}

		printAWSRoles(roles)
		return "", trace.NotFound("failed to find the %q role ARN", cf.AWSRole)
	}

	// Match by role name.
	rolesMatched := roles.FindRolesByName(cf.AWSRole)
	switch len(rolesMatched) {
	case 1:
		return rolesMatched[0].ARN, nil
	case 0:
		printAWSRoles(roles)
		return "", trace.NotFound("failed to find the %q role name", cf.AWSRole)
	default:
		// Print roles matched the provided role name.
		printAWSRoles(rolesMatched)
		return "", trace.BadParameter("provided role name %q is ambiguous, please specify full role ARN", cf.AWSRole)
	}
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

	f, err := ioutil.TempFile("", "*_aws_local_proxy_cert.pem")
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
