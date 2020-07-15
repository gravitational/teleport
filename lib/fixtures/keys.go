package fixtures

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strings"

	"golang.org/x/net/http2"

	"github.com/gravitational/trace"
)

var PEMBytes = map[string][]byte{
	"ecdsa": []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEINGWx0zo6fhJ/0EAfrPzVFyFC9s18lBt3cRoEDhS3ARooAoGCCqGSM49
AwEHoUQDQgAEi9Hdw6KvZcWxfg2IDhA7UkpDtzzt6ZqJXSsFdLd+Kx4S3Sx4cVO+
6/ZOXRnPmNAlLUqjShUsUBBngG0u2fqEqA==
-----END EC PRIVATE KEY-----`),
	"rsa": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAz1MArBKUGR4pHEwGS8PC6buJcjY7IHd5E8N7bDezVlmkZhz3
2bMLCkKpoHGrcgL5UmiyTjcMZkfp/mVVkqGGQo+7ufiSbrUMgWhXpy0JL+ec2THY
9Q2LTF4VXE5Q1/3mc0yTxwm1DQsOMc5eysFDlOoztkkrTo1SFqxMIP/IB+UVs9pD
r3VUYCu+U5UFH0/5y7puR6BTc/kf6p1OR3cFN9hnyt0JAKewiHBpY8XVkBxTNU4z
WPyS2NPo4ir76XXVR0Y6oXnAewpngUVLbKOOQOy79au7+zQs/OQ11LhaiXoxDdSP
eFBeYeTUjej9YaBFKidV72W3SGOzcizu47+EUwIDAQABAoIBAHeDPojy8MKF+2bf
gGWehLaeL/5RusXdeUNmVbitZ0koxbdDjbDGIGAay5O80vsXMchKqDakTxaK8B2B
JtIvIKkwGCR9YVRGM95JWvX45SnjVyxxKsMguqMcPS4Hy1yndXgTtcBwHRlWvSkC
8Ovqet3WIFc9WKSgnKiLTBtdt16sq0OO0aF3yfb2tf5jT4KHKd18KFSvKO1oG7Ka
D57uj1wpB0CnFqPSCLx1FECG0PN8hPKipZInuzQv08bwIspTuBENTZESUs24KCye
y23seugGv//7gfv1QlXOuzBJLa4JPj6wg87z1u7b+OJit1xE8VU1LSh5a73G6xDU
NC/65EkCgYEA9E7THlhklAT5gRDfW99nCKCGWPMgfpQtbnC+c14Ef1owETcYENUU
zlcn8ZSAbgCFSJX4yRXdlvyuBzImw7N9ni94awysQxhZCF6brHF1yp2KhnznGd9+
PUP8ouictcbVCbkVFsH5c6xWWe4ojcdLDHCLlp/gIGF8C1q13H+aiVcCgYEA2T8R
GVEsjSnQKP39VZBkyDxeFy5aPVHK1PxO59yCoMov0CAAal09NuRvUzNC0c2+0K14
vrx9CSfPtwvUGLK3iIEhqawglnpJvIHCvYDZA8kaQipdCcLreT00I4i+zWqYVMCx
+8FJGdAev0PZHeUZmZxhA9rS90yxe0Z2n98NM2UCgYBGTHA/aRv3476PvvUmkJAr
UVWXPs543dZ80wBaXhFZO/Bc48ePAGFuRnH998dE3+16R31BD4OlsKu68llpMrrQ
y8QQuaLP46+q0t5krnlAhjiYHlS5gy/mHSwTDHAbdk1S8Oj6lXJcMJjgY8FTmqcj
uzbPbs2lQ6fX9JAkFKu5HQKBgQDMavaI7wPP1I9lcxFEyPi8HWmfwGLzHhqQbNVG
gQx9haKV4PbjHtbx5uMF089FIacyLnjWaP/ydH6US9IIZ2ohTPjC8g876NenRCZd
MHeDg2Bs7/XZsIrn6vo7kXmQSoQKA8O2E7rYSigUayBKa/+5thbnjKlEP+slBzmp
1zVRrQKBgHmGNSOpSuQiHRn9YuzZ/h5dX8jCLf+wHJzymCC1wVur8IxJjhhSuOIp
7JPquig/B6L2pNoxPa41VDGawQjJY5m4l3ap/oJj61HBB+Auf29BWXqg7V7B7XMB
NFJgTFxC2o3mVBkQ/s6FeDl62hpMheCuO6jRYbZjsM2tUeAKORws
-----END RSA PRIVATE KEY-----`),
}

// LocalhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var LocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9SjY1bIw4
iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZBl2+XsDul
rKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQABo2gwZjAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zAuBgNVHREEJzAlggtleGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAA
AAAAATANBgkqhkiG9w0BAQsFAAOBgQCEcetwO59EWk7WiJsG4x8SY+UIAA+flUI9
tyC4lNhbcF2Idq9greZwbYCqTTTr2XiRNSMLCOjKyI7ukPoPjo16ocHj+P3vZGfs
h1fIw3cSS2OolhloGw/XM6RWPWtPAlGykKLciQrBru5NAPvCMsb/I1DAceTiotQM
fblo6RBxUQ==
-----END CERTIFICATE-----`)

// LocalhostKey is the private key for localhostCert.
var LocalhostKey = []byte(testingKey(`-----BEGIN RSA TESTING KEY-----
MIICXgIBAAKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9
SjY1bIw4iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZB
l2+XsDulrKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQAB
AoGAGRzwwir7XvBOAy5tM/uV6e+Zf6anZzus1s1Y1ClbjbE6HXbnWWF/wbZGOpet
3Zm4vD6MXc7jpTLryzTQIvVdfQbRc6+MUVeLKwZatTXtdZrhu+Jk7hx0nTPy8Jcb
uJqFk541aEw+mMogY/xEcfbWd6IOkp+4xqjlFLBEDytgbIECQQDvH/E6nk+hgN4H
qzzVtxxr397vWrjrIgPbJpQvBsafG7b0dA4AFjwVbFLmQcj2PprIMmPcQrooz8vp
jy4SHEg1AkEA/v13/5M47K9vCxmb8QeD/asydfsgS5TeuNi8DoUBEmiSJwma7FXY
fFUtxuvL7XvjwjN5B30pNEbc6Iuyt7y4MQJBAIt21su4b3sjXNueLKH85Q+phy2U
fQtuUE9txblTu14q3N7gHRZB4ZMhFYyDy8CKrN2cPg/Fvyt0Xlp/DoCzjA0CQQDU
y2ptGsuSmgUtWj3NM9xuwYPm+Z/F84K6+ARYiZ6PYj013sovGKUFfYAqVXVlxtIX
qyUBnu3X9ps8ZfjLZO7BAkEAlT4R5Yl6cGhaJQYZHOde3JEMhNRcVFMO8dJDaFeo
f9Oeos0UUothgiDktdQHxdNEwLjQf7lJJBzV+5OtwswCWA==
-----END RSA TESTING KEY-----`))

func testingKey(s string) string { return strings.ReplaceAll(s, "TESTING KEY", "PRIVATE KEY") }

// TLSConfig is TLS configuration for running local TLS tests
type TLSConfig struct {
	// CertPool is a trusted certificate authority pool
	// that consists of self signed cert
	CertPool *x509.CertPool
	// Certificate is a client x509 client cert
	Certificate *x509.Certificate
	// TLS is a TLS server configuration
	TLS *tls.Config
}

// NewClient creates a HTTP client
func (t *TLSConfig) NewClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: t.CertPool,
			},
		},
	}
}

// LocalTLSConfig returns local TLS config with self signed certificate
func LocalTLSConfig() (*TLSConfig, error) {
	cert, err := tls.X509KeyPair(LocalhostCert, LocalhostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := &tls.Config{
		NextProtos:   []string{http2.NextProtoTLS, "http/1.1"},
		Certificates: []tls.Certificate{cert},
	}

	certificate, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)

	return &TLSConfig{
		CertPool:    certPool,
		TLS:         cfg,
		Certificate: certificate,
	}, nil
}
