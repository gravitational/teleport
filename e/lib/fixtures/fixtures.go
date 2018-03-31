// package fixtures provides common fixtures for tests
package fixtures

import (
	"github.com/gravitational/license"
	check "gopkg.in/check.v1"
)

// TestLicense returns license used in tests
func TestLicense(c *check.C) *license.License {
	parsed, err := license.ParseString(testLicense)
	c.Assert(err, check.IsNil)
	return parsed
}

const testLicense = `-----BEGIN CERTIFICATE-----
MIIDpDCCAoygAwIBAgIUUD6WS/1zgUMRMkt5rfruU0yNhlEwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAxMHbGljZW5zZTAeFw0xNzExMTYyMjAwNDRaFw0xNzExMTcw
MDAwNDRaMC0xGTAXBgNVBAoTEGdyYXZpdGF0aW9uYWwuaW8xEDAOBgNVBAMTB2xp
Y2Vuc2UwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDBgKWX6Z6L2VNS
X0zNUviiAVDJ6EL6uCAg/oRoQw2cGVQsThyo3W5JkRLcagXGKTK6wsZlhqPdxssJ
mXo6wXyfNjWus7C+9y1IzlJDrNWl7sZbKCPpNvydjK1bjRfS7mSB8BemmR2/aZCb
2XETLVkfhbuQuUPgbxDKeq02qg3SufjTsfdAalkENKABFSECrtzDKa44s0R4DxVo
fseyliqkoPbd8c5va7nGuIdgHIzbMLCLJCDB1VuHtWgC1haCusMd78WZyoWzRKuR
FaOGwUElRUTqasAlbcduf1zxOtdVVrY7wrayZuvJhUp31tFirjwzKrD+iyRNJ6MF
HvnWvouhAgMBAAGjgdYwgdMwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsG
AQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBT80HVtMBzG
EkUUL++AfT9dFboBfjAfBgNVHSMEGDAWgBSH9wt6UcxJ3ERINERoCQ49TIWX1TAP
BgNVHREECDAGhwR/AAABMEMGAlUqBD17ImV4cGlyYXRpb24iOiIyMDE3LTExLTE3
VDAwOjAwOjQ0LjMyNTAyNzc2MloiLCJtYXhfbm9kZXMiOjN9MA0GCSqGSIb3DQEB
CwUAA4IBAQAVzyPqi874otuqa7KxRO/r+zcXz0yBSWjk1G5X/jOpN3BZyMpXv3Bp
c9BpGrU4VNIREnc4nqv8VWMSyqz6M4Wt0bwOyU9Tj7cv6YWOtf0VvaAvjNHSLg6m
mDzhzRINweQIZ9k3UW0ABI79wJX+BxPh4uJ3dMjOPvU73nwnIIg5zXJ043ceenKx
KcAXhCxTGvDLWzjGceppsTjYaZSCwhWYzIejVyTmeY9yh2ibOoge9WPf3ap+ApPr
dojHNASmlhXGmDdE2KhAmIndwea5VyH2dJYKwpSk1ZMqEFQGgTXv/ZF3Bix6N5KZ
dAuOkPE3OcKnWSwb2f2clkeDoXB7c8D8
-----END CERTIFICATE-----
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAwYCll+mei9lTUl9MzVL4ogFQyehC+rggIP6EaEMNnBlULE4c
qN1uSZES3GoFxikyusLGZYaj3cbLCZl6OsF8nzY1rrOwvvctSM5SQ6zVpe7GWygj
6Tb8nYytW40X0u5kgfAXppkdv2mQm9lxEy1ZH4W7kLlD4G8QynqtNqoN0rn407H3
QGpZBDSgARUhAq7cwymuOLNEeA8VaH7HspYqpKD23fHOb2u5xriHYByM2zCwiyQg
wdVbh7VoAtYWgrrDHe/FmcqFs0SrkRWjhsFBJUVE6mrAJW3Hbn9c8TrXVVa2O8K2
smbryYVKd9bRYq48Myqw/oskTSejBR751r6LoQIDAQABAoIBAHytmoTmV2y+i+xQ
UVkes+sWs+pUiAup5bG8rK3NPpCs1Upyzg6UFkK6gg+ZFL1YwEILy++QsDbuptY5
mMMQ9m5TbIVzbFevRfNaVTEbxNFUp2QG2hSjhGMzSGPr5kTXq9T5URPcJom1yCJT
GYOEvZ8M+QzIAo8yoPwFzWOicKOsZXK0qYdin7y096hZgqchdAtvbeWrEbizNUUY
zbRTPDNCe9zwYTHrqSn6JrIdFzN7BAeF6BgavdxdtU3PF0EaAZSJP3ILQ3WGfx2A
IR1iNwGNom/uZcjsQ0OAStODE+vYzy+Gy59G2QQec79fYwP3YWRCWneFaCzCLHnq
KlevE/0CgYEAxdGG9tisLJzG+g8M3dMN2eueinFZAkrd4U/FN7fegsE3Yy1Z2GZF
kAzTTChxSPUyWzjKlORIPHrXclfAFmJunDnPrHVYOCGSkGJFOE/w4vThDbUN/ApZ
8c7EMt2C4l8SATj4kCbw0Qsn2poA6XyvQ/2TGBsP9jfKvDTVmFi6F0MCgYEA+moo
KbVlaXzLVFMPpYJnwYzBBV/1Es/Un6lCgkDOVqFI7gShTwAtb/yAfLVvr/oI4oy8
42ncso/J6b2DciducXTLkbw7/5a9btz5sDnnX2WZD/HHaBmJje47Dmi6JhxkDlCs
oak4G5xaB9qRLFqM9/ss/YU0VfeqsyxGUuOAKUsCgYBOl/6dWFyfpPJRK2WbRF0+
daSZsIsCpCgfeogKqRzYqleNKdmGZqvAnbfdjDvmFrUZFSk3mrMwhEXRAhgpTJZR
r45ZII4aTwxiHQkPZIN6SHyZ65NQzfQKZHIDG4sC0W7f2Xi4HSCUjXAaJBG0snsX
8klczHO9CVGdEQjD7IyS7QKBgA2o8s3rpj+N3i2YZlcZ+Pz256Saamz/R1L6UbV3
QYo6PBc3y3Dayp+8P2oOH6yS0B9DnB4vrSlUbKhCfUQh4IVx4JTvlrpHh8ffaANz
9SogCaxz/POxyO4kG7aageUIUXDyd6hN6dCfw81/38FyoxP38KlXtdYmr3ocpS1q
WZhnAoGBAK5Pi2GQhrO4WMCneiTtTbAtz4sQrIkLn25k8i0ogbUWAHrKUp0aLYSA
BFKYBTHXhb/Fs1uEOwEsxVMU6SxfnwYZS9XG08PURasf/ooJx2/4GPCYhhEtrZm8
5utxTzihcDnNNHmN7l8SYxnlU9M6epjoyzdwgL2TvGCnAbvGjoTW
-----END RSA PRIVATE KEY-----`

const (
	// AWSDoc is AWS instance metadata document
	AWSDoc = `{
  "devpayProductCodes" : null,
  "marketplaceProductCodes" : null,
  "version" : "2017-09-30",
  "instanceId" : "i-095aa02b39cb14231",
  "billingProducts" : null,
  "instanceType" : "t2.micro",
  "availabilityZone" : "us-west-2b",
  "kernelId" : null,
  "ramdiskId" : null,
  "accountId" : "126027368216",
  "architecture" : "x86_64",
  "imageId" : "ami-c250c8ba",
  "pendingTime" : "2018-03-27T23:54:03Z",
  "privateIp" : "10.0.1.213",
  "region" : "us-west-2"
}`
	// AWSSig is AWS instance metadata signature
	AWSSig = `NitpTNs510R4nTAvUIS+tUejufqLhoBF9U+JAHljvxY36xJxCF2ydn09bolF9Ou9UDfctD7utqg2
cu+NIXKHje6yhppDqvTNCOsWQ0XqG7MigGucsEMl8CuV6/7liTL+W30lXKOALCIMuj44TlbXBdss
ZGsf+dc/sUQeaNiG4zs=`
)
