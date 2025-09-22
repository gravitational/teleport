/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package fixtures

import (
	apifixtures "github.com/gravitational/teleport/api/fixtures"
)

const SAMLOktaAuthRequestID = `_4d84cad1-1c61-4e4f-8ab6-1358b8d0da77`
const SAMLOktaAuthnResponseXML = `<?xml version="1.0" encoding="UTF-8"?><saml2p:Response xmlns:saml2p="urn:oasis:names:tc:SAML:2.0:protocol" Destination="https://localhost:3080/v1/webapi/saml/acs" ID="id23076376064475199270314772" InResponseTo="_4d84cad1-1c61-4e4f-8ab6-1358b8d0da77" IssueInstant="2017-05-10T18:52:44.797Z" Version="2.0" xmlns:xs="http://www.w3.org/2001/XMLSchema"><saml2:Issuer xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion" Format="urn:oasis:names:tc:SAML:2.0:nameid-format:entity">http://www.okta.com/exkafftca6RqPVgyZ0h7</saml2:Issuer><ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/><ds:SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/><ds:Reference URI="#id23076376064475199270314772"><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/><ds:Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"><ec:InclusiveNamespaces xmlns:ec="http://www.w3.org/2001/10/xml-exc-c14n#" PrefixList="xs"/></ds:Transform></ds:Transforms><ds:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/><ds:DigestValue>wvBbayjxY78ouyo1DYjGrAOfLYymapbZeylWWnbA+lQ=</ds:DigestValue></ds:Reference></ds:SignedInfo><ds:SignatureValue>QSKPoRIEwjNw/QT3GO3huhdE0seUqSrWvZWIFwkAZv3D04Q5SgsiKHJbP22VsaMUVWojnYbzLfk62nnCSPvdnJmCEH7N3SV+3TkTfeJrCOqi34NLwHadBNTSURnFA+Y+p+HNYE4x4vtg8Vn5KF/teM9hMwAEqKimobYRIS3fPW8jVRcMBdkki5HaM3OCXy9JL1krTkFMGmHobeoaV4taIv7lDpfPw9fRuys1oX0VKfGXmVMpG24n1KB8jLOuC9GYL4HdB9LhIHfznzW3xiKVXm4rJiVIg9PMSQ6SV698yFXEjh5DOdLZPIz5qcizkiL7jujPUSwZQSTArp4m6ft3Pw==</ds:SignatureValue><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAVvvlUB6MA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMTcwNTA5MjMzODQ3WhcNMjcwNTA5MjMzOTQ3WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
ltQB+ZTGKoaNiWQRZ/bzl9oNmbjFyLiVlDASaYnuv1yBx70/Tzr9VXn0gWkl5yH0zIpzREvR5qM1
VAaH3dgNbxTg15f0e5xDk7r5ggS11mX5p8S1Ca9UQmqhRRv7jhMJxHbCy4rFV5jO/uyNQDaMZLPd
zFuzpwKaWhy/UCQ3lDmNzxp3Q6T3FULV+fvs7tJp+8p6qfpoGkANGVfs/Jx/kgbbk0JZG2wk4VVl
b1rZTZJWQ6hCLwTAsD/WixcUx1BFTXmqoZTYNETATVJQ+bEMCVf8K4hxbH6hEgjoL//AE9zgpa1m
uvKwevYBvYZ/+VRy+It3d9mq73AdrG9vchE3qQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQASANAj
8JQdBdKIrrbU6n1egwETwkOUwWyUja/5t+C/RIZPuKP5XmsUhFecbCrML2+M7HG0l5leqyD3u5pS
yhyBz99QWZegoRJy05tciuQUCyPrp6zDzl5De3byq5WQ6Ke+uiRb2GFdDNDhLuMlE48aLWyjm4qh
31Q0/wAWJ1zwmrYxu4p/OhZemU7myuSF5tp35rzV3CPRN31d2UcZAwzMUgwKkCE3yT1o+lLskg/k
C7yZIZM0DuazwkaenExrncvPtF6KL7eccudcknNjhRjFD3Yx1nNXgbVRHvVaElm0YxLiLcl8l0Rn
pHM7WKwFyW1dvEDax3BGj9/cbKvpvcwR</ds:X509Certificate></ds:X509Data></ds:KeyInfo></ds:Signature><saml2p:Status xmlns:saml2p="urn:oasis:names:tc:SAML:2.0:protocol"><saml2p:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></saml2p:Status><saml2:Assertion xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion" ID="id23076376064501965895215823" IssueInstant="2017-05-10T18:52:44.797Z" Version="2.0" xmlns:xs="http://www.w3.org/2001/XMLSchema"><saml2:Issuer Format="urn:oasis:names:tc:SAML:2.0:nameid-format:entity" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion">http://www.okta.com/exkafftca6RqPVgyZ0h7</saml2:Issuer><ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/><ds:SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/><ds:Reference URI="#id23076376064501965895215823"><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/><ds:Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"><ec:InclusiveNamespaces xmlns:ec="http://www.w3.org/2001/10/xml-exc-c14n#" PrefixList="xs"/></ds:Transform></ds:Transforms><ds:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/><ds:DigestValue>Bg7B7U6pHc0PU91Ed768yrae+kU0ZgZoafKTj1oJZ10=</ds:DigestValue></ds:Reference></ds:SignedInfo><ds:SignatureValue>GBN1oQJm1Yk4Icq9vb/6YASjQtYof3yJTSvd8uMc9NgQ6qOfkkw+QMNiqPeoHdaPfUiwKyZQrIynsjMMlRzBe/0zEbrP67wseAxPTIKFgbWPk/X2WUbtU3Jg5ijtUWawmoIoChMqZhxkm/rc/7zgRfNFZWGmucgk/GzxzhJb0n3ZtDiG2ZqI7tAnp3O5Oc8rMorYCun1sV/bm+k9HTwNOUaBSjm/d0BjWDfWCtl4KOlC9XHDg7Ht1i++Vjz5Dqt4/JkGUy8LrmLxep3ifXakRwDgK7qlDBRTKU9Up4vTwUPxWprgLZ0u0ze7h7DNwYCLfGC48X6MlaH+tbhklocsjg==</ds:SignatureValue><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAVvvlUB6MA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMTcwNTA5MjMzODQ3WhcNMjcwNTA5MjMzOTQ3WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
ltQB+ZTGKoaNiWQRZ/bzl9oNmbjFyLiVlDASaYnuv1yBx70/Tzr9VXn0gWkl5yH0zIpzREvR5qM1
VAaH3dgNbxTg15f0e5xDk7r5ggS11mX5p8S1Ca9UQmqhRRv7jhMJxHbCy4rFV5jO/uyNQDaMZLPd
zFuzpwKaWhy/UCQ3lDmNzxp3Q6T3FULV+fvs7tJp+8p6qfpoGkANGVfs/Jx/kgbbk0JZG2wk4VVl
b1rZTZJWQ6hCLwTAsD/WixcUx1BFTXmqoZTYNETATVJQ+bEMCVf8K4hxbH6hEgjoL//AE9zgpa1m
uvKwevYBvYZ/+VRy+It3d9mq73AdrG9vchE3qQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQASANAj
8JQdBdKIrrbU6n1egwETwkOUwWyUja/5t+C/RIZPuKP5XmsUhFecbCrML2+M7HG0l5leqyD3u5pS
yhyBz99QWZegoRJy05tciuQUCyPrp6zDzl5De3byq5WQ6Ke+uiRb2GFdDNDhLuMlE48aLWyjm4qh
31Q0/wAWJ1zwmrYxu4p/OhZemU7myuSF5tp35rzV3CPRN31d2UcZAwzMUgwKkCE3yT1o+lLskg/k
C7yZIZM0DuazwkaenExrncvPtF6KL7eccudcknNjhRjFD3Yx1nNXgbVRHvVaElm0YxLiLcl8l0Rn
pHM7WKwFyW1dvEDax3BGj9/cbKvpvcwR</ds:X509Certificate></ds:X509Data></ds:KeyInfo></ds:Signature><saml2:Subject xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">ops@gravitational.io</saml2:NameID><saml2:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"><saml2:SubjectConfirmationData InResponseTo="_4d84cad1-1c61-4e4f-8ab6-1358b8d0da77" NotOnOrAfter="2017-05-10T18:57:44.797Z" Recipient="https://localhost:3080/v1/webapi/saml/acs"/></saml2:SubjectConfirmation></saml2:Subject><saml2:Conditions NotBefore="2017-05-10T18:47:44.797Z" NotOnOrAfter="2017-05-10T18:57:44.797Z" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:AudienceRestriction><saml2:Audience>https://localhost:3080/v1/webapi/saml/acs</saml2:Audience></saml2:AudienceRestriction></saml2:Conditions><saml2:AuthnStatement AuthnInstant="2017-05-10T18:42:30.615Z" SessionIndex="_4d84cad1-1c61-4e4f-8ab6-1358b8d0da77" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:AuthnContext><saml2:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml2:AuthnContextClassRef></saml2:AuthnContext></saml2:AuthnStatement><saml2:AttributeStatement xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:Attribute Name="groups" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"><saml2:AttributeValue xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:string">Everyone</saml2:AttributeValue></saml2:Attribute></saml2:AttributeStatement></saml2:Assertion></saml2p:Response>`

const SAMLOktaCertPEM = `-----BEGIN CERTIFICATE-----
MIIDpDCCAoygAwIBAgIGAVvvlUB6MA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMTcwNTA5MjMzODQ3WhcNMjcwNTA5MjMzOTQ3WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
ltQB+ZTGKoaNiWQRZ/bzl9oNmbjFyLiVlDASaYnuv1yBx70/Tzr9VXn0gWkl5yH0zIpzREvR5qM1
VAaH3dgNbxTg15f0e5xDk7r5ggS11mX5p8S1Ca9UQmqhRRv7jhMJxHbCy4rFV5jO/uyNQDaMZLPd
zFuzpwKaWhy/UCQ3lDmNzxp3Q6T3FULV+fvs7tJp+8p6qfpoGkANGVfs/Jx/kgbbk0JZG2wk4VVl
b1rZTZJWQ6hCLwTAsD/WixcUx1BFTXmqoZTYNETATVJQ+bEMCVf8K4hxbH6hEgjoL//AE9zgpa1m
uvKwevYBvYZ/+VRy+It3d9mq73AdrG9vchE3qQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQASANAj
8JQdBdKIrrbU6n1egwETwkOUwWyUja/5t+C/RIZPuKP5XmsUhFecbCrML2+M7HG0l5leqyD3u5pS
yhyBz99QWZegoRJy05tciuQUCyPrp6zDzl5De3byq5WQ6Ke+uiRb2GFdDNDhLuMlE48aLWyjm4qh
31Q0/wAWJ1zwmrYxu4p/OhZemU7myuSF5tp35rzV3CPRN31d2UcZAwzMUgwKkCE3yT1o+lLskg/k
C7yZIZM0DuazwkaenExrncvPtF6KL7eccudcknNjhRjFD3Yx1nNXgbVRHvVaElm0YxLiLcl8l0Rn
pHM7WKwFyW1dvEDax3BGj9/cbKvpvcwR
-----END CERTIFICATE-----`

const SAMLOktaSSO = `https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml`

const SAMLOktaConnectorV2 = `kind: saml
version: v2
metadata:
  name: OktaSAML
  namespace: default
spec:
  acs: https://localhost:3080/v1/webapi/saml/acs
  attributes_to_roles:
    - {name: "groups", value: "Everyone", roles: ["admin"]}
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.okta.com/exkafftca6RqPVgyZ0h7"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAVvvlUB6MA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
    A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
    MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
    DWluZm9Ab2t0YS5jb20wHhcNMTcwNTA5MjMzODQ3WhcNMjcwNTA5MjMzOTQ3WjCBkjELMAkGA1UE
    BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
    BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
    KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
    ltQB+ZTGKoaNiWQRZ/bzl9oNmbjFyLiVlDASaYnuv1yBx70/Tzr9VXn0gWkl5yH0zIpzREvR5qM1
    VAaH3dgNbxTg15f0e5xDk7r5ggS11mX5p8S1Ca9UQmqhRRv7jhMJxHbCy4rFV5jO/uyNQDaMZLPd
    zFuzpwKaWhy/UCQ3lDmNzxp3Q6T3FULV+fvs7tJp+8p6qfpoGkANGVfs/Jx/kgbbk0JZG2wk4VVl
    b1rZTZJWQ6hCLwTAsD/WixcUx1BFTXmqoZTYNETATVJQ+bEMCVf8K4hxbH6hEgjoL//AE9zgpa1m
    uvKwevYBvYZ/+VRy+It3d9mq73AdrG9vchE3qQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQASANAj
    8JQdBdKIrrbU6n1egwETwkOUwWyUja/5t+C/RIZPuKP5XmsUhFecbCrML2+M7HG0l5leqyD3u5pS
    yhyBz99QWZegoRJy05tciuQUCyPrp6zDzl5De3byq5WQ6Ke+uiRb2GFdDNDhLuMlE48aLWyjm4qh
    31Q0/wAWJ1zwmrYxu4p/OhZemU7myuSF5tp35rzV3CPRN31d2UcZAwzMUgwKkCE3yT1o+lLskg/k
    C7yZIZM0DuazwkaenExrncvPtF6KL7eccudcknNjhRjFD3Yx1nNXgbVRHvVaElm0YxLiLcl8l0Rn
    pHM7WKwFyW1dvEDax3BGj9/cbKvpvcwR</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`

const SAMLOktaConnectorV2WithPadding = `kind: saml
version: v2
metadata:
  name: OktaSAML
  namespace: default
spec:
  acs: https://localhost:3080/v1/webapi/saml/acs
  attributes_to_roles:
    - {name: "groups", value: "Everyone", roles: ["admin"]}
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.okta.com/exkafftca6RqPVgyZ0h7"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAVvvlUB6MA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
      A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
      MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
      DWluZm9Ab2t0YS5jb20wHhcNMTcwNTA5MjMzODQ3WhcNMjcwNTA5MjMzOTQ3WjCBkjELMAkGA1UE
      BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
      BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
      KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
      ltQB+ZTGKoaNiWQRZ/bzl9oNmbjFyLiVlDASaYnuv1yBx70/Tzr9VXn0gWkl5yH0zIpzREvR5qM1
      VAaH3dgNbxTg15f0e5xDk7r5ggS11mX5p8S1Ca9UQmqhRRv7jhMJxHbCy4rFV5jO/uyNQDaMZLPd
      zFuzpwKaWhy/UCQ3lDmNzxp3Q6T3FULV+fvs7tJp+8p6qfpoGkANGVfs/Jx/kgbbk0JZG2wk4VVl
      b1rZTZJWQ6hCLwTAsD/WixcUx1BFTXmqoZTYNETATVJQ+bEMCVf8K4hxbH6hEgjoL//AE9zgpa1m
      uvKwevYBvYZ/+VRy+It3d9mq73AdrG9vchE3qQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQASANAj
      8JQdBdKIrrbU6n1egwETwkOUwWyUja/5t+C/RIZPuKP5XmsUhFecbCrML2+M7HG0l5leqyD3u5pS
      yhyBz99QWZegoRJy05tciuQUCyPrp6zDzl5De3byq5WQ6Ke+uiRb2GFdDNDhLuMlE48aLWyjm4qh
      31Q0/wAWJ1zwmrYxu4p/OhZemU7myuSF5tp35rzV3CPRN31d2UcZAwzMUgwKkCE3yT1o+lLskg/k
      C7yZIZM0DuazwkaenExrncvPtF6KL7eccudcknNjhRjFD3Yx1nNXgbVRHvVaElm0YxLiLcl8l0Rn
      pHM7WKwFyW1dvEDax3BGj9/cbKvpvcwR
    </ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`

const SAMLConnectorMissingIDPSSODescriptor = `kind: saml
version: v2
metadata:
  name: OktaSAML
  namespace: default
spec:
  acs: https://localhost:3080/v1/webapi/saml/acs
  sso: https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml
  attributes_to_roles:
    - {name: "groups", value: "Everyone", roles: ["admin"]}
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?><EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="0001-01-01T00:00:00Z" entityID="dummy"></EntityDescriptor>`

const (
	TLSCACertPEM = apifixtures.TLSCACertPEM
	TLSCAKeyPEM  = apifixtures.TLSCAKeyPEM
	// Backwards-compatibility alias for teleport.e
	SigningCertPEM = TLSCACertPEM
)

const EncryptionCertPEM = `-----BEGIN CERTIFICATE-----
MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAy
MTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEB
AQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3p
hPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/
tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4H
eyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXj
SdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRg
G1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQt
PoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceM
SdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEG
AXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+u
Td7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72Zko
iADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkN
bwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0j
BBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkq
hkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm
4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1Vh
H+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+
jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsS
pk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnp
PVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96ji
y8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAm
fDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho
1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSw
HaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzl
cpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=
-----END CERTIFICATE-----`

const EncryptionKeyPEM = `-----BEGIN PRIVATE KEY-----
MIIJQwIBADANBgkqhkiG9w0BAQEFAASCCS0wggkpAgEAAoICAQDiEvFfAwgR8rfF
PXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnav
dk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K
8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM
+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcP
m8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1P
YcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5Io
QJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX
7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Ku
lp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0
zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsR
TN81lRll/Wgd4EknznXq060RQBkNbwIDAQABAoICAQCo8+MzUH6teylf3KhoqiGU
ahoQmQEPSNwPUQASPnlSFanAZM439KcMr//m/9SIxAKHtJ0FL9/0Rfjo9JAhTTJe
ZAlqeBFB0YmyOI4uUWMDgc3G+Fwx4WkAbvkfbICR8GOW/uFYCNF+CSrNDdbYTatc
tXaARvt/cfGWnL7Lz7LNgHlDuTKPDPLPE5I2xqPHlQUM7eu4necbxZlFlDjLsCgm
mHHvYoSUqOSQ20myJ96jmDy1c0UtAPJjBldTUBcLWF/UtOm6FemuBJnMbm1qKwTK
rNUAUFrC/bdIQToZMdo3IPhsZvj9odqK62pPsCSocDld5+anTw+BKfwrQTFkS2VY
H0dciQwIDdaEUVjXnDJnfJrO5CBUhyyyD8X64Ap/yUVSJwrQAeHWGoeZUxr2bUes
DtN2jT6X/pIG4dFeC+z1n61uNWPVptxYEKfu6rndUstHndGTlN/eOGr+bsRbhEXP
7vOMM1mHP05a/BVAsL3SOmXmv3xxp9PNfzdCm4lC6SPLUXknlP/TereW38Nfo08I
jxjyk9p54lplLdBTlG9pqiESxbQi/cd3cIE1rAtTPm214CfcO/dHhGBiB3Q91GV/
qnXgKNOCYHNzGvFERNHBUcSGA/O55xEFbEVoUVJqyJ9ygV/s0PM7Fr9Tl1+rgiSD
xcT7A/QJ1bRsMtyehNTj4QKCAQEA+8QcM7CvCSnJxyg62H6lYookA7qxCUlm7JST
jK6NF0DhXpAtWsoIa9koVOIOZLMhbj/DpWqvc2MTtH+UEgiCBpeHW599b3cKjhm7
89gFN7HRIXJ2xGL05t5s630MPWy74NBth/SQvx5rzutmvWtkHlLa+gePtOeEHeCf
mOxYLrYbJsht9pa87EZUNUNosTdq7hZcsJsrRHJN6Wa1EhRYG8FvjbLo4gXFhfGj
Uv4fgkMLMj4kqvudfcaJv62ZXs1jKjYE6bBqHUQLp07c6GCBrk6ZIiaBNXOOWy+k
QMTS+ldMEilvv1TT9I4uqoQA7o9u7DjsG5sSVDqBZ22fJseAqQKCAQEA5eA5f4p5
saeunf/Ki+L4IGGrW/Ih55xOmPSZt6tIVBMEXWy33Z9pEJXtqkIGFlBowo4WWyxN
Vmj+kM4XER1bAOKE6gKMv7iYKp/rzrB7DgcY9jWVwVPEvZRWWp+EtF5DDEJSDfbp
OyYei8EGefYUKmQEyzcCdfph6OmyTvCAF27MYFE5ZpyiNIRZAPdA6RgdaDODFUQ3
/vAAqing3g0Pyuw19hSQCtOlHrT5m5WzTi1yljVMI/dUCtMp7yk6/2zc9uKK2Mk2
v0UCOWTKLPQV3ETKJigNEn0ur8pMaKNpAM6WBwekyCJuQWFb51KxF5Bc+n72WD+/
GaDa0UyZgxo0VwKCAQBPvmAIZ1ApoNjOggmRhRuxSHv7ymhEvsEg8jaB+s+pq902
bIhRF2jvcAr8R9WzQ6G1H/FCNbZ438rgAwDNbXBx0hEHjk7WvWfUdoY3yBZu+513
8J95uLZFYfIx7Jux4PzpSltHEsm+H06abalPGfLOQAQn6bk03ZfVNs6WS1XrBbc3
44gg8MHKPMRzUnSYnSr7Wo3lSmC7/1B6OxPjNBpsQCqrQR3OaXGU6WKH6QHl6oJj
WZeXqLbLndUHp17Kzlc4iX+o3T3fIyxlw+7ok5i/sxmB3ZxTZ9SRQVfPRAhnTrtD
jWhdu+qerWJOlB0PctL5c1YlsEpv71AJiIk+aTZxAoIBAAoXXs7PiGoZH1xGR2D+
tL/PKdOefIiLXxPt4PWkKkeukgl75VJwVg9pVYac4WGHZCHuVOLpvfdmIo6+zVpt
/Hm8d/NB62XbN6rfXF21d6F1BE6CqbFT+RYNdgECcbPtU2otWybLyQ9UrBCch6lA
+T+nJmK5Zn1BYZz07WPzwNvGfGhaCHgNtj0x9ipJsGrLKTdS05VSalbhuFXAAuQc
lK3m0rOb0Xr4MY54iWCgIL/01MvtSQtnJyRWgsfB+poN8GFSLqA3rRSWdfOJDisN
CAykZG9qYLCIGE2VRudtDQYBC6sBVeWHRWnPWVZ9VdLf/oTsn+nd2ojIe/KmNzL/
Kn8CggEBAKJgv3ldOEmt8VliX68lFQzZ7sH/YDyJciAx1keQaCNZvlqsHTUIS/oG
F8aT30/iGe8axYA+T8oqZVwdozpgyT9VZASNpHXPYGwYfndBxnwlC2CCgjgHAyN6
nYKKtrggXPfsofj7alOcU5n+o9dM8kNgjhM9gjIXtZUl6mKhKZiAqyZb3hNEo+cg
ye+1ZN8CMxPpiT3YyhfMyc9ZyoYCb8lFmpDOhhJj3nRIfSnJmsfsDmtbNOGHaqgZ
4pn7vJTccAPw9fvJ/llKsJOKZLLtOuc2nGJiM3OKItnZaOsb9bNg5wtl/0eZt0hH
T+zhBIl1qramlEnT1hYCydxx4ceziIo=
-----END PRIVATE KEY-----`

// UUID is the unique identifier used in tests
const UUID = "11111111-1111-1111-1111-111111111111"

// UserCertificateRolesAndTraits is a SSH user certificate in the standard
// format with roles "admin" and traits "logins": []string{"foo"}.
const UserCertificateStandard = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgeB1flZgJfeV7s+K3bgQA0cSQ4y+YttYjvdAehNV+cIAAAAADAQABAAABAQDH76SMYg80Iyd3+E80R7tr4BwUEcAwXNpNzp8DNfI/pxl7bl85mkCj0JU2CD+ANLWjFtJcbPP7QLkb/IT7HJhjt+bVeHrqgCs8ImK0fu8lbHEu8rAaJTyopQlDkrBLoIMUcoDB4Ox/A4Oo/LfdDr2JJYlZfhav6lIT1u0w5ZKldl4FGlODlnuS3QE8ljVHseiUEfE/Jvb/uMaYyGFdrU3lozX41eUK0AREcriCgkO4Y9GBGB0BqZjJsjYIMoGu6NIRuEDzhC562r33mzCGdVxBBVJnhf5u0faAZrO0thZRUXUEzJ3+nbXR8TMZeAuA8XmCsqgBAEGGtyu13Rwxx1AhAAAAAAAAAAAAAAABAAAAA2ZvbwAAAAcAAAADZm9vAAAAAF1CHPgAAAAAXUIdcAAAAAAAAAB5AAAACnBlcm1pdC1wdHkAAAAAAAAADnRlbGVwb3J0LXJvbGVzAAAAJgAAACJ7InZlcnNpb24iOiJ2MSIsInJvbGVzIjpbImFkbWluIl19AAAAD3RlbGVwb3J0LXRyYWl0cwAAABQAAAAQCg4KBWxvZ2luEgUKA2ZvbwAAAAAAAAEXAAAAB3NzaC1yc2EAAAADAQABAAABAQC018WJxQ6S117eabPMCLpZ5AgdI9tMXe4bYdHReMJ5mqqnoaCynrjFAowa2bXmifIBWso5NA1JVnbXWX2kcSObcLdG1iwjutak9bWrvBukncz8KO/4p+D/8fIXDNtpbqao0tOxriS7Ez2bHzq0DM0H0VgEZK/NCtxGC4FdH1nrwhanI0S7+GUUSZidkN2LaN9Nbn5DjdbvbkeG+L1DvIdKaEOrHV888GhtLkU4oBEe0w7t8WBbK2Ly5GCP3LoAyppHAY6pXca94bEFfRZG6PFY/H7hViXUg4R+08ByNy64+6aX1zHP4LcgMxb09bz22X9v88a8Dg4c504hGZ3lYEvNAAABDwAAAAdzc2gtcnNhAAABAC0T0ZuIyY8jJRSvKAJGyoJDX0oM6ZCM92ebBelZFo8hGU2K9l6DdGUJ8Jsm+asinWSe5eUAxAULsRvNZRbaw8ADAdvuzDAeUXTmwDUWOEFYVskH9bTratTYKACXFh4bLmDxYko3Kc5Iqf7irtvIyoOhNqyY6ODS4kNU8O6moChA7kXvKITkJuYHoHdk/8nDuU42Im72hYMmO1E+LQqmEtX9khE9OGjrdhLPeti4+mUo0vlEpLb271Ex3KNz2Nrnq/4UCC/GRxY7bhW2Qhmb6PLK7qlrdP+3K2flAnEiirCHvLUO4HipaQa/c13IsidXknHLwmdgwU3cG26eNafcVwc=`

// UserCertificateLegacy is a SSH user certificate in the old legacy format.
const UserCertificateLegacy = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgUTR740WMqrewurgb80LEYI4ePvOoDSS1PuG/fVOtd+UAAAADAQABAAABAQC4gxu87n0GfSqgTrcbDByLmWl2cpvJqHSfHtYTenfhfKoXyYqnVT6YMHSov13gT0l3RMVy/qFKJfbRREGjSQ/YW1zG/y0xCnjRmKgPLyHLzyc+yOrTbIwn9rHIJeuF1DlIJLzhUo/MMi4UZPHLtsLBiJ94MVQMj3R2fRC55gm0ZKcvpmk3H57wbbEze4gQLXI2Crl/JxG0h9CSV5YfRbvftbpoTOR9A4tYQUvNobBz8scdAGHpOYilBIkANwzwOmNw/CLikD6TtirWzwCq/GiO0oec5oeb2Anrq7uGHYzZzx+E+iNe+ebY/xiuN8anLYa9JGQ4YlLSC47xOM+8LviTAAAAAAAAAAAAAAABAAAAA2ZvbwAAAAcAAAADZm9vAAAAAF1CHqcAAAAAXUIfHwAAAAAAAAASAAAACnBlcm1pdC1wdHkAAAAAAAAAAAAAARcAAAAHc3NoLXJzYQAAAAMBAAEAAAEBAOrxHaRQxJKiZ/XZJMs+p4H8bai3aPVgavGIE9uB+1LGkGNzWy1QWmePLBcFred5aNnpoTjl3ZPRT5iRfnaK0ggl8P/4k1O+ZtUkDTDcbtVyjCH9raVzqOtQ/BEu5sXEokOHNRuySJSMJLUQZV94vGpIuXmRY6z8X4uwI4ui/ZUFLOLCcilWxxg87dKnUm9Tg1TzTDHE5M9QXCrUl/IOoV2491paoiUJRvFSQMCQfTCePrBcKne+f5S2gmV8KIMaL4ucFFMhxcP5rVPd/bNDquM46MW9TrESzl6XETKrOO5psgHWKUgCM44u+DOjvbqUOAWc6IIDQ7KonxToTSTehqcAAAEPAAAAB3NzaC1yc2EAAAEArj9BOUTTbPhv6MahtMa/oe9mnZI/R0c5kTtDplGFbfNSg2JI6UHbE/ijM4yR3X2f4tuj4eL4MJxFYKik+mJ7fhxuxlsYq42CILr/7uDu4YKLwdS5pYZtyIVF2KeYjdMZrdXBo/c1dfNgiAvQCyY4knjxnRNgGG7pRw1R2qDQjXlxqrdFiPLgLKAkY/gcBDio4iIlfe/bqNnlJTHDQ4+E8v1PQw1zHJ65WADd8iMT6+RyJ2B9h6B1b2Mtj17pf8DOT6snjmq/FUgYgN1n6wh8IuMMGOOF5orOWjYeXrqQeN1kNN95eQrSXsGEesDJgMeL2Km3KZOH1+ZTGnvoierzlA==`

const (
	SSHCAPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAvJGHcmQNWUjY2eKasmw171qZR0B5FOnzy/nAGB1JAE+QokFe
Bjo8Gkk3L2TSuVNn0NI5uo5Jwp7GYtbfSbowo11E922Bwp0sFoVzeeUMyLud9EPz
Hl8+VvE8WEa1lC4D4aqravAfTeeePrONIYoBttX5oYXQ7aZkM8N7yS7KWNOZpy9f
n1vkSCpDOK29edLHWVyiDcXzULxEbXhPFl9Ly9shuEbqic2LRggxBnh3fhy53u8X
5qj8bp+21GGsQJaZYZtc9ieNYamo/KQcA0hFfUgTmV74ehY0vZ7yQk+2dW22cFqw
Dv+xNmnNHlfuYhHNCfk8rnztxfbqHfifgCArQQIDAQABAoIBADhq8jNva+8CtJ68
BbzMU3bBjIqc550yQhcNKkQMvwKwy31AQXlrgv/6V+B+Me3w3mbD/zGp0LfB+Wkp
ELVmV5cJGNFOmjw3+jDizKHzvddxCtlCW0MDDAvHMV7YCQvEmLSz84WTQkp0ugvY
fKlEOS8S5hVFjDUOS3yRSD/xF+lrIlYUaR4gXnDAJZx9ttgfZlHOp8ehxk+1bn59
3Fv1fCXcCKmKUlTk1kFasD8P+2M3MKP42Ih5ap9cfLSVPiBS/6JRBxIlZrHM9/2a
w6vEp+qMwwgCmxLPMwZfem6LNHO/huTrWKf4ltVubb5bUXIe22udKp2WK4NWc3Ka
uG8EleECgYEA4A9Mwd0QJs0j1kpuJDNIjfFx6IROv3QAb0QPq0+192ZF8P9AEj8B
TNDQVzb/skM+2NDdvhZ5v4+OJQcUNpEskhX+5ikk8QHGAUY6vT8rO6oiIRMaxLuJ
OEDc2Qms1OmctTmgSVyaxfXIK2/GDdvOizt0Z7Y7abza4bigEm49hyMCgYEA13MI
H429Ua0tnVVmGJ/4OjnKbgtF7i02r50vDVktPruKWNy1bhRkRyaOoCH7Zt9WXF2j
GapZZN1N/clO4vf9gikH0VCo4Tc2JR635dXdfISlt8NLXmR800Ms1UCAKlwIOQjz
dgHcvEbvFwSe1MFgOJVGL82G2rUA/zDVOKdjXEsCgYAZxyjZlQlqrWdWHDIX0B6k
1gZ47d/xfvMd2gLDfuQ8lnOtinBgqQcJQ2z028sHQ11TrJQWbpeLRoTgFbRposIx
/H3bFRi+8alKND5Fz6K1tpk+nOgTglADPNMr1UUhKc9xujOKvTDBXcmt1ao/pe5Z
bnmyBPFI9QVpusgP1scVaQKBgE5mJYaV5VZbVkXyVXyQeZt2fBsfLwtEmKm+4OhS
kwxI4kcDyWGNOhBKD4xl0T3V928VA8zLGEyD22WGY5Zj93PtylJ4r3uEw8cuLm0M
LdSp0EPWZQ6sMmAOCbpwBjNj2fonL7C5bMF2bnpJzCJPW9w7NZcfivr68qnp8yzy
fE2RAoGBALWvlHVH/29KOVmM52sOk49tcyc3czjs/YANvbokiItxOB8VPY6QQQnS
/CBsCZxUuWegYmkUnstHDmY1LYqjxW4goOqizIksaReivPmsTuQ1qd+aqXTfg2pt
uy6c6X17xkP5q2Lq4i90ikyWm3Oc25aUEw48pRyK/6rABRUzpDLB
-----END RSA PRIVATE KEY-----
`
	SSHCAPublicKey = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8kYdyZA1ZSNjZ4pqybDXvWplHQHkU6fPL+cAYHUkAT5CiQV4GOjwaSTcvZNK5U2fQ0jm6jknCnsZi1t9JujCjXUT3bYHCnSwWhXN55QzIu530Q/MeXz5W8TxYRrWULgPhqqtq8B9N554+s40higG21fmhhdDtpmQzw3vJLspY05mnL1+fW+RIKkM4rb150sdZXKINxfNQvERteE8WX0vL2yG4RuqJzYtGCDEGeHd+HLne7xfmqPxun7bUYaxAlplhm1z2J41hqaj8pBwDSEV9SBOZXvh6FjS9nvJCT7Z1bbZwWrAO/7E2ac0eV+5iEc0J+TyufO3F9uod+J+AICtB`
)

const (
	JWTSignerPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAt/ocQMUoUDIHkTlmmAGDpup20ucTMWeawzlRDs1KZP4aAUHg
5rM5VE0CIPLAMeGFb/dCckEW+0X9V3bPLp+lnui+wrjG6T+O1mhFo1hJzr1R23p5
upZk3InSZazGN04x2HUtZ+D9pSYYyxiWMAO2U6pEZtPdGwVybO51E46E3jvWL0po
RsUdUGGRTCZmIHKPS83OiN+1XCAZ4qZFp5k9au9zUJkFd6ACTsQJ5EHReuO4Nts+
RE61w2NsTlkvWRYL3Vd1xclkOEBkMmjEaJh98M2FwzvIWBS2qH1Ow78c7AvT8xXx
OKmnPfCWIx6X7jHuteM37I2ItiiNAcAyuNBOzwIDAQABAoIBAAc8rXPWzZkp/qY1
zdVY6ebc/kOZl2WwH6RiUs/0P2Lto/Q8tS4eCrlINjc5lVng9zDKVzDLYq4LuMWC
BPBek1NG8IoUXq66M1I309VzGaQqSlgJ31P5qooKWd5qB3oRd2B+a4TUkuW2M+95
Th8hZkCwR/SLjP0NH80tLCnSx2M+gjZmweI/oXIh60ylIcGgVUlJ/nRvZdWvHmld
PoD4GzI+PStUFR3yzwF9SImoCGMLGcoJBBOccKBlG6384FEbR5RNHUZJeZvwj476
jy3wj9m16GmGhOVZijPM84xIL2jfeQS8DPkBAXNHIdOiYCjgcsk1b1J5AdawvAz6
FWLadIECgYEA6QR7P/engYAhyCMAOUJmvG6ss2KAdHe3G8Mjd3OormSVh1HkMa05
FsidAMrD0fI2Em2qiuSk88AvgbIBNaTbo4PlfzDSpAYaxoOVucak3oj4/KP5SGa7
TdvRSKBqxp/OfHuHZx0delDGW4VNW1He/x6/JnUV15qFX73lUStHIOECgYEAyh9k
eqEv6RgCE5S/2sTQjJh75LCdY70wx0KyrpqOoXXS7gYhBaMV/lydmWQcR+rW390r
U8h5eT2eXE0+hh5SkbNX/P8kcJ0taUwbUUkzmAe651EES4ZCN5x+zprqUDwQqqlM
1K2F+ULnY3PkUrrZObQddir486uRbXFSZ4Rqda8CgYBm1R13O1nm4p8F7bxZiJ5C
Ji189MlvnK1oSRPL0XTtkWIT1+X2rlV1Yo83HESS0GtgcplCtmi9UWElwWKbQ+fS
H5EWMnui+zaxyLw4whtcQeJvzAVlGEEsuQeBH5o/kaLUeMdmkAjERAVlukxLMrRQ
rkb5N86t2XlmqS0cRxcawQKBgQCJC4gBbdEiZtjhlfYPy2rsKWe3w9izi8/LC3pD
0R/scgs2wIkbXVzIPtvM6YgTazOOTlPWVxOmFRWO2AEQxvaNO+Do9cYrZScpQiUz
lEKbToJ33QLggoPbWQzR4VAGXvOeA3TIr28rdyWU1Tt2rKIk8e8X9EMgVLAiWLfa
4HmemQKBgHEx08lXMDWpQRG7s7K+o05W66mjpogIn8/hsvLuc1UE1V/ZIKZL0jo5
TON8GV/tU3F64uFowISnZOPOyBdm37UmiC+o3Ue5MFpH7wPsCMlpVi1WzPAj5W/o
PEqXyCrHytbPuH88DitxbALCPFIAI12EefmJgO7i8UrqahYyI7Lo
-----END RSA PRIVATE KEY-----`
	JWTSignerPublicKey = `-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAt/ocQMUoUDIHkTlmmAGDpup20ucTMWeawzlRDs1KZP4aAUHg5rM5
VE0CIPLAMeGFb/dCckEW+0X9V3bPLp+lnui+wrjG6T+O1mhFo1hJzr1R23p5upZk
3InSZazGN04x2HUtZ+D9pSYYyxiWMAO2U6pEZtPdGwVybO51E46E3jvWL0poRsUd
UGGRTCZmIHKPS83OiN+1XCAZ4qZFp5k9au9zUJkFd6ACTsQJ5EHReuO4Nts+RE61
w2NsTlkvWRYL3Vd1xclkOEBkMmjEaJh98M2FwzvIWBS2qH1Ow78c7AvT8xXxOKmn
PfCWIx6X7jHuteM37I2ItiiNAcAyuNBOzwIDAQAB
-----END RSA PUBLIC KEY-----`
)
