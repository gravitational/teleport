package samlsp

import (
	"io/ioutil"
	"net/http"
	"strings"

	. "gopkg.in/check.v1"
)

var _ = Suite(&ParseTest{})

type ParseTest struct {
}

func (test *ParseTest) SetUpTest(c *C) {

}

type mockTransport func(req *http.Request) (*http.Response, error)

func (mt mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return mt(req)
}

func (test *ParseTest) TestCanParseTestshibMetadata(c *C) {
	http.DefaultTransport = mockTransport(func(req *http.Request) (*http.Response, error) {
		responseBody := `<EntitiesDescriptor Name="urn:mace:shibboleth:testshib:two"
    xmlns="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#"
    xmlns:mdalg="urn:oasis:names:tc:SAML:metadata:algsupport" xmlns:mdui="urn:oasis:names:tc:SAML:metadata:ui"
    xmlns:shibmd="urn:mace:shibboleth:metadata:1.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">

    <!-- This file contains the metadata for the testing IdP and SP
     that are operated by TestShib as a service for testing new
     Shibboleth and SAML providers. -->

    <EntityDescriptor entityID="https://idp.testshib.org/idp/shibboleth">

        <Extensions>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha512" />
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#sha384" />
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256" />
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1" />
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha512" />
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha384" />
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" />
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1" />
        </Extensions>

        <IDPSSODescriptor
            protocolSupportEnumeration="urn:oasis:names:tc:SAML:1.1:protocol urn:mace:shibboleth:1.0 urn:oasis:names:tc:SAML:2.0:protocol">
            <Extensions>
                <shibmd:Scope regexp="false">testshib.org</shibmd:Scope>
                <mdui:UIInfo>
                    <mdui:DisplayName xml:lang="en">TestShib Test IdP</mdui:DisplayName>
                    <mdui:Description xml:lang="en">TestShib IdP. Use this as a source of attributes
                        for your test SP.</mdui:Description>
                    <mdui:Logo height="88" width="253"
                        >https://www.testshib.org/testshibtwo.jpg</mdui:Logo>
                </mdui:UIInfo>

            </Extensions>
            <!-- old signing key
            <KeyDescriptor>
                <ds:KeyInfo>
                    <ds:X509Data>
                        <ds:X509Certificate>
                            MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV
                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD
                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4
                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI
                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl
                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B
                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C
                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe
                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT
                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614
                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH
                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G
                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86
                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl
                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo
                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN
                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL
                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo
                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4
                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj
                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr
                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==
                        </ds:X509Certificate>
                    </ds:X509Data>
                </ds:KeyInfo>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc" />
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#tripledes-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-1_5"/>
            </KeyDescriptor>
            -->

            <!-- new signing key -->
            <KeyDescriptor>
                <ds:KeyInfo>
                    <ds:X509Data>
                        <ds:X509Certificate>
                            MIIDAzCCAeugAwIBAgIVAPX0G6LuoXnKS0Muei006mVSBXbvMA0GCSqGSIb3DQEB
                            CwUAMBsxGTAXBgNVBAMMEGlkcC50ZXN0c2hpYi5vcmcwHhcNMTYwODIzMjEyMDU0
                            WhcNMzYwODIzMjEyMDU0WjAbMRkwFwYDVQQDDBBpZHAudGVzdHNoaWIub3JnMIIB
                            IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAg9C4J2DiRTEhJAWzPt1S3ryh
                            m3M2P3hPpwJwvt2q948vdTUxhhvNMuc3M3S4WNh6JYBs53R+YmjqJAII4ShMGNEm
                            lGnSVfHorex7IxikpuDPKV3SNf28mCAZbQrX+hWA+ann/uifVzqXktOjs6DdzdBn
                            xoVhniXgC8WCJwKcx6JO/hHsH1rG/0DSDeZFpTTcZHj4S9MlLNUtt5JxRzV/MmmB
                            3ObaX0CMqsSWUOQeE4nylSlp5RWHCnx70cs9kwz5WrflnbnzCeHU2sdbNotBEeTH
                            ot6a2cj/pXlRJIgPsrL/4VSicPZcGYMJMPoLTJ8mdy6mpR6nbCmP7dVbCIm/DQID
                            AQABoz4wPDAdBgNVHQ4EFgQUUfaDa2mPi24x09yWp1OFXmZ2GPswGwYDVR0RBBQw
                            EoIQaWRwLnRlc3RzaGliLm9yZzANBgkqhkiG9w0BAQsFAAOCAQEASKKgqTxhqBzR
                            OZ1eVy++si+eTTUQZU4+8UywSKLia2RattaAPMAcXUjO+3cYOQXLVASdlJtt+8QP
                            dRkfp8SiJemHPXC8BES83pogJPYEGJsKo19l4XFJHPnPy+Dsn3mlJyOfAa8RyWBS
                            80u5lrvAcr2TJXt9fXgkYs7BOCigxtZoR8flceGRlAZ4p5FPPxQR6NDYb645jtOT
                            MVr3zgfjP6Wh2dt+2p04LG7ENJn8/gEwtXVuXCsPoSCDx9Y0QmyXTJNdV1aB0AhO
                            RkWPlFYwp+zOyOIR+3m1+pqWFpn0eT/HrxpdKa74FA3R2kq4R7dXe4G0kUgXTdqX
                            MLRKhDgdmA==
                        </ds:X509Certificate>
                    </ds:X509Data>
                </ds:KeyInfo>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc" />
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#tripledes-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-1_5"/>
            </KeyDescriptor>

            <ArtifactResolutionService Binding="urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding"
                Location="https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/ArtifactResolution"
                index="1"/>
            <ArtifactResolutionService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP"
                Location="https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/ArtifactResolution"
                index="2"/>

            <NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>
            <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>

            <SingleSignOnService Binding="urn:mace:shibboleth:1.0:profiles:AuthnRequest"
                Location="https://idp.testshib.org/idp/profile/Shibboleth/SSO"/>
            <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
                Location="https://idp.testshib.org/idp/profile/SAML2/POST/SSO"/>
            <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
                Location="https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO"/>
            <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP"
                Location="https://idp.testshib.org/idp/profile/SAML2/SOAP/ECP"/>

        </IDPSSODescriptor>

        <AttributeAuthorityDescriptor
            protocolSupportEnumeration="urn:oasis:names:tc:SAML:1.1:protocol urn:oasis:names:tc:SAML:2.0:protocol">

            <!-- old SSL/TLS
            <KeyDescriptor>
                <ds:KeyInfo>
                    <ds:X509Data>
                        <ds:X509Certificate>
                            MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV
                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD
                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4
                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI
                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl
                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B
                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C
                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe
                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT
                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614
                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH
                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G
                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86
                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl
                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo
                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN
                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL
                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo
                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4
                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj
                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr
                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==
                        </ds:X509Certificate>
                    </ds:X509Data>
                </ds:KeyInfo>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc" />
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#tripledes-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-1_5"/>
            </KeyDescriptor>
            -->

            <!-- new SSL/TLS -->
            <KeyDescriptor>
                <ds:KeyInfo>
                    <ds:X509Data>
                        <ds:X509Certificate>
                            MIIDAzCCAeugAwIBAgIVAPX0G6LuoXnKS0Muei006mVSBXbvMA0GCSqGSIb3DQEB
                            CwUAMBsxGTAXBgNVBAMMEGlkcC50ZXN0c2hpYi5vcmcwHhcNMTYwODIzMjEyMDU0
                            WhcNMzYwODIzMjEyMDU0WjAbMRkwFwYDVQQDDBBpZHAudGVzdHNoaWIub3JnMIIB
                            IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAg9C4J2DiRTEhJAWzPt1S3ryh
                            m3M2P3hPpwJwvt2q948vdTUxhhvNMuc3M3S4WNh6JYBs53R+YmjqJAII4ShMGNEm
                            lGnSVfHorex7IxikpuDPKV3SNf28mCAZbQrX+hWA+ann/uifVzqXktOjs6DdzdBn
                            xoVhniXgC8WCJwKcx6JO/hHsH1rG/0DSDeZFpTTcZHj4S9MlLNUtt5JxRzV/MmmB
                            3ObaX0CMqsSWUOQeE4nylSlp5RWHCnx70cs9kwz5WrflnbnzCeHU2sdbNotBEeTH
                            ot6a2cj/pXlRJIgPsrL/4VSicPZcGYMJMPoLTJ8mdy6mpR6nbCmP7dVbCIm/DQID
                            AQABoz4wPDAdBgNVHQ4EFgQUUfaDa2mPi24x09yWp1OFXmZ2GPswGwYDVR0RBBQw
                            EoIQaWRwLnRlc3RzaGliLm9yZzANBgkqhkiG9w0BAQsFAAOCAQEASKKgqTxhqBzR
                            OZ1eVy++si+eTTUQZU4+8UywSKLia2RattaAPMAcXUjO+3cYOQXLVASdlJtt+8QP
                            dRkfp8SiJemHPXC8BES83pogJPYEGJsKo19l4XFJHPnPy+Dsn3mlJyOfAa8RyWBS
                            80u5lrvAcr2TJXt9fXgkYs7BOCigxtZoR8flceGRlAZ4p5FPPxQR6NDYb645jtOT
                            MVr3zgfjP6Wh2dt+2p04LG7ENJn8/gEwtXVuXCsPoSCDx9Y0QmyXTJNdV1aB0AhO
                            RkWPlFYwp+zOyOIR+3m1+pqWFpn0eT/HrxpdKa74FA3R2kq4R7dXe4G0kUgXTdqX
                            MLRKhDgdmA==
                        </ds:X509Certificate>
                    </ds:X509Data>
                </ds:KeyInfo>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc" />
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#tripledes-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-1_5"/>
            </KeyDescriptor>

            <AttributeService Binding="urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding"
                Location="https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/AttributeQuery"/>
            <AttributeService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP"
                Location="https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/AttributeQuery"/>

            <NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>
            <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>

        </AttributeAuthorityDescriptor>

        <Organization>
            <OrganizationName xml:lang="en">TestShib Two Identity Provider</OrganizationName>
            <OrganizationDisplayName xml:lang="en">TestShib Two</OrganizationDisplayName>
            <OrganizationURL xml:lang="en">http://www.testshib.org/testshib-two/</OrganizationURL>
        </Organization>
        <ContactPerson contactType="technical">
            <GivenName>Nate</GivenName>
            <SurName>Klingenstein</SurName>
            <EmailAddress>ndk@internet2.edu</EmailAddress>
        </ContactPerson>
    </EntityDescriptor>

    <!-- = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = -->
    <!--             Metadata for SP.TESTSHIB.ORG                    -->
    <!-- = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = -->

    <EntityDescriptor entityID="https://sp.testshib.org/shibboleth-sp">

        <Extensions>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha512"/>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#sha384"/>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#sha224"/>
            <mdalg:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha512"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha384"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha256"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha224"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha512"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha384"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2009/xmldsig11#dsa-sha256"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha1"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/>
            <mdalg:SigningMethod Algorithm="http://www.w3.org/2000/09/xmldsig#dsa-sha1"/>
        </Extensions>


        <!-- An SP supporting SAML 1 and 2 contains this element with protocol support as shown. -->
        <SPSSODescriptor
            protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol urn:oasis:names:tc:SAML:1.1:protocol http://schemas.xmlsoap.org/ws/2003/07/secext">

            <Extensions>
                <!-- A request initiator at /Testshib that you can use to customize authentication requests issued to your IdP by TestShib. -->
                <init:RequestInitiator xmlns:init="urn:oasis:names:tc:SAML:profiles:SSO:request-init" Binding="urn:oasis:names:tc:SAML:profiles:SSO:request-init" Location="https://sp.testshib.org/Shibboleth.sso/TestShib"/>

                <mdui:UIInfo>
                    <mdui:DisplayName xml:lang="en">TestShib Test SP</mdui:DisplayName>
                    <mdui:Description xml:lang="en">TestShib SP. Log into this to test your machine.
                        Once logged in check that all attributes that you expected have been
                        released.</mdui:Description>
                    <mdui:Logo height="88" width="253">https://www.testshib.org/testshibtwo.jpg</mdui:Logo>
                </mdui:UIInfo>
            </Extensions>

            <KeyDescriptor>
                <ds:KeyInfo>
                    <ds:X509Data>
                        <ds:X509Certificate>
                            MIIEPjCCAyagAwIBAgIBADANBgkqhkiG9w0BAQUFADB3MQswCQYDVQQGEwJVUzEV
                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMSIwIAYD
                            VQQKExlUZXN0U2hpYiBTZXJ2aWNlIFByb3ZpZGVyMRgwFgYDVQQDEw9zcC50ZXN0
                            c2hpYi5vcmcwHhcNMDYwODMwMjEyNDM5WhcNMTYwODI3MjEyNDM5WjB3MQswCQYD
                            VQQGEwJVUzEVMBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1
                            cmdoMSIwIAYDVQQKExlUZXN0U2hpYiBTZXJ2aWNlIFByb3ZpZGVyMRgwFgYDVQQD
                            Ew9zcC50ZXN0c2hpYi5vcmcwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB
                            AQDJyR6ZP6MXkQ9z6RRziT0AuCabDd3x1m7nLO9ZRPbr0v1LsU+nnC363jO8nGEq
                            sqkgiZ/bSsO5lvjEt4ehff57ERio2Qk9cYw8XCgmYccVXKH9M+QVO1MQwErNobWb
                            AjiVkuhWcwLWQwTDBowfKXI87SA7KR7sFUymNx5z1aoRvk3GM++tiPY6u4shy8c7
                            vpWbVfisfTfvef/y+galxjPUQYHmegu7vCbjYP3On0V7/Ivzr+r2aPhp8egxt00Q
                            XpilNai12LBYV3Nv/lMsUzBeB7+CdXRVjZOHGuQ8mGqEbsj8MBXvcxIKbcpeK5Zi
                            JCVXPfarzuriM1G5y5QkKW+LAgMBAAGjgdQwgdEwHQYDVR0OBBYEFKB6wPDxwYrY
                            StNjU5P4b4AjBVQVMIGhBgNVHSMEgZkwgZaAFKB6wPDxwYrYStNjU5P4b4AjBVQV
                            oXukeTB3MQswCQYDVQQGEwJVUzEVMBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYD
                            VQQHEwpQaXR0c2J1cmdoMSIwIAYDVQQKExlUZXN0U2hpYiBTZXJ2aWNlIFByb3Zp
                            ZGVyMRgwFgYDVQQDEw9zcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN
                            BgkqhkiG9w0BAQUFAAOCAQEAc06Kgt7ZP6g2TIZgMbFxg6vKwvDL0+2dzF11Onpl
                            5sbtkPaNIcj24lQ4vajCrrGKdzHXo9m54BzrdRJ7xDYtw0dbu37l1IZVmiZr12eE
                            Iay/5YMU+aWP1z70h867ZQ7/7Y4HW345rdiS6EW663oH732wSYNt9kr7/0Uer3KD
                            9CuPuOidBacospDaFyfsaJruE99Kd6Eu/w5KLAGG+m0iqENCziDGzVA47TngKz2v
                            PVA+aokoOyoz3b53qeti77ijatSEoKjxheBWpO+eoJeGq/e49Um3M2ogIX/JAlMa
                            Inh+vYSYngQB2sx9LGkR9KHaMKNIGCDehk93Xla4pWJx1w==
                        </ds:X509Certificate>
                    </ds:X509Data>
                </ds:KeyInfo>
                <EncryptionMethod Algorithm="http://www.w3.org/2009/xmlenc11#aes128-gcm"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2009/xmlenc11#aes192-gcm"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2009/xmlenc11#aes256-gcm"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#tripledes-cbc"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2009/xmlenc11#rsa-oaep"/>
                <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"/>
            </KeyDescriptor>

            <!-- This tells IdPs that Single Logout is supported and where/how to request it. -->

            <SingleLogoutService Location="https://sp.testshib.org/Shibboleth.sso/SLO/SOAP"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP"/>
            <SingleLogoutService Location="https://sp.testshib.org/Shibboleth.sso/SLO/Redirect"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"/>
            <SingleLogoutService Location="https://sp.testshib.org/Shibboleth.sso/SLO/POST"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"/>
            <SingleLogoutService Location="https://sp.testshib.org/Shibboleth.sso/SLO/Artifact"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact"/>


            <!-- This tells IdPs that you only need transient identifiers. -->
            <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>
            <NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>

            <!--
		This tells IdPs where and how to send authentication assertions. Mostly
		the SP will tell the IdP what location to use in its request, but this
		is how the IdP validates the location and also figures out which
		SAML version/binding to use.
		-->

            <AssertionConsumerService index="1" isDefault="true"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
                Location="https://sp.testshib.org/Shibboleth.sso/SAML2/POST"/>
            <AssertionConsumerService index="2"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST-SimpleSign"
                Location="https://sp.testshib.org/Shibboleth.sso/SAML2/POST-SimpleSign"/>
            <AssertionConsumerService index="3"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact"
                Location="https://sp.testshib.org/Shibboleth.sso/SAML2/Artifact"/>
            <AssertionConsumerService index="4"
                Binding="urn:oasis:names:tc:SAML:1.0:profiles:browser-post"
                Location="https://sp.testshib.org/Shibboleth.sso/SAML/POST"/>
            <AssertionConsumerService index="5"
                Binding="urn:oasis:names:tc:SAML:1.0:profiles:artifact-01"
                Location="https://sp.testshib.org/Shibboleth.sso/SAML/Artifact"/>
            <AssertionConsumerService index="6"
                Binding="http://schemas.xmlsoap.org/ws/2003/07/secext"
                Location="https://sp.testshib.org/Shibboleth.sso/ADFS"/>

            <!-- A couple additional assertion consumers for the registration webapp. -->

            <AssertionConsumerService index="7"
                Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
                Location="https://www.testshib.org/Shibboleth.sso/SAML2/POST"/>
            <AssertionConsumerService index="8"
                Binding="urn:oasis:names:tc:SAML:1.0:profiles:browser-post"
                Location="https://www.testshib.org/Shibboleth.sso/SAML/POST"/>

        </SPSSODescriptor>

        <!-- This is just information about the entity in human terms. -->
        <Organization>
            <OrganizationName xml:lang="en">TestShib Two Service Provider</OrganizationName>
            <OrganizationDisplayName xml:lang="en">TestShib Two</OrganizationDisplayName>
            <OrganizationURL xml:lang="en">http://www.testshib.org/testshib-two/</OrganizationURL>
        </Organization>
        <ContactPerson contactType="technical">
            <GivenName>Nate</GivenName>
            <SurName>Klingenstein</SurName>
            <EmailAddress>ndk@internet2.edu</EmailAddress>
        </ContactPerson>

    </EntityDescriptor>


</EntitiesDescriptor>`
		return &http.Response{
			Header:     http.Header{},
			Request:    req,
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})

	_, err := New(Options{IDPMetadataURL: "https://idp.testshib.org/idp/shibboleth"})
	c.Assert(err, IsNil)
}

func (test *ParseTest) TestCanParseGoogleMetadata(c *C) {
	http.DefaultTransport = mockTransport(func(req *http.Request) (*http.Response, error) {
		responseBody := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://accounts.google.com/o/saml2?idpid=123456789" validUntil="2021-01-03T16:17:49.000Z">
  <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIIDRTCCAi2gAwIBAgIJAJmEH6sae9eeMA0GCSqGSIb3DQEBBQUAMCAxHjAcBgNV
BAMTFW15c2VydmljZS5leGFtcGxlLmNvbTAeFw0xNjAxMDUxNjIxMDhaFw0xNzAx
MDQxNjIxMDhaMCAxHjAcBgNVBAMTFW15c2VydmljZS5leGFtcGxlLmNvbTCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMONpY1MGEw+GF3FSjVLuyZcS6i/
CYi4VNIDKkX+w//yd2SKvEWI6cvEdwDhrFXeIyISq59uTPhsyc6Ig4Lr1XtbjhjW
LSpTO7+nVCCjxF5mYm22550N4BUaO0cA4uLyU1lAlPIWdgvSowwpoeSy0PLqtqfN
RJL/MHrNXZAXNjpPyUfonfq3jcZVLrvxg9pbEjMa5NanlSweNVtPSF806gZCID5f
BBafN/m5xL18Vx3zfk7qv9G9jrkK24aaQ0lsjLY+kIXdRkfz6Qc/nWMri0kJPIev
Slwj/rQQ/oF1ljyYXvGaSlIPIRGilXqCc6+yMN7/YMNN5T9b51ENMLyk16kCAwEA
AaOBgTB/MB0GA1UdDgQWBBSk7n+OaCqaDMkeIq0jFuNatPtuBDBQBgNVHSMESTBH
gBSk7n+OaCqaDMkeIq0jFuNatPtuBKEkpCIwIDEeMBwGA1UEAxMVbXlzZXJ2aWNl
LmV4YW1wbGUuY29tggkAmYQfqxp7154wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0B
AQUFAAOCAQEAMpvF7AwBv29vmgRYf1riXM+jidmANHADjgoR49JLijto1fo32RL8
47UewSZW+0ojocBYWqTK0EPl7vOxVBGEmtn/OQyGhs2H1urdtokDp7LzoMJoj2jR
cOqlJq7b1mkTI4y7btNRmFwWFFx7/ftkYk1qxbfiqXfMtN02aznFhZTLdcfyCPia
ZIfLCYSxUx2xzv5jp16T2JxyQusAJo82yIZOICYgZRzoKodTSsP8cOOfRhnUlUE+
ZV50RFLmIOGWbFvl8ktKyqlDC7eWT0+7C+YV3qRdOuDn+SlXFGhw8wgGV2HtVT7D
MUsphIQXHXtrx4Z3qRgE/uZ8z98LA35XfA==</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://accounts.google.com/o/saml2/idp?idpid=123456789"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://accounts.google.com/o/saml2/idp?idpid=123456789"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`
		return &http.Response{
			Header:     http.Header{},
			Request:    req,
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})

	_, err := New(Options{IDPMetadataURL: "https://accounts.google.com/o/saml2?idpid=123456789"})
	c.Assert(err, IsNil)
}

func (test *ParseTest) TestCanParseFreeIPAMetadata(c *C) {
	http.DefaultTransport = mockTransport(func(req *http.Request) (*http.Response, error) {
		responseBody := `<?xml version='1.0' encoding='UTF-8'?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" validUntil="2021-10-11T12:03:21.537380" entityID="https://ipa.example.com/idp/saml2/metadata">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo>
        <ds:X509Data>
          <ds:X509Certificate>MIIDRTCCAi2gAwIBAgIJAJmEH6sae9eeMA0GCSqGSIb3DQEBBQUAMCAxHjAcBgNV
BAMTFW15c2VydmljZS5leGFtcGxlLmNvbTAeFw0xNjAxMDUxNjIxMDhaFw0xNzAx
MDQxNjIxMDhaMCAxHjAcBgNVBAMTFW15c2VydmljZS5leGFtcGxlLmNvbTCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMONpY1MGEw+GF3FSjVLuyZcS6i/
CYi4VNIDKkX+w//yd2SKvEWI6cvEdwDhrFXeIyISq59uTPhsyc6Ig4Lr1XtbjhjW
LSpTO7+nVCCjxF5mYm22550N4BUaO0cA4uLyU1lAlPIWdgvSowwpoeSy0PLqtqfN
RJL/MHrNXZAXNjpPyUfonfq3jcZVLrvxg9pbEjMa5NanlSweNVtPSF806gZCID5f
BBafN/m5xL18Vx3zfk7qv9G9jrkK24aaQ0lsjLY+kIXdRkfz6Qc/nWMri0kJPIev
Slwj/rQQ/oF1ljyYXvGaSlIPIRGilXqCc6+yMN7/YMNN5T9b51ENMLyk16kCAwEA
AaOBgTB/MB0GA1UdDgQWBBSk7n+OaCqaDMkeIq0jFuNatPtuBDBQBgNVHSMESTBH
gBSk7n+OaCqaDMkeIq0jFuNatPtuBKEkpCIwIDEeMBwGA1UEAxMVbXlzZXJ2aWNl
LmV4YW1wbGUuY29tggkAmYQfqxp7154wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0B
AQUFAAOCAQEAMpvF7AwBv29vmgRYf1riXM+jidmANHADjgoR49JLijto1fo32RL8
47UewSZW+0ojocBYWqTK0EPl7vOxVBGEmtn/OQyGhs2H1urdtokDp7LzoMJoj2jR
cOqlJq7b1mkTI4y7btNRmFwWFFx7/ftkYk1qxbfiqXfMtN02aznFhZTLdcfyCPia
ZIfLCYSxUx2xzv5jp16T2JxyQusAJo82yIZOICYgZRzoKodTSsP8cOOfRhnUlUE+
ZV50RFLmIOGWbFvl8ktKyqlDC7eWT0+7C+YV3qRdOuDn+SlXFGhw8wgGV2HtVT7D
MUsphIQXHXtrx4Z3qRgE/uZ8z98LA35XfA==
</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:KeyDescriptor use="encryption">
      <ds:KeyInfo>
        <ds:X509Data>
          <ds:X509Certificate>MIIDRTCCAi2gAwIBAgIJAJmEH6sae9eeMA0GCSqGSIb3DQEBBQUAMCAxHjAcBgNV
BAMTFW15c2VydmljZS5leGFtcGxlLmNvbTAeFw0xNjAxMDUxNjIxMDhaFw0xNzAx
MDQxNjIxMDhaMCAxHjAcBgNVBAMTFW15c2VydmljZS5leGFtcGxlLmNvbTCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMONpY1MGEw+GF3FSjVLuyZcS6i/
CYi4VNIDKkX+w//yd2SKvEWI6cvEdwDhrFXeIyISq59uTPhsyc6Ig4Lr1XtbjhjW
LSpTO7+nVCCjxF5mYm22550N4BUaO0cA4uLyU1lAlPIWdgvSowwpoeSy0PLqtqfN
RJL/MHrNXZAXNjpPyUfonfq3jcZVLrvxg9pbEjMa5NanlSweNVtPSF806gZCID5f
BBafN/m5xL18Vx3zfk7qv9G9jrkK24aaQ0lsjLY+kIXdRkfz6Qc/nWMri0kJPIev
Slwj/rQQ/oF1ljyYXvGaSlIPIRGilXqCc6+yMN7/YMNN5T9b51ENMLyk16kCAwEA
AaOBgTB/MB0GA1UdDgQWBBSk7n+OaCqaDMkeIq0jFuNatPtuBDBQBgNVHSMESTBH
gBSk7n+OaCqaDMkeIq0jFuNatPtuBKEkpCIwIDEeMBwGA1UEAxMVbXlzZXJ2aWNl
LmV4YW1wbGUuY29tggkAmYQfqxp7154wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0B
AQUFAAOCAQEAMpvF7AwBv29vmgRYf1riXM+jidmANHADjgoR49JLijto1fo32RL8
47UewSZW+0ojocBYWqTK0EPl7vOxVBGEmtn/OQyGhs2H1urdtokDp7LzoMJoj2jR
cOqlJq7b1mkTI4y7btNRmFwWFFx7/ftkYk1qxbfiqXfMtN02aznFhZTLdcfyCPia
ZIfLCYSxUx2xzv5jp16T2JxyQusAJo82yIZOICYgZRzoKodTSsP8cOOfRhnUlUE+
ZV50RFLmIOGWbFvl8ktKyqlDC7eWT0+7C+YV3qRdOuDn+SlXFGhw8wgGV2HtVT7D
MUsphIQXHXtrx4Z3qRgE/uZ8z98LA35XfA==
</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://ipa.example.com/idp/saml2/SSO/POST"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://ipa.example.com/idp/saml2/SSO/Redirect"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP" Location="https://ipa.example.com/idp/saml2/SSO/SOAP"/>
    <md:SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://ipa.example.com/idp/saml2/SLO/Redirect"/>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:persistent</md:NameIDFormat>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</md:NameIDFormat>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`
		return &http.Response{
			Header:     http.Header{},
			Request:    req,
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})

	_, err := New(Options{IDPMetadataURL: "https://ipa.example.com/idp/saml2/metadata"})
	c.Assert(err, IsNil)
}
