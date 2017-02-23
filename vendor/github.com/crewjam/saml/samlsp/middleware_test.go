package samlsp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	. "gopkg.in/check.v1"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/testsaml"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MiddlewareTest struct {
	AuthnRequest string
	SamlResponse string
	Key          string
	Certificate  string
	IDPMetadata  string
	Middleware   Middleware
}

var _ = Suite(&MiddlewareTest{})

type testRandomReader struct {
	Next byte
}

func (tr *testRandomReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = tr.Next
		tr.Next += 2
	}
	return len(p), nil
}

const expectedToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJodHRwczovLzE1NjYxNDQ0Lm5ncm9rLmlvL3NhbWwyL21ldGFkYXRhIiwiZXhwIjoxNDQ4OTQyMjI5LCJpYXQiOjE0NDg5MzQ5ODEsIm5iZiI6MTQ0ODkzNTAyOSwic3ViIjoiXzQxYmQyOTU5NzZkYWRkNzBlMTQ4MGYzMThlNzcyODQxIiwiYXR0ciI6eyJjbiI6WyJNZSBNeXNlbGYgQW5kIEkiXSwiZWR1UGVyc29uQWZmaWxpYXRpb24iOlsiTWVtYmVyIiwiU3RhZmYiXSwiZWR1UGVyc29uRW50aXRsZW1lbnQiOlsidXJuOm1hY2U6ZGlyOmVudGl0bGVtZW50OmNvbW1vbi1saWItdGVybXMiXSwiZWR1UGVyc29uUHJpbmNpcGFsTmFtZSI6WyJteXNlbGZAdGVzdHNoaWIub3JnIl0sImVkdVBlcnNvblNjb3BlZEFmZmlsaWF0aW9uIjpbIk1lbWJlckB0ZXN0c2hpYi5vcmciLCJTdGFmZkB0ZXN0c2hpYi5vcmciXSwiZWR1UGVyc29uVGFyZ2V0ZWRJRCI6WyIiXSwiZ2l2ZW5OYW1lIjpbIk1lIE15c2VsZiJdLCJzbiI6WyJBbmQgSSJdLCJ0ZWxlcGhvbmVOdW1iZXIiOlsiNTU1LTU1NTUiXSwidWlkIjpbIm15c2VsZiJdfX0.owRdm3bcXkYV4ePjcJw3qHILRLDhNHAdG3gddLOBJPw"

func (test *MiddlewareTest) SetUpTest(c *C) {
	saml.TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05.999999999 MST 2006", "Mon Dec 1 01:57:09.123456789 UTC 2015")
		return rv
	}
	jwt.TimeFunc = saml.TimeNow
	saml.RandReader = &testRandomReader{}

	test.AuthnRequest = `https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO?RelayState=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1cmkiOiIvIn0.eoUmy2fQduAz--6N82xIOmufY1ZZeRi5x--B7m1pNIY&SAMLRequest=lJJBj9MwEIX%2FSuR7Yzt10sZKIpWtkCotsGqB%2B5BMW4vELp4JsP8et4DYE5Tr%2BPnN957dbGY%2B%2Bz1%2BmZE4%2Bz6NnloxR28DkCPrYUKy3NvD5s2jLXJlLzFw6MMosg0RRnbBPwRP84TxgPGr6%2FHD%2FrEVZ%2BYLWSl1WVXaGJP7UwyfcxckwTQWEnoS2TbtdB6uHn9uuOGSczqgs%2FuUh3i6DmTaenQjyitGIfc4uIg9y8Phnch221a4YVFjpVflcqgM1sUajiWsYGk01KujKVRfJyHRjDtPDJ5bUShdLrReLNX7QtmysrrMK6Pqem3MeqFKq5TInn6lfeX84PypFSL7iJFuwKkN0TU303hPc%2FC7L5G9DnEC%2Frv8OkmxjjepRc%2BOn0X3r14nZBiAoZE%2FwbrmbfLZbZ%2FC6Prn%2F3zgcQzfHiICYys4zii6%2B4E5gieXsBv5kqBr5Msf1%2F0IAAD%2F%2Fw%3D%3D`
	test.SamlResponse = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><saml2p:Response xmlns:saml2p=\"urn:oasis:names:tc:SAML:2.0:protocol\" Destination=\"https://15661444.ngrok.io/saml2/acs\" ID=\"_e9b3332eeaf348da6786aed16300aca9\" InResponseTo=\"id-9e61753d64e928af5a7a341a97f420c9\" IssueInstant=\"2015-12-01T01:56:21.375Z\" Version=\"2.0\"><saml2:Issuer xmlns:saml2=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://idp.testshib.org/idp/shibboleth</saml2:Issuer><saml2p:Status><saml2p:StatusCode Value=\"urn:oasis:names:tc:SAML:2.0:status:Success\"/></saml2p:Status><saml2:EncryptedAssertion xmlns:saml2=\"urn:oasis:names:tc:SAML:2.0:assertion\"><xenc:EncryptedData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\" Id=\"_dab0b1dbbc0595ab06473034e3bb798c\" Type=\"http://www.w3.org/2001/04/xmlenc#Element\"><xenc:EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"/><ds:KeyInfo xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"><xenc:EncryptedKey Id=\"_dd9264352cef16103cdb21fae97fa951\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><ds:DigestMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#sha1\" xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"/></xenc:EncryptionMethod><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJVUzELMAkGA1UE\nCAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTAwMjAwMDg1MVoX\nDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28x\nEjAQBgNVBAMMCWxvY2FsaG9zdDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308\nkWLhZVT4vOulqx/9ibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTv\nSPmHO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKvRsw0X/gf\nnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgkakpMdAqJfs24maGb90Dv\nTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeTQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+\ncvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvnOwJlNCASPZRH/JmF8tX0hoHuAQ==</ds:X509Certificate></ds:X509Data></ds:KeyInfo><xenc:CipherData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:CipherValue>i/wh2ubXbhTH5W3hwc5VEf4DH1xifeTuxoe64ULopGJ0M0XxBKgDEIfTg59JUMmDYB4L8UStTFfqJk9BRGcMeYWVfckn5gCwLptD9cz26irw+7Ud7MIorA7z68v8rEyzwagKjz8VKvX1afgec0wobVTNN3M1Bn+SOyMhAu+Z4tE=</xenc:CipherValue></xenc:CipherData></xenc:EncryptedKey></ds:KeyInfo><xenc:CipherData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:CipherValue>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</xenc:CipherValue></xenc:CipherData></xenc:EncryptedData></saml2:EncryptedAssertion></saml2p:Response>"
	test.Key = "-----BEGIN RSA PRIVATE KEY-----\nMIICXgIBAAKBgQDU8wdiaFmPfTyRYuFlVPi866WrH/2JubkHzp89bBQopDaLXYxi\n3PTu3O6Q/KaKxMOFBqrInwqpv/omOGZ4ycQ51O9I+Yc7ybVlW94lTo2gpGf+Y/8E\nPsVbnZaFutRctJ4dVIp9aQ2TpLiGT0xX1OzBO/JEgq9GzDRf+B+eqSuglwIDAQAB\nAoGBAMuy1eN6cgFiCOgBsB3gVDdTKpww87Qk5ivjqEt28SmXO13A1KNVPS6oQ8SJ\nCT5Azc6X/BIAoJCURVL+LHdqebogKljhH/3yIel1kH19vr4E2kTM/tYH+qj8afUS\nJEmArUzsmmK8ccuNqBcllqdwCZjxL4CHDUmyRudFcHVX9oyhAkEA/OV1OkjM3CLU\nN3sqELdMmHq5QZCUihBmk3/N5OvGdqAFGBlEeewlepEVxkh7JnaNXAXrKHRVu/f/\nfbCQxH+qrwJBANeQERF97b9Sibp9xgolb749UWNlAdqmEpmlvmS202TdcaaT1msU\n4rRLiQN3X9O9mq4LZMSVethrQAdX1whawpkCQQDk1yGf7xZpMJ8F4U5sN+F4rLyM\nRq8Sy8p2OBTwzCUXXK+fYeXjybsUUMr6VMYTRP2fQr/LKJIX+E5ZxvcIyFmDAkEA\nyfjNVUNVaIbQTzEbRlRvT6MqR+PTCefC072NF9aJWR93JimspGZMR7viY6IM4lrr\nvBkm0F5yXKaYtoiiDMzlOQJADqmEwXl0D72ZG/2KDg8b4QZEmC9i5gidpQwJXUc6\nhU+IVQoLxRq0fBib/36K9tcrrO5Ba4iEvDcNY+D8yGbUtA==\n-----END RSA PRIVATE KEY-----\n"
	test.Certificate = "-----BEGIN CERTIFICATE-----\nMIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJV\nUzELMAkGA1UECAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0\nMB4XDTEzMTAwMjAwMDg1MVoXDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMx\nCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28xEjAQBgNVBAMMCWxvY2FsaG9zdDCB\nnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308kWLhZVT4vOulqx/9\nibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTvSPmH\nO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKv\nRsw0X/gfnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgk\nakpMdAqJfs24maGb90DvTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeT\nQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+cvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvn\nOwJlNCASPZRH/JmF8tX0hoHuAQ==\n-----END CERTIFICATE-----\n"
	test.IDPMetadata = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\" xmlns:mdalg=\"urn:oasis:names:tc:SAML:metadata:algsupport\" xmlns:mdui=\"urn:oasis:names:tc:SAML:metadata:ui\" xmlns:shibmd=\"urn:mace:shibboleth:metadata:1.0\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" Name=\"urn:mace:shibboleth:testshib:two\" entityID=\"https://idp.testshib.org/idp/shibboleth\">\n\t<Extensions>\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha512\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#sha384\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha256\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#sha1\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha512\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha384\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha256\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#rsa-sha1\" />\n\t</Extensions>\n\t<IDPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:1.1:protocol urn:mace:shibboleth:1.0 urn:oasis:names:tc:SAML:2.0:protocol\">\n\t\t<Extensions>\n\t\t\t<shibmd:Scope regexp=\"false\">testshib.org</shibmd:Scope>\n\t\t\t<mdui:UIInfo>\n\t\t\t\t<mdui:DisplayName xml:lang=\"en\">TestShib Test IdP</mdui:DisplayName>\n\t\t\t\t<mdui:Description xml:lang=\"en\">TestShib IdP. Use this as a source of attributes\n                        for your test SP.</mdui:Description>\n\t\t\t\t<mdui:Logo height=\"88\" width=\"253\">https://www.testshib.org/testshibtwo.jpg</mdui:Logo>\n\t\t\t</mdui:UIInfo>\n\t\t</Extensions>\n\t\t<KeyDescriptor>\n\t\t\t<ds:KeyInfo>\n\t\t\t\t<ds:X509Data>\n\t\t\t\t\t<ds:X509Certificate>MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV\n                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD\n                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4\n                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI\n                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl\n                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B\n                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C\n                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe\n                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT\n                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614\n                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH\n                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G\n                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86\n                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl\n                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo\n                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN\n                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL\n                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo\n                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4\n                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj\n                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr\n                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==</ds:X509Certificate>\n\t\t\t\t</ds:X509Data>\n\t\t\t</ds:KeyInfo>\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes256-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes192-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#tripledes-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-1_5\" />\n\t\t</KeyDescriptor>\n\t\t<ArtifactResolutionService Binding=\"urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/ArtifactResolution\" index=\"1\" />\n\t\t<ArtifactResolutionService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/ArtifactResolution\" index=\"2\" />\n\t\t<NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>\n\t\t<NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>\n\t\t<SingleSignOnService Binding=\"urn:mace:shibboleth:1.0:profiles:AuthnRequest\" Location=\"https://idp.testshib.org/idp/profile/Shibboleth/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://idp.testshib.org/idp/profile/SAML2/POST/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org/idp/profile/SAML2/SOAP/ECP\" />\n\t</IDPSSODescriptor>\n\t<AttributeAuthorityDescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:1.1:protocol urn:oasis:names:tc:SAML:2.0:protocol\">\n\t\t<KeyDescriptor>\n\t\t\t<ds:KeyInfo>\n\t\t\t\t<ds:X509Data>\n\t\t\t\t\t<ds:X509Certificate>MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV\n                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD\n                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4\n                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI\n                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl\n                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B\n                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C\n                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe\n                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT\n                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614\n                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH\n                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G\n                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86\n                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl\n                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo\n                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN\n                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL\n                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo\n                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4\n                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj\n                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr\n                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==</ds:X509Certificate>\n\t\t\t\t</ds:X509Data>\n\t\t\t</ds:KeyInfo>\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes256-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes192-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#tripledes-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-1_5\" />\n\t\t</KeyDescriptor>\n\t\t<AttributeService Binding=\"urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/AttributeQuery\" />\n\t\t<AttributeService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/AttributeQuery\" />\n\t\t<NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>\n\t\t<NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>\n\t</AttributeAuthorityDescriptor>\n\t<Organization>\n\t\t<OrganizationName xml:lang=\"en\">TestShib Two Identity Provider</OrganizationName>\n\t\t<OrganizationDisplayName xml:lang=\"en\">TestShib Two</OrganizationDisplayName>\n\t\t<OrganizationURL xml:lang=\"en\">http://www.testshib.org/testshib-two/</OrganizationURL>\n\t</Organization>\n\t<ContactPerson contactType=\"technical\">\n\t\t<GivenName>Nate</GivenName>\n\t\t<SurName>Klingenstein</SurName>\n\t\t<EmailAddress>ndk@internet2.edu</EmailAddress>\n\t</ContactPerson>\n</EntityDescriptor>"

	test.Middleware = Middleware{
		ServiceProvider: saml.ServiceProvider{
			Key:         test.Key,
			Certificate: test.Certificate,
			MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
			AcsURL:      "https://15661444.ngrok.io/saml2/acs",
			IDPMetadata: &saml.Metadata{},
		},
		CookieName:   "ttt",
		CookieMaxAge: time.Hour * 2,
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &test.Middleware.ServiceProvider.IDPMetadata)
	c.Assert(err, IsNil)
}

func (test *MiddlewareTest) TestCanProduceMetadata(c *C) {
	req, _ := http.NewRequest("GET", "/saml2/metadata", nil)

	resp := httptest.NewRecorder()
	test.Middleware.ServeHTTP(resp, req)
	c.Assert(resp.Code, Equals, http.StatusOK)
	c.Assert(resp.Header().Get("Content-type"), Equals, "application/samlmetadata+xml")
	c.Assert(string(resp.Body.Bytes()), DeepEquals, ""+
		"<EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" validUntil=\"2015-12-03T01:57:09.123Z\" entityID=\"https://15661444.ngrok.io/saml2/metadata\">\n"+
		"  <SPSSODescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" AuthnRequestsSigned=\"false\" WantAssertionsSigned=\"true\" protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\">\n"+
		"    <KeyDescriptor use=\"signing\">\n"+
		"      <KeyInfo xmlns=\"http://www.w3.org/2000/09/xmldsig#\">\n"+
		"        <X509Data>\n"+
		"          <X509Certificate>MIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJVUzELMAkGA1UECAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTAwMjAwMDg1MVoXDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28xEjAQBgNVBAMMCWxvY2FsaG9zdDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308kWLhZVT4vOulqx/9ibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTvSPmHO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKvRsw0X/gfnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgkakpMdAqJfs24maGb90DvTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeTQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+cvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvnOwJlNCASPZRH/JmF8tX0hoHuAQ==</X509Certificate>\n"+
		"        </X509Data>\n"+
		"      </KeyInfo>\n"+
		"    </KeyDescriptor>\n"+
		"    <KeyDescriptor use=\"encryption\">\n"+
		"      <KeyInfo xmlns=\"http://www.w3.org/2000/09/xmldsig#\">\n"+
		"        <X509Data>\n"+
		"          <X509Certificate>MIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJVUzELMAkGA1UECAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTAwMjAwMDg1MVoXDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28xEjAQBgNVBAMMCWxvY2FsaG9zdDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308kWLhZVT4vOulqx/9ibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTvSPmHO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKvRsw0X/gfnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgkakpMdAqJfs24maGb90DvTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeTQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+cvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvnOwJlNCASPZRH/JmF8tX0hoHuAQ==</X509Certificate>\n"+
		"        </X509Data>\n"+
		"      </KeyInfo>\n"+
		"      <EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\"></EncryptionMethod>\n"+
		"      <EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes192-cbc\"></EncryptionMethod>\n"+
		"      <EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes256-cbc\"></EncryptionMethod>\n"+
		"      <EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\"></EncryptionMethod>\n"+
		"    </KeyDescriptor>\n"+
		"    <AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://15661444.ngrok.io/saml2/acs\" index=\"1\"></AssertionConsumerService>\n"+
		"  </SPSSODescriptor>\n"+
		"</EntityDescriptor>")
}

func (test *MiddlewareTest) TestFourOhFour(c *C) {
	req, _ := http.NewRequest("GET", "/this/is/not/a/supported/uri", nil)

	resp := httptest.NewRecorder()
	test.Middleware.ServeHTTP(resp, req)
	c.Assert(resp.Code, Equals, http.StatusNotFound)
	respBuf, _ := ioutil.ReadAll(resp.Body)
	c.Assert(string(respBuf), Equals, "404 page not found\n")
}

func (test *MiddlewareTest) TestRequireAccountNoCreds(c *C) {
	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusFound)
	c.Assert(resp.Header().Get("Set-Cookie"), Equals,
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6="+
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImlkLTAwMDIwNDA2MDgwYTBjMGUxMDEyMTQxNjE4MWExYzFlMjAyMjI0MjYiLCJ1cmkiOiIvZnJvYiJ9.7f-xjK5ZzpP_51YL4aPQSQcIBKKCRb_j6CE9pZieJG0"+
			"; Path=/saml2/acs; Max-Age=90")

	redirectURL, err := url.Parse(resp.Header().Get("Location"))
	c.Assert(err, IsNil)
	decodedRequest, err := testsaml.ParseRedirectRequest(redirectURL)
	c.Assert(err, IsNil)
	c.Assert(string(decodedRequest), Equals, "<AuthnRequest xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" IssueInstant=\"2015-12-01T01:57:09.123Z\" AssertionConsumerServiceURL=\"https://15661444.ngrok.io/saml2/acs\" Destination=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" ID=\"id-00020406080a0c0e10121416181a1c1e20222426\" ProtocolBinding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Version=\"2.0\"><Issuer xmlns=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://15661444.ngrok.io/saml2/metadata</Issuer><NameIDPolicy xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" AllowCreate=\"true\">urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDPolicy></AuthnRequest>")
}

func (test *MiddlewareTest) TestRequireAccountNoCredsPostBinding(c *C) {
	test.Middleware.ServiceProvider.IDPMetadata.IDPSSODescriptor.SingleSignOnService = test.Middleware.ServiceProvider.IDPMetadata.IDPSSODescriptor.SingleSignOnService[1:2]
	c.Assert("", Equals, test.Middleware.ServiceProvider.GetSSOBindingLocation(saml.HTTPRedirectBinding))

	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusOK)
	c.Assert(resp.Header().Get("Set-Cookie"), Equals,
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6="+
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImlkLTAwMDIwNDA2MDgwYTBjMGUxMDEyMTQxNjE4MWExYzFlMjAyMjI0MjYiLCJ1cmkiOiIvZnJvYiJ9.7f-xjK5ZzpP_51YL4aPQSQcIBKKCRb_j6CE9pZieJG0"+
			"; Path=/saml2/acs; Max-Age=90")
	c.Assert(string(resp.Body.Bytes()), Equals, ""+
		"<!DOCTYPE html>"+
		"<html>"+
		"<body>"+
		"<form method=\"post\" action=\"https://idp.testshib.org/idp/profile/SAML2/POST/SSO\" id=\"SAMLRequestForm\">"+
		"<input type=\"hidden\" name=\"SAMLRequest\" value=\"PEF1dGhuUmVxdWVzdCB4bWxucz0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBJc3N1ZUluc3RhbnQ9IjIwMTUtMTItMDFUMDE6NTc6MDkuMTIzWiIgQXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlVVJMPSJodHRwczovLzE1NjYxNDQ0Lm5ncm9rLmlvL3NhbWwyL2FjcyIgRGVzdGluYXRpb249Imh0dHBzOi8vaWRwLnRlc3RzaGliLm9yZy9pZHAvcHJvZmlsZS9TQU1MMi9QT1NUL1NTTyIgSUQ9ImlkLTAwMDIwNDA2MDgwYTBjMGUxMDEyMTQxNjE4MWExYzFlMjAyMjI0MjYiIFByb3RvY29sQmluZGluZz0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmJpbmRpbmdzOkhUVFAtUE9TVCIgVmVyc2lvbj0iMi4wIj48SXNzdWVyIHhtbG5zPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXNzZXJ0aW9uIiBGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OmVudGl0eSI&#43;aHR0cHM6Ly8xNTY2MTQ0NC5uZ3Jvay5pby9zYW1sMi9tZXRhZGF0YTwvSXNzdWVyPjxOYW1lSURQb2xpY3kgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpwcm90b2NvbCIgQWxsb3dDcmVhdGU9InRydWUiPnVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OnRyYW5zaWVudDwvTmFtZUlEUG9saWN5PjwvQXV0aG5SZXF1ZXN0Pg==\" />"+
		"<input type=\"hidden\" name=\"RelayState\" value=\"KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6\" />"+
		"<input id=\"SAMLSubmitButton\" type=\"submit\" value=\"Submit\" />"+
		"</form>"+
		"<script>document.getElementByID('SAMLSubmitButton').style.visibility=\"hidden\";</script>"+
		"<script>document.getElementById('SAMLRequestForm').submit();</script>"+
		"</body>"+
		"</html>")

	// check that the CSP script hash is set correctly
	scriptContent := "document.getElementById('SAMLRequestForm').submit();"
	scriptSum := sha256.Sum256([]byte(scriptContent))
	scriptHash := base64.StdEncoding.EncodeToString(scriptSum[:])
	c.Assert(resp.Header().Get("Content-Security-Policy"), Equals,
		"default-src; script-src 'sha256-"+scriptHash+"'; reflected-xss block; referrer no-referrer;")

	c.Assert(resp.Header().Get("Content-type"), Equals, "text/html")
}

func (test *MiddlewareTest) TestRequireAccountCreds(c *C) {
	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Assert(r.Header.Get("X-Saml-Telephonenumber"), Equals, "555-5555")
			c.Assert(r.Header["X-Saml-Edupersonscopedaffiliation"], DeepEquals, []string{"Member@testshib.org", "Staff@testshib.org"})
			c.Assert(r.Header.Get("X-Saml-Sn"), Equals, "And I")
			c.Assert(r.Header.Get("X-Saml-Edupersonentitlement"), Equals, "urn:mace:dir:entitlement:common-lib-terms")
			c.Assert(r.Header.Get("X-Saml-Edupersontargetedid"), Equals, "")
			c.Assert(r.Header.Get("X-Saml-Givenname"), Equals, "Me Myself")
			c.Assert(r.Header.Get("X-Saml-Cn"), Equals, "Me Myself And I")
			c.Assert(r.Header["X-Saml-Edupersonaffiliation"], DeepEquals, []string{"Member", "Staff"})
			c.Assert(r.Header.Get("X-Saml-Uid"), Equals, "myself")
			c.Assert(r.Header.Get("X-Saml-Edupersonprincipalname"), Equals, "myself@testshib.org")
			w.WriteHeader(http.StatusTeapot)
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusTeapot)
}

func (test *MiddlewareTest) TestRequireAccountBadCreds(c *C) {
	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.yejJbiI6Ik1lIE15c2VsZiBBbmQgSSIsImVkdVBlcnNvbkFmZmlsaWF0aW9uIjoiU3RhZmYiLCJlZHVQZXJzb25FbnRpdGxlbWVudCI6InVybjptYWNlOmRpcjplbnRpdGxlbWVudDpjb21tb24tbGliLXRlcm1zIiwiZWR1UGVyc29uUHJpbmNpcGFsTmFtZSI6Im15c2VsZkB0ZXN0c2hpYi5vcmciLCJlZHVQZXJzb25TY29wZWRBZmZpbGlhdGlvbiI6IlN0YWZmQHRlc3RzaGliLm9yZyIsImVkdVBlcnNvblRhcmdldGVkSUQiOiIiLCJleHAiOjE0NDg5Mzg2MjksImdpdmVuTmFtZSI6Ik1lIE15c2VsZiIsInNuIjoiQW5kIEkiLCJ0ZWxlcGhvbmVOdW1iZXIiOiI1NTUtNTU1NSIsInVpZCI6Im15c2VsZiJ9.SqeTkbGG35oFj_9H-d9oVdV-Hb7Vqam6LvZLcmia7FY; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusFound)

	c.Assert(resp.Header().Get("Set-Cookie"), Equals,
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6="+
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImlkLTAwMDIwNDA2MDgwYTBjMGUxMDEyMTQxNjE4MWExYzFlMjAyMjI0MjYiLCJ1cmkiOiIvZnJvYiJ9.7f-xjK5ZzpP_51YL4aPQSQcIBKKCRb_j6CE9pZieJG0"+
			"; Path=/saml2/acs; Max-Age=90")
	redirectURL, err := url.Parse(resp.Header().Get("Location"))
	c.Assert(err, IsNil)
	decodedRequest, err := testsaml.ParseRedirectRequest(redirectURL)
	c.Assert(err, IsNil)
	c.Assert(string(decodedRequest), Equals, "<AuthnRequest xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" IssueInstant=\"2015-12-01T01:57:09.123Z\" AssertionConsumerServiceURL=\"https://15661444.ngrok.io/saml2/acs\" Destination=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" ID=\"id-00020406080a0c0e10121416181a1c1e20222426\" ProtocolBinding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Version=\"2.0\"><Issuer xmlns=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://15661444.ngrok.io/saml2/metadata</Issuer><NameIDPolicy xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" AllowCreate=\"true\">urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDPolicy></AuthnRequest>")

}

func (test *MiddlewareTest) TestRequireAccountExpiredCreds(c *C) {
	jwt.TimeFunc = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 UTC 2006", "Mon Dec 1 01:31:21 UTC 2115")
		return rv
	}

	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusFound)
	c.Assert(resp.Header().Get("Set-Cookie"), Equals,
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6="+
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImlkLTAwMDIwNDA2MDgwYTBjMGUxMDEyMTQxNjE4MWExYzFlMjAyMjI0MjYiLCJ1cmkiOiIvZnJvYiJ9.7f-xjK5ZzpP_51YL4aPQSQcIBKKCRb_j6CE9pZieJG0"+
			"; Path=/saml2/acs; Max-Age=90")

	redirectURL, err := url.Parse(resp.Header().Get("Location"))
	c.Assert(err, IsNil)
	decodedRequest, err := testsaml.ParseRedirectRequest(redirectURL)
	c.Assert(err, IsNil)
	c.Assert(string(decodedRequest), Equals, "<AuthnRequest xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" IssueInstant=\"2015-12-01T01:57:09.123Z\" AssertionConsumerServiceURL=\"https://15661444.ngrok.io/saml2/acs\" Destination=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" ID=\"id-00020406080a0c0e10121416181a1c1e20222426\" ProtocolBinding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Version=\"2.0\"><Issuer xmlns=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://15661444.ngrok.io/saml2/metadata</Issuer><NameIDPolicy xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" AllowCreate=\"true\">urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDPolicy></AuthnRequest>")
}

func (test *MiddlewareTest) TestRequireAccountPanicOnRequestToACS(c *C) {
	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("POST", "https://15661444.ngrok.io/saml2/acs", nil)
	resp := httptest.NewRecorder()
	c.Assert(func() { handler.ServeHTTP(resp, req) }, Panics,
		"don't wrap Middleware with RequireAccount")
}

func (test *MiddlewareTest) TestRejectRequestWithMagicHeader(c *C) {
	handler := test.Middleware.RequireAccount(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	req.Header.Set("X-Saml-Uid", "root") // ... evil
	resp := httptest.NewRecorder()
	c.Assert(func() { handler.ServeHTTP(resp, req) }, Panics,
		"X-Saml-* headers should not exist when this function is called")
}

func (test *MiddlewareTest) TestRequireAttribute(c *C) {
	handler := test.Middleware.RequireAccount(
		RequireAttribute("eduPersonAffiliation", "Staff")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusTeapot)
}

func (test *MiddlewareTest) TestRequireAttributeWrongValue(c *C) {
	handler := test.Middleware.RequireAccount(
		RequireAttribute("eduPersonAffiliation", "DomainAdmins")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("not reached")
			})))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusForbidden)
}

func (test *MiddlewareTest) TestRequireAttributeNotPresent(c *C) {
	handler := test.Middleware.RequireAccount(
		RequireAttribute("valueThatDoesntExist", "doesntMatter")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("not reached")
			})))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusForbidden)
}

func (test *MiddlewareTest) TestRequireAttributeMissingAccount(c *C) {
	handler := RequireAttribute("eduPersonAffiliation", "DomainAdmins")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("not reached")
		}))

	req, _ := http.NewRequest("GET", "/frob", nil)
	req.Header.Set("Cookie", ""+
		"ttt="+expectedToken+"; "+
		"Path=/; Max-Age=7200")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	c.Assert(resp.Code, Equals, http.StatusForbidden)
}

func (test *MiddlewareTest) TestCanParseResponse(c *C) {
	v := &url.Values{}
	v.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	v.Set("RelayState", "KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6")
	req, _ := http.NewRequest("POST", "/saml2/acs", bytes.NewReader([]byte(v.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", ""+
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6="+
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImlkLTllNjE3NTNkNjRlOTI4YWY1YTdhMzQxYTk3ZjQyMGM5IiwidXJpIjoiL2Zyb2IifQ.RHNEmqXQcB_ncWZSPkhOL4Sx5hZFD6eHP1RJ0ZgUhuk")

	resp := httptest.NewRecorder()
	test.Middleware.ServeHTTP(resp, req)
	c.Assert(resp.Code, Equals, http.StatusFound)

	c.Assert(resp.Header().Get("Location"), Equals, "/frob")
	c.Assert(resp.Header()["Set-Cookie"], DeepEquals, []string{
		"saml_KCosLjAyNDY4Ojw-QEJERkhKTE5QUlRWWFpcXmBiZGZoamxucHJ0dnh6=",
		"ttt=" + expectedToken + "; " +
			"Path=/; Max-Age=7200",
	})
}

func (test *MiddlewareTest) TestRejectsInvalidRelayState(c *C) {
	v := &url.Values{}
	v.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	v.Set("RelayState", "ICIkJigqLC4wMjQ2ODo8PkBCREZISkxOUFJUVlhaXF5gYmRmaGpsbnBy")
	req, _ := http.NewRequest("POST", "/saml2/acs", bytes.NewReader([]byte(v.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()
	test.Middleware.ServeHTTP(resp, req)
	c.Assert(resp.Code, Equals, http.StatusForbidden)
	c.Assert(resp.Header().Get("Location"), Equals, "")
	c.Assert(resp.Header().Get("Set-Cookie"), Equals, "")
}

func (test *MiddlewareTest) TestHandlesInvalidResponse(c *C) {
	v := &url.Values{}
	v.Set("SAMLResponse", "this is not a valid saml response")
	v.Set("RelayState", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1cmkiOiIvZnJvYiJ9.QEnpCWpKnhmzWZyfI8GIgCCWyH7qTly8vw-V4oJ1ni0")
	req, _ := http.NewRequest("POST", "/saml2/acs", bytes.NewReader([]byte(v.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()
	test.Middleware.ServeHTTP(resp, req)

	// note: it is important that when presented with an invalid request,
	// the ACS handles DOES NOT reveal detailed error information in the
	// HTTP response.
	c.Assert(resp.Code, Equals, http.StatusForbidden)
	respBody, _ := ioutil.ReadAll(resp.Body)
	c.Assert(string(respBody), Equals, "Forbidden\n")
	c.Assert(resp.Header().Get("Location"), Equals, "")
	c.Assert(resp.Header().Get("Set-Cookie"), Equals, "")
}
