// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package configure

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func Test_processEntityDescriptorFlag(t *testing.T) {

	edOk := `<?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor entityID="http://www.okta.com/exk14fxcpjuKMcor30h8" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`

	dir := t.TempDir()

	edBadFile := filepath.Join(dir, "entity_descriptor_bad.xml")
	require.NoError(t, os.WriteFile(edBadFile, []byte("foo bar baz"), 0777))

	edGoodFile := filepath.Join(dir, "entity_descriptor_good.xml")
	require.NoError(t, os.WriteFile(edGoodFile, []byte(edOk), 0777))

	tests := []struct {
		name             string
		entityDescriptor string

		wantErr bool
		want    types.SAMLConnectorSpecV2
	}{
		{
			name:             "URL",
			entityDescriptor: "https://my-idp.com/entity_descriptor.xml",
			wantErr:          false,
			want:             types.SAMLConnectorSpecV2{EntityDescriptorURL: "https://my-idp.com/entity_descriptor.xml"},
		},
		{
			name:             "bad file",
			entityDescriptor: edBadFile,
			wantErr:          true,
		},
		{
			name:             "good file",
			entityDescriptor: edGoodFile,
			wantErr:          false,
			want:             types.SAMLConnectorSpecV2{EntityDescriptor: edOk},
		},
		{
			name:             "verbatim XML",
			entityDescriptor: edOk,
			wantErr:          false,
			want:             types.SAMLConnectorSpecV2{EntityDescriptor: edOk},
		},
		{
			name:             "error",
			entityDescriptor: "does_not_exist.xml",
			wantErr:          true,
		},
		{
			name:             "empty",
			entityDescriptor: "",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := types.SAMLConnectorSpecV2{}
			err := processEntityDescriptorFlag(context.Background(), &spec, tt.entityDescriptor, slog.New(logutils.DiscardHandler{}))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, spec)
			}
		})
	}
}

func Test_validateEntityDescriptor(t *testing.T) {
	edMalformed := `    <md:EntityDescriptor entityID="http://www.okta.com/exk14fxcpjuKMcor30h8" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate>
							nonBase64Cert
							</ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/>
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`

	edOk := `<?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor entityID="http://www.okta.com/exk14fxcpjuKMcor30h8" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`

	edNoCerts := `<?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor entityID="http://www.okta.com/exk14fxcpjuKMcor30h8" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`

	certFromFlag := `-----BEGIN CERTIFICATE-----
MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr
-----END CERTIFICATE-----`

	tests := []struct {
		name                string
		entityDescriptorXML string
		certificate         string
		wantErr             bool
	}{
		{
			name:                "empty ed fails",
			entityDescriptorXML: "",
			certificate:         "",
			wantErr:             true,
		},
		{
			name:                "random data ed fails",
			entityDescriptorXML: `lkzjlkasdjamsdn,mnczkladjiwd,zdnqoiwlkdnaldskalkjalskdjalksdj,.mzxc.,mqwe`,
			certificate:         "",
			wantErr:             true,
		},
		{
			name:                "malformed ed fails",
			entityDescriptorXML: edMalformed,
			wantErr:             true,
		},
		{
			name:                "malformed ed with good cert still fails",
			entityDescriptorXML: edMalformed,
			certificate:         certFromFlag,
			wantErr:             true,
		},
		{
			name:                "good ed works",
			entityDescriptorXML: edOk,
			certificate:         "",
			wantErr:             false,
		},
		{
			name:                "ed without certs fails",
			entityDescriptorXML: edNoCerts,
			certificate:         "",
			wantErr:             true,
		},
		{
			name:                "ed without certs with cert from flag works",
			entityDescriptorXML: edNoCerts,
			certificate:         certFromFlag,
			wantErr:             false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntityDescriptor([]byte(tt.entityDescriptorXML), tt.certificate)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_validateCert(t *testing.T) {
	cert := `-----BEGIN CERTIFICATE-----
MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr
-----END CERTIFICATE-----`

	tests := []struct {
		name       string
		certString string
		wantErr    bool
	}{
		{
			name:       "empty cert fails",
			certString: "",
			wantErr:    true,
		},
		{
			name:       "random string fails",
			certString: "akjldsjlkadsjalkdsjalkdsjqiowkadsdjlkasdjiqwdalksdj,mzncxkladjiqowkajshd",
			wantErr:    true,
		},
		{
			name:       "good cert works",
			certString: cert,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCert(tt.certString)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
