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
	"crypto/x509/pkix"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
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
			log.WithError(err).Errorf("Failed clean temporary self-signed local proxy cert.")
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

func genAndSetAWSCredentials() (*credentials.Credentials, error) {
	id := uuid.NewUUID().String()
	secret := uuid.NewUUID().String()
	if err := setFakeAWSEnvCredentials(id, secret); err != nil {
		return nil, trace.Wrap(err)
	}
	return credentials.NewStaticCredentials(id, secret, ""), nil
}

func createLocalAWSCLIProxy(cf *CLIConf, tc *client.TeleportClient, cred *credentials.Credentials, localCerts tls.Certificate) (*alpnproxy.LocalProxy, error) {
	if !tc.ALPNSNIListenerEnabled {
		return nil, trace.NotFound("remote Teleport Proxy doesn't support AWS CLI access protocol")
	}

	appCerts, err := loadAWSAppCertificate(tc)
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
		Protocol:           alpncommon.ProtocolAWSCLI,
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

func loadAWSAppCertificate(tc *client.TeleportClient) (tls.Certificate, error) {
	if tc.CurrentAWSCLIApp == "" {
		return tls.Certificate{}, trace.NotFound("please login into AWS Console App 'tsh app login' first")
	}
	key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cc, ok := key.AppTLSCerts[tc.CurrentAWSCLIApp]
	if !ok {
		return tls.Certificate{}, trace.NotFound("please login into AWS Console App 'tsh app login' first")
	}
	cert, err := tls.X509KeyPair(cc, key.Priv)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return cert, nil
}

func validateARNRole(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, arnRole string) error {
	if ok := awsarn.IsARN(arnRole); !ok {
		// User provided invalid formatted ARN role string, print all available ARN roles for the user and indicate
		// and indicate about invalid ARN format.
		printArrayAs(profile.AWSRolesARNs, "Available ARNs")
		return trace.BadParameter("invalid AWS ARN role format: %q", arnRole)
	}

	for _, v := range profile.AWSRolesARNs {
		if v == arnRole {
			return nil
		}
	}

	printArrayAs(profile.AWSRolesARNs, "Available ARNs")
	return trace.NotFound("user is not allowed to use selected AWS ARN role: %q.", arnRole)
}

func printArrayAs(validARNs []string, columnName string) {
	if len(validARNs) == 0 {
		return
	}
	t := asciitable.MakeTable([]string{columnName})
	for _, v := range validARNs {
		t.AddRow([]string{v})
	}
	fmt.Println(t.AsBuffer().String())

}

// findARNBasedOnRoleName tries to match roleName parameter with allowed user ARNs obtained from the Teleport API based on
// user roles profile. If there is a match the IAM role is created based on accountID and roleName fields.
func findARNBasedOnRoleName(profile *client.ProfileStatus, accountID, roleName string) (string, error) {
	var validRolesName []string
	for _, v := range profile.AWSRolesARNs {
		arn, err := awsarn.Parse(v)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// Example of the ANR Resource: 'role/EC2FullAccess'
		parts := strings.Split(arn.Resource, "/")
		if len(parts) < 1 || parts[0] != "role" {
			continue
		}

		roleNameWithPath := strings.Join(parts[1:], "/")
		if arn.AccountID == accountID {
			validRolesName = append(validRolesName, roleNameWithPath)
		}
		if arn.AccountID == accountID && roleNameWithPath == roleName {
			return arn.String(), nil
		}
	}
	if len(validRolesName) != 0 {
		printArrayAs(validRolesName, "Available Roles")
	}
	return "", trace.NotFound("failed to find role ARN based on AWSAccountID(%q) and RoleName(%q)", accountID, roleName)
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

func getARNFromFlags(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, app types.Application) (string, error) {
	if cf.AWSRoleARN == "" && cf.AWSRoleName == "" {
		return "", trace.BadParameter("either --aws-role-arn or --aws-role-name flag is required")
	}

	if cf.AWSRoleARN != "" {
		if err := validateARNRole(cf, tc, profile, cf.AWSRoleARN); err != nil {
			return "", trace.Wrap(err)
		}
		return cf.AWSRoleARN, nil
	}
	// Try to construct ARN value based on RoleName and APP AWSAccountID.
	accountID, ok := app.GetAllLabels()[constants.AWSAccountIDLabel]
	if !ok {
		// APP configuration doesn't contain an accountID value.
		return "", trace.BadParameter("the role name is ambiguous, please provide role ARN by --aws-role-arn flag")
	}
	var err error
	arn, err := findARNBasedOnRoleName(profile, accountID, cf.AWSRoleName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return arn, nil
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
