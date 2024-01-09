package licensefile

import (
	"encoding/json"
	"os"
	"time"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib/services"
)

// LicenseFile describes license file
type LicenseFile struct {
	// KeyPair is the license key pair
	KeyPair *liblicense.License
	// License is the instance of the license
	License types.License
}

// IsExpired returns true if the license expiry time is in the past, and false
// if it is not.
func (l *LicenseFile) IsExpired() bool {
	return time.Now().After(l.KeyPair.Cert.NotAfter)
}

// ExpiresIn returns how long until the license expires. The result will be
// negative if the license has expired.
func (l *LicenseFile) ExpiresIn() time.Duration {
	return time.Until(l.KeyPair.Cert.NotAfter)
}

// IsDisabled returns true if the license has expired by more than the grace
// interval, and false if it has not.
func (l *LicenseFile) IsDisabled() bool {
	disabledAt := l.KeyPair.Cert.NotAfter.Add(constants.LicenseGraceInterval)
	return time.Now().After(disabledAt)
}

// DisabledIn returns how long until features should be disabled due to license
// expiry. The result will be negative if that time has already passed.
func (l *LicenseFile) DisabledIn() time.Duration {
	disabledAt := l.KeyPair.Cert.NotAfter.Add(constants.LicenseGraceInterval)
	return time.Until(disabledAt)
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
		return nil, trace.Wrap(err, "unable to read license file: %v", filePath)
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
