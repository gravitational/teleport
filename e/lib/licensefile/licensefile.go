package licensefile

import (
	"encoding/json"
	"os"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws"
	"github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/services"

	liblicense "github.com/gravitational/license"

	"github.com/gravitational/trace"
)

// LicenseFile describes license file
type LicenseFile struct {
	// KeyPair is the license key pair
	KeyPair *liblicense.License
	// License is the instance of the license
	License types.License
}

func (l *LicenseFile) IsExpired() bool {
	return time.Now().After(l.KeyPair.Cert.NotAfter)
}

func (l *LicenseFile) ExpiresIn() time.Duration {
	return l.KeyPair.Cert.NotAfter.Sub(time.Now())
}

// ReadAndActivate reads and activates a license from the file
func ReadAndActivate(filePath string) (*LicenseFile, error) {
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

	modules.SetModules(licenseFile.License)

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
	}

	return &LicenseFile{licenseKeyPair, license}, nil
}
