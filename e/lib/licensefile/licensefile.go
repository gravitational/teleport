package licensefile

import (
	"encoding/json"
	"os"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws"
	"github.com/gravitational/teleport/lib/services"
)

// LicenseFile describes license file
type LicenseFile struct {
	// KeyPair is the license key pair
	KeyPair *liblicense.License
	// License is the instance of the license
	License types.License
}

// GetKeyPair returns the license KeyPair as the github.com/graviational/license
// value.
func (l *LicenseFile) GetKeyPair() *liblicense.License {
	return l.KeyPair
}

// NewLicenseFile reads a license from filePath.
func NewLicenseFile(filePath string) (*LicenseFile, error) {
	if filePath == "" {
		return nil, trace.BadParameter("missing license file path")
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, trace.Wrap(err, "unable to read license file")
	}

	licenseFile, err := FromPEM(bytes)
	if err != nil {
		return nil, trace.Wrap(err, "unable to parse and validate license PEM")
	}

	if licenseFile.License.GetAWSProductID() != "" || licenseFile.License.GetAWSAccountID() != "" {
		if err := aws.Verify(*licenseFile.KeyPair, licenseFile.License); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return licenseFile, nil
}

// FromPEM parses PEM and creates an instance of LicenseFile
func FromPEM(pem []byte) (*LicenseFile, error) {
	var license types.License
	licenseKeyPair, err := liblicense.ParseLicensePEM(pem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check if it's a legacy license
	var legacyLicense LegacyLicense
	if err := json.Unmarshal(licenseKeyPair.RawPayload, &legacyLicense); err == nil {
		license, err = legacyLicense.ToV3()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		license, err = services.UnmarshalLicense(licenseKeyPair.RawPayload)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		license.SetExpiry(licenseKeyPair.Cert.NotAfter)
		license.SetAnonymizationKey(string(licenseKeyPair.AnonymizationKey))
	}

	return &LicenseFile{licenseKeyPair, license}, nil
}
