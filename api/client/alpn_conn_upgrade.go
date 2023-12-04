/*
Copyright 2022 Gravitational, Inc.

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
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/api/utils/tlsutils"
)

// IsALPNConnUpgradeRequired returns true if a tunnel is required through a HTTP
// connection upgrade for ALPN connections.
//
// The function makes a test connection to the Proxy Service and checks if the
// ALPN is supported. If not, the Proxy Service is likely behind an AWS ALB or
// some custom proxy services that strip out ALPN and SNI information on the
// way to our Proxy Service.
//
// In those cases, the Teleport client should make a HTTP "upgrade" call to the
// Proxy Service to establish a tunnel for the originally planned traffic to
// preserve the ALPN and SNI information.
func IsALPNConnUpgradeRequired(ctx context.Context, addr string, insecure bool, opts ...DialOption) bool {
	if result, ok := OverwriteALPNConnUpgradeRequirementByEnv(addr); ok {
		return result
	}

	var alpnDropped, pinnedCertRewritten bool
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		alpnDropped = testALPNDropped(ctx, addr, insecure, opts...)
	}()

	go func() {
		defer wg.Done()
		pinnedCertRewritten = testPinnedCertRewritten(ctx, addr, insecure, opts...)
	}()

	wg.Wait()
	return alpnDropped || pinnedCertRewritten
}

func testALPNDropped(ctx context.Context, addr string, insecure bool, opts ...DialOption) bool {
	// Use NewDialer which takes care of ProxyURL, and use a shorter I/O
	// timeout to avoid blocking caller.
	baseDialer := NewDialer(
		ctx,
		defaults.DefaultIdleTimeout,
		5*time.Second,
		append(opts,
			WithInsecureSkipVerify(insecure),
			WithALPNConnUpgrade(false),
		)...,
	)

	tlsConfig := &tls.Config{
		NextProtos:         []string{string(constants.ALPNSNIProtocolReverseTunnel)},
		InsecureSkipVerify: insecure,
	}
	testConn, err := tlsutils.TLSDial(ctx, baseDialer, "tcp", addr, tlsConfig)
	if err != nil {
		if isRemoteNoALPNError(err) {
			logrus.Debugf("ALPN connection upgrade required for %q: %v. No ALPN protocol is negotiated by the server.", addr, true)
			return true
		}
		if isUnadvertisedALPNError(err) {
			logrus.Debugf("ALPN connection upgrade required for %q: %v.", addr, err)
			return true
		}

		// If dialing TLS fails for any other reason, we assume connection
		// upgrade is not required so it will fallback to original connection
		// method.
		logrus.Infof("ALPN connection upgrade test failed for %q: %v.", addr, err)
		return false
	}
	defer testConn.Close()

	// Upgrade required when ALPN is not supported on the remote side so
	// NegotiatedProtocol comes back as empty.
	result := testConn.ConnectionState().NegotiatedProtocol == ""
	logrus.Debugf("ALPN connection upgrade ALPN negotiated for %q: %v.", addr, result)
	return result
}

func testPinnedCertRewritten(ctx context.Context, addr string, insecure bool, opts ...DialOption) bool {
	// Use NewDialer which takes care of ProxyURL, and use a shorter I/O
	// timeout to avoid blocking caller.
	baseDialer := NewDialer(
		ctx,
		defaults.DefaultIdleTimeout,
		5*time.Second,
		append(opts,
			WithALPNConnUpgrade(false),
		)...,
	)

	cert, err := tls.X509KeyPair([]byte(TLSRoutingTestPinnedCertPem), []byte(TLSRoutingTestPinnedKeyPem))
	if err != nil {
		// TODO explain
		return false
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM([]byte(TLSRoutingTestPinnedCertPem))

	tlsConfig := &tls.Config{
		NextProtos:   []string{constants.ALPNSNIProtocolPinnedCert},
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
		ServerName:   "tls." + constants.APIDomain,
	}
	testConn, err := tlsutils.TLSDial(ctx, baseDialer, "tcp", addr, tlsConfig)
	if err != nil {
		// Ignore other errors
		return strings.Contains(err.Error(), "failed to verify certificate: x509: certificate signed by unknown authority")
	}
	testConn.Close()
	return false
}

func isRemoteNoALPNError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "remote error" && strings.Contains(opErr.Err.Error(), "tls: no application protocol")
}

// isUnadvertisedALPNError returns true if the error indicates that the server
// returns an ALPN value that the client does not expect during TLS handshake.
//
// Reference:
// https://github.com/golang/go/blob/2639a17f146cc7df0778298c6039156d7ca68202/src/crypto/tls/handshake_client.go#L838
func isUnadvertisedALPNError(err error) bool {
	return strings.Contains(err.Error(), "tls: server selected unadvertised ALPN protocol")
}

// OverwriteALPNConnUpgradeRequirementByEnv overwrites ALPN connection upgrade
// requirement by environment variable.
//
// TODO(greedy52) DELETE in 15.0
func OverwriteALPNConnUpgradeRequirementByEnv(addr string) (bool, bool) {
	envValue := os.Getenv(defaults.TLSRoutingConnUpgradeEnvVar)
	if envValue == "" {
		return false, false
	}
	result := isALPNConnUpgradeRequiredByEnv(addr, envValue)
	logrus.WithField(defaults.TLSRoutingConnUpgradeEnvVar, envValue).Debugf("ALPN connection upgrade required for %q: %v.", addr, result)
	return result, true
}

// isALPNConnUpgradeRequiredByEnv checks if ALPN connection upgrade is required
// based on provided env value.
//
// The env value should contain a list of conditions separated by either ';' or
// ','. A condition is in format of either '<addr>=<bool>' or '<bool>'. The
// former specifies the upgrade requirement for a specific address and the
// later specifies the upgrade requirement for all other addresses. By default,
// upgrade is not required if target is not specified in the env value.
//
// Sample values:
// true
// <some.cluster.com>=yes,<another.cluster.com>=no
// 0,<some.cluster.com>=1
func isALPNConnUpgradeRequiredByEnv(addr, envValue string) bool {
	tokens := strings.FieldsFunc(envValue, func(r rune) bool {
		return r == ';' || r == ','
	})

	var upgradeRequiredForAll bool
	for _, token := range tokens {
		switch {
		case strings.ContainsRune(token, '='):
			if _, boolText, ok := strings.Cut(token, addr+"="); ok {
				upgradeRequiredForAddr, err := utils.ParseBool(boolText)
				if err != nil {
					logrus.Debugf("Failed to parse %v: %v", envValue, err)
				}
				return upgradeRequiredForAddr
			}

		default:
			if boolValue, err := utils.ParseBool(token); err != nil {
				logrus.Debugf("Failed to parse %v: %v", envValue, err)
			} else {
				upgradeRequiredForAll = boolValue
			}
		}
	}
	return upgradeRequiredForAll
}

// alpnConnUpgradeDialer makes an "HTTP" upgrade call to the Proxy Service then
// tunnels the connection with this connection upgrade.
type alpnConnUpgradeDialer struct {
	dialer    ContextDialer
	tlsConfig *tls.Config
	withPing  bool
}

// newALPNConnUpgradeDialer creates a new alpnConnUpgradeDialer.
func newALPNConnUpgradeDialer(dialer ContextDialer, tlsConfig *tls.Config, withPing bool) ContextDialer {
	return &alpnConnUpgradeDialer{
		dialer:    dialer,
		tlsConfig: tlsConfig,
		withPing:  withPing,
	}
}

// DialContext implements ContextDialer
func (d *alpnConnUpgradeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	logrus.Debugf("ALPN connection upgrade for %v.", addr)

	tlsConn, err := tlsutils.TLSDial(ctx, d.dialer, network, addr, d.tlsConfig.Clone())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upgradeURL := url.URL{
		Host:   addr,
		Scheme: "https",
		Path:   constants.WebAPIConnUpgrade,
	}

	conn, err := upgradeConnThroughWebAPI(tlsConn, upgradeURL, d.upgradeType())
	if err != nil {
		return nil, trace.NewAggregate(tlsConn.Close(), err)
	}
	return conn, nil
}

func (d *alpnConnUpgradeDialer) upgradeType() string {
	if d.withPing {
		return constants.WebAPIConnUpgradeTypeALPNPing
	}
	return constants.WebAPIConnUpgradeTypeALPN
}

func upgradeConnThroughWebAPI(conn net.Conn, api url.URL, upgradeType string) (net.Conn, error) {
	req, err := http.NewRequest(http.MethodGet, api.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Add(constants.WebAPIConnUpgradeHeader, upgradeType)
	req.Header.Add(constants.WebAPIConnUpgradeTeleportHeader, upgradeType)

	// Set "Connection" header to meet RFC spec:
	// https://datatracker.ietf.org/doc/html/rfc2616#section-14.42
	// Quote: "the upgrade keyword MUST be supplied within a Connection header
	// field (section 14.10) whenever Upgrade is present in an HTTP/1.1
	// message."
	//
	// Some L7 load balancers/reverse proxies like "ngrok" and "tailscale"
	// require this header to be set to complete the upgrade flow. The header
	// must be set on both the upgrade request here and the 101 Switching
	// Protocols response from the server.
	req.Header.Add(constants.WebAPIConnUpgradeConnectionHeader, constants.WebAPIConnUpgradeConnectionType)

	// Send the request and check if upgrade is successful.
	if err = req.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if http.StatusSwitchingProtocols != resp.StatusCode {
		if http.StatusNotFound == resp.StatusCode {
			return nil, trace.NotImplemented(
				"connection upgrade call to %q with upgrade type %v failed with status code %v. Please upgrade the server and try again.",
				constants.WebAPIConnUpgrade,
				upgradeType,
				resp.StatusCode,
			)
		}
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	if upgradeType == constants.WebAPIConnUpgradeTypeALPNPing {
		return pingconn.New(conn), nil
	}
	return conn, nil
}

const (
	// With req.Conf:
	// [req]
	// distinguished_name = req_distinguished_name
	// x509_extensions = v3_req
	// prompt = no
	// [req_distinguished_name]
	// C = US
	// O = Teleport
	// CN = tls.teleport.cluster.local
	// [v3_req]
	// keyUsage = keyEncipherment, dataEncipherment
	// extendedKeyUsage = serverAuth, clientAuth
	// subjectAltName = @alt_names
	// [alt_names]
	// DNS.1 = tls.teleport.cluster.local
	//
	// Run:
	// openssl req -x509 -nodes -days 3650 -newkey rsa:2048 -keyout cert.pem -out cert.pem -config req.conf -extensions 'v3_req'

	TLSRoutingTestPinnedCertPem = `
-----BEGIN CERTIFICATE-----
MIIDjDCCAnSgAwIBAgIUFmjJXNJltChAvZBONtjwpx2L7A8wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCVVMxETAPBgNVBAoMCFRlbGVwb3J0MSMwIQYDVQQDDBp0
bHMudGVsZXBvcnQuY2x1c3Rlci5sb2NhbDAeFw0yMzEyMDQxNjE1NThaFw0zMzEy
MDExNjE1NThaMEUxCzAJBgNVBAYTAlVTMREwDwYDVQQKDAhUZWxlcG9ydDEjMCEG
A1UEAwwadGxzLnRlbGVwb3J0LmNsdXN0ZXIubG9jYWwwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQC9L76xXPdtUpEf3uLSa6/0FkhpHfl5byqmCoYsVIug
0SBQsAqXBdXA4mbrKIH34fvuva3KFMN9sSxfSo04DZ8YmE9zAbxlUQjOzyRWS+iL
PO8xPHPF1jf6sjWm3okWLk3nahR2YZOtxKxWfRxSXXuz91iTQzW00ikrOBz21kiZ
Bl/4Be7gYSHmSG8l4tlqDrAxouWka6AhGk5sA+bCt6lXa9vVT9Btdk3kA3qE4BCe
7nUQMUAPCJQmqUqMFuOFqUcBMwos9QREctF6icDYdcAp27NgC1l69okEF66eh7iA
NFC1nieS5UQXqWOdA3/+7ZAu5JndrpoWKmiHzvQyjH53AgMBAAGjdDByMAsGA1Ud
DwQEAwIEMDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwJQYDVR0RBB4w
HIIadGxzLnRlbGVwb3J0LmNsdXN0ZXIubG9jYWwwHQYDVR0OBBYEFC3YIHTp+BBU
MJpf/bCARlSMPqXoMA0GCSqGSIb3DQEBCwUAA4IBAQA4RueHe414+sHq0D4a9Ru4
M1szeEg+ITNVvsO3d0Lh98PTutTQ3/ujd3xjPsL8xPBvWskqjx4a0GobhBs4Wcp8
yKaHde4h1UTxlH4Yh6fpH5iucclh/GBinQllQ/hnD/gmSW3iZrpkqGzRpZa1gxhr
R83yHL/MnA6iZjj2cq1c7JQlu5RTaNaYYiMsVrbAbH2/zJV7ddMi5yOKw/noApVP
oPnrtSo6NW+wmgBtOOEQvEeFuhwkd95fkp4H3Ps79HBh4RJTdnLIeFUqgyRndP9V
MOFyKNw39KequK4AU7bBq9hF2mJ/XRc5V3kNRKDiSkImWzjBygcqauHQy1Igmoui
-----END CERTIFICATE-----
`
	TLSRoutingTestPinnedKeyPem = `
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC9L76xXPdtUpEf
3uLSa6/0FkhpHfl5byqmCoYsVIug0SBQsAqXBdXA4mbrKIH34fvuva3KFMN9sSxf
So04DZ8YmE9zAbxlUQjOzyRWS+iLPO8xPHPF1jf6sjWm3okWLk3nahR2YZOtxKxW
fRxSXXuz91iTQzW00ikrOBz21kiZBl/4Be7gYSHmSG8l4tlqDrAxouWka6AhGk5s
A+bCt6lXa9vVT9Btdk3kA3qE4BCe7nUQMUAPCJQmqUqMFuOFqUcBMwos9QREctF6
icDYdcAp27NgC1l69okEF66eh7iANFC1nieS5UQXqWOdA3/+7ZAu5JndrpoWKmiH
zvQyjH53AgMBAAECggEADlXd10a6IPiOsqGLAnLShGZj2kNBMihwTOCjRhyp7+eo
0TRluQfiKJl/PvZ00rm3A2IwFw33ukCAoj/d749orM5txsMs6Wh4iGM916Qs3NAj
N9Hi2+zdlQuH8TsPnDSqBo0NO+Ms84/hlzQnvz4CL6LgfVgsa6U5JWM9Hp8iJSYr
J1pg+K8l1mZL/T/jb6MJpujGa8vGlogWt1zFPKhkJ4srz16a7yqShNX9LwFtGaAE
iKA+TE8XczCzdtOqguzTrnbfEDU/37F/eSG3VlHKML/ozyhYEb236W/d8TeGEvqR
zdwaObXs8mjiFsuAf7SES+UAbfM8Heuaoqy7/fOW0QKBgQDg5yPdVlEzcQ1yXT8K
pDG4yZJ/iSGDBkytNZmG52L87zVFIu4dT1/YqJzo0IwOdDGbKtCcyYrxb5x8ltwH
QRda4HoB6jcPEkoUUQzDL7Y3WDxJRX304xaRar9xomBizaP+r+Q2QPMkSCNW63y7
1b5dkSyttLVK4khwhd0W0SU9fQKBgQDXWFsyEH/5uCLXfArFNX9IwYimNL7N6D+y
xJ5Odk+UY0yVH1aMkcrmrHnzfv5TdaJy98rjKmgZk3jsgAx/wXWkMyY9nIblyQdG
JqSpHUwjVu1hspEk+bqPM34BVjA87Wa69Qdd/49sMPaB1ThyIh8CwYfcViZCDJ2V
iByeCoK+AwKBgQCWHbHyqwrIK02uaE8L60zE6sa+GeokarADbSNsyEVqTsBfxVDq
f3CaTPFu9MSHYUc7KvjTrjLvtG/fOVLkBK5yGiNV49+cT7jilrbOEaquhla3EYth
SbJmnbnrP1bWnCw6c20ASZoBPaVY/xXiymimS6Bm0ZewxBlWAgPwlukkgQKBgGy5
xqmbXRH3H1hO3508anyQgm7wWJnbtjWLQiZ5Y6qXDDaKcQdeIOSglp4TM1NuJEwJ
wh057v9izv4RlL34Lm5uCNO4sP9ZpVuM7TwZd7SsEgRuxQu3LrNYmzkPjCFm96RT
TJnwCzjj68IXpn0xrxiUIAVmVcCpX/L8mv5MbkCDAoGBAMz1FrVEcqZhOeTE1R19
2YrVcsGoTU8c9N7b38GkC+bHEwZwgkgCr3X8pGd49iCWmEGMQ05d2/uutWC49zZk
+0Fus910xvgtZko94Mrz+ZH4ZSk7dlT47uqYEejLEMwlP0DtmtGz6rvO4p2OGI9t
M092xRnAe9OeWwLHnff76Nh1
-----END PRIVATE KEY-----
`
)
