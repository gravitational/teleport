/*
Copyright 2017 Gravitational, Inc.

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

package constants

import "encoding/asn1"

const (
	// TLSKeyAlgo is default TLS algo used for K8s X509 certs
	TLSKeyAlgo = "rsa"

	// TLSKeySize is default TLS key size used for K8s X509 certs
	TLSKeySize = 2048

	// RSAPrivateKeyPEMBlock is the name of the PEM block where private key is stored
	RSAPrivateKeyPEMBlock = "RSA PRIVATE KEY"

	// CertificatePEMBlock is the name of the PEM block where certificate is stored
	CertificatePEMBlock = "CERTIFICATE"

	// LicenseKeyPair is a name of the license key pair
	LicenseKeyPair = "license"

	// LoopbackIP is IP of the loopback interface
	LoopbackIP = "127.0.0.1"

	// LicenseKeyBits used when generating private key for license certificate
	LicenseKeyBits = 2048

	// LicenseOrg is the default name of license subject organization
	LicenseOrg = "gravitational.io"

	// LicenseTimeFormat represents format of expiration time in license payload
	LicenseTimeFormat = "2006-01-02 15:04:05"
)

// LicenseASNExtensionID is an extension ID used when encoding/decoding
// license payload into certificates
var LicenseASN1ExtensionID = asn1.ObjectIdentifier{2, 5, 42}

// EC2InstanceTypes maps AWS instance types to their number of CPUs,
// used for determining whether license allows a certain instance
// type in some cases
var EC2InstanceTypes = map[string]int{
	"t2.nano":     1,
	"t2.micro":    1,
	"t2.small":    1,
	"t2.medium":   2,
	"t2.large":    2,
	"m3.medium":   1,
	"m3.large":    2,
	"m3.xlarge":   4,
	"m3.2xlarge":  8,
	"m4.large":    2,
	"m4.xlarge":   4,
	"m4.2xlarge":  8,
	"m4.4xlarge":  16,
	"m4.10xlarge": 40,
	"c3.large":    2,
	"c3.xlarge":   4,
	"c3.2xlarge":  8,
	"c3.4xlarge":  16,
	"c3.8xlarge":  32,
	"c4.large":    2,
	"c4.xlarge":   4,
	"c4.2xlarge":  8,
	"c4.4xlarge":  16,
	"c4.8xlarge":  36,
	"x1.32xlarge": 128,
	"g2.2xlarge":  8,
	"g2.8xlarge":  32,
	"r3.large":    2,
	"r3.xlarge":   4,
	"r3.2xlarge":  8,
	"r3.4xlarge":  16,
	"r3.8xlarge":  32,
	"i2.xlarge":   4,
	"i2.2xlarge":  8,
	"i2.4xlarge":  16,
	"i2.8xlarge":  32,
	"d2.xlarge":   4,
	"d2.2xlarge":  8,
	"d2.4xlarge":  16,
	"d2.8xlarge":  36,
}
