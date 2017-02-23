package saml

import (
	"encoding/base64"
	"encoding/xml"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml/testsaml"

	. "gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type ServiceProviderTest struct {
	AuthnRequest string
	SamlResponse string
	Key          string
	Certificate  string
	IDPMetadata  string
}

// Helper to decode SAML redirect binding requests
// http://play.golang.org/p/sTlV0pCS2y
//     x1 := "lJJBj9MwEIX%2FSuR7Y4%2FJRisriVS2Qqq0QNUAB27GmbYWiV08E6D%2FHqeA6AnKdfz85nvPbtYzn8Iev8xIXHyfxkCtmFMw0ZInE%2ByEZNiZfv362ehSmXOKHF0cRbEmwsQ%2BhqcYaJ4w9Zi%2Beofv98%2BtODGfyUgJD3UNVVWV4Zji59JHSXYatbSORLHJO32wi8efG344l5wP6OQ%2FlTEdl4HMWw9%2BRLlgaLnHwSd0LPv%2BrSi2m1b4YaWU0qpStXpUVjmFoEBDBTU8ggUHmIVEM24DsQ3cCq3gYQV6peCdAvMCjIaPotj9ivfSh8GHYytE8QETXQlzfNE1V5d0T1X2d0GieBXTZPnv8mWScxyuUoOBPV9E968iJ2Q7WLaN%2FAnWNW%2Byz3azi6N3l%2F980XGM354SWsZWcJpRdPcDc7KBfMZu5C1B18jbL9b9CAAA%2F%2F8%3D"
//     x2, _ := url.QueryUnescape(x1)
//     x3, _ := base64.StdEncoding.DecodeString(x2)
//     x4, _ := ioutil.ReadAll(flate.NewReader(bytes.NewReader(x3)))
//     fmt.Printf("%s\n", x4)

var _ = Suite(&ServiceProviderTest{})

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

func (test *ServiceProviderTest) SetUpTest(c *C) {
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 MST 2006", "Mon Dec 1 01:57:09 UTC 2015")
		return rv
	}
	RandReader = &testRandomReader{}

	test.AuthnRequest = `https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO?RelayState=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1cmkiOiIvIn0.eoUmy2fQduAz--6N82xIOmufY1ZZeRi5x--B7m1pNIY&SAMLRequest=lJJBj9MwEIX%2FSuR7Yzt10sZKIpWtkCotsGqB%2B5BMW4vELp4JsP8et4DYE5Tr%2BPnN957dbGY%2B%2Bz1%2BmZE4%2Bz6NnloxR28DkCPrYUKy3NvD5s2jLXJlLzFw6MMosg0RRnbBPwRP84TxgPGr6%2FHD%2FrEVZ%2BYLWSl1WVXaGJP7UwyfcxckwTQWEnoS2TbtdB6uHn9uuOGSczqgs%2FuUh3i6DmTaenQjyitGIfc4uIg9y8Phnch221a4YVFjpVflcqgM1sUajiWsYGk01KujKVRfJyHRjDtPDJ5bUShdLrReLNX7QtmysrrMK6Pqem3MeqFKq5TInn6lfeX84PypFSL7iJFuwKkN0TU303hPc%2FC7L5G9DnEC%2Frv8OkmxjjepRc%2BOn0X3r14nZBiAoZE%2FwbrmbfLZbZ%2FC6Prn%2F3zgcQzfHiICYys4zii6%2B4E5gieXsBv5kqBr5Msf1%2F0IAAD%2F%2Fw%3D%3D`
	test.SamlResponse = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><saml2p:Response xmlns:saml2p=\"urn:oasis:names:tc:SAML:2.0:protocol\" Destination=\"https://15661444.ngrok.io/saml2/acs\" ID=\"_e9b3332eeaf348da6786aed16300aca9\" InResponseTo=\"id-9e61753d64e928af5a7a341a97f420c9\" IssueInstant=\"2015-12-01T01:56:21.375Z\" Version=\"2.0\"><saml2:Issuer xmlns:saml2=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://idp.testshib.org/idp/shibboleth</saml2:Issuer><saml2p:Status><saml2p:StatusCode Value=\"urn:oasis:names:tc:SAML:2.0:status:Success\"/></saml2p:Status><saml2:EncryptedAssertion xmlns:saml2=\"urn:oasis:names:tc:SAML:2.0:assertion\"><xenc:EncryptedData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\" Id=\"_dab0b1dbbc0595ab06473034e3bb798c\" Type=\"http://www.w3.org/2001/04/xmlenc#Element\"><xenc:EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"/><ds:KeyInfo xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"><xenc:EncryptedKey Id=\"_dd9264352cef16103cdb21fae97fa951\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><ds:DigestMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#sha1\" xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"/></xenc:EncryptionMethod><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJVUzELMAkGA1UE\nCAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTAwMjAwMDg1MVoX\nDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28x\nEjAQBgNVBAMMCWxvY2FsaG9zdDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308\nkWLhZVT4vOulqx/9ibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTv\nSPmHO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKvRsw0X/gf\nnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgkakpMdAqJfs24maGb90Dv\nTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeTQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+\ncvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvnOwJlNCASPZRH/JmF8tX0hoHuAQ==</ds:X509Certificate></ds:X509Data></ds:KeyInfo><xenc:CipherData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:CipherValue>i/wh2ubXbhTH5W3hwc5VEf4DH1xifeTuxoe64ULopGJ0M0XxBKgDEIfTg59JUMmDYB4L8UStTFfqJk9BRGcMeYWVfckn5gCwLptD9cz26irw+7Ud7MIorA7z68v8rEyzwagKjz8VKvX1afgec0wobVTNN3M1Bn+SOyMhAu+Z4tE=</xenc:CipherValue></xenc:CipherData></xenc:EncryptedKey></ds:KeyInfo><xenc:CipherData xmlns:xenc=\"http://www.w3.org/2001/04/xmlenc#\"><xenc:CipherValue>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</xenc:CipherValue></xenc:CipherData></xenc:EncryptedData></saml2:EncryptedAssertion></saml2p:Response>"
	test.Key = "-----BEGIN RSA PRIVATE KEY-----\nMIICXgIBAAKBgQDU8wdiaFmPfTyRYuFlVPi866WrH/2JubkHzp89bBQopDaLXYxi\n3PTu3O6Q/KaKxMOFBqrInwqpv/omOGZ4ycQ51O9I+Yc7ybVlW94lTo2gpGf+Y/8E\nPsVbnZaFutRctJ4dVIp9aQ2TpLiGT0xX1OzBO/JEgq9GzDRf+B+eqSuglwIDAQAB\nAoGBAMuy1eN6cgFiCOgBsB3gVDdTKpww87Qk5ivjqEt28SmXO13A1KNVPS6oQ8SJ\nCT5Azc6X/BIAoJCURVL+LHdqebogKljhH/3yIel1kH19vr4E2kTM/tYH+qj8afUS\nJEmArUzsmmK8ccuNqBcllqdwCZjxL4CHDUmyRudFcHVX9oyhAkEA/OV1OkjM3CLU\nN3sqELdMmHq5QZCUihBmk3/N5OvGdqAFGBlEeewlepEVxkh7JnaNXAXrKHRVu/f/\nfbCQxH+qrwJBANeQERF97b9Sibp9xgolb749UWNlAdqmEpmlvmS202TdcaaT1msU\n4rRLiQN3X9O9mq4LZMSVethrQAdX1whawpkCQQDk1yGf7xZpMJ8F4U5sN+F4rLyM\nRq8Sy8p2OBTwzCUXXK+fYeXjybsUUMr6VMYTRP2fQr/LKJIX+E5ZxvcIyFmDAkEA\nyfjNVUNVaIbQTzEbRlRvT6MqR+PTCefC072NF9aJWR93JimspGZMR7viY6IM4lrr\nvBkm0F5yXKaYtoiiDMzlOQJADqmEwXl0D72ZG/2KDg8b4QZEmC9i5gidpQwJXUc6\nhU+IVQoLxRq0fBib/36K9tcrrO5Ba4iEvDcNY+D8yGbUtA==\n-----END RSA PRIVATE KEY-----\n"
	test.Certificate = "-----BEGIN CERTIFICATE-----\nMIIB7zCCAVgCCQDFzbKIp7b3MTANBgkqhkiG9w0BAQUFADA8MQswCQYDVQQGEwJV\nUzELMAkGA1UECAwCR0ExDDAKBgNVBAoMA2ZvbzESMBAGA1UEAwwJbG9jYWxob3N0\nMB4XDTEzMTAwMjAwMDg1MVoXDTE0MTAwMjAwMDg1MVowPDELMAkGA1UEBhMCVVMx\nCzAJBgNVBAgMAkdBMQwwCgYDVQQKDANmb28xEjAQBgNVBAMMCWxvY2FsaG9zdDCB\nnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA1PMHYmhZj308kWLhZVT4vOulqx/9\nibm5B86fPWwUKKQ2i12MYtz07tzukPymisTDhQaqyJ8Kqb/6JjhmeMnEOdTvSPmH\nO8m1ZVveJU6NoKRn/mP/BD7FW52WhbrUXLSeHVSKfWkNk6S4hk9MV9TswTvyRIKv\nRsw0X/gfnqkroJcCAwEAATANBgkqhkiG9w0BAQUFAAOBgQCMMlIO+GNcGekevKgk\nakpMdAqJfs24maGb90DvTLbRZRD7Xvn1MnVBBS9hzlXiFLYOInXACMW5gcoRFfeT\nQLSouMM8o57h0uKjfTmuoWHLQLi6hnF+cvCsEFiJZ4AbF+DgmO6TarJ8O05t8zvn\nOwJlNCASPZRH/JmF8tX0hoHuAQ==\n-----END CERTIFICATE-----\n"
	test.IDPMetadata = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\" xmlns:mdalg=\"urn:oasis:names:tc:SAML:metadata:algsupport\" xmlns:mdui=\"urn:oasis:names:tc:SAML:metadata:ui\" xmlns:shibmd=\"urn:mace:shibboleth:metadata:1.0\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" Name=\"urn:mace:shibboleth:testshib:two\" entityID=\"https://idp.testshib.org/idp/shibboleth\">\n\t<Extensions>\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha512\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#sha384\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha256\" />\n\t\t<mdalg:DigestMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#sha1\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha512\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha384\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha256\" />\n\t\t<mdalg:SigningMethod Algorithm=\"http://www.w3.org/2000/09/xmldsig#rsa-sha1\" />\n\t</Extensions>\n\t<IDPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:1.1:protocol urn:mace:shibboleth:1.0 urn:oasis:names:tc:SAML:2.0:protocol\">\n\t\t<Extensions>\n\t\t\t<shibmd:Scope regexp=\"false\">testshib.org</shibmd:Scope>\n\t\t\t<mdui:UIInfo>\n\t\t\t\t<mdui:DisplayName xml:lang=\"en\">TestShib Test IdP</mdui:DisplayName>\n\t\t\t\t<mdui:Description xml:lang=\"en\">TestShib IdP. Use this as a source of attributes\n                        for your test SP.</mdui:Description>\n\t\t\t\t<mdui:Logo height=\"88\" width=\"253\">https://www.testshib.org/testshibtwo.jpg</mdui:Logo>\n\t\t\t</mdui:UIInfo>\n\t\t</Extensions>\n\t\t<KeyDescriptor>\n\t\t\t<ds:KeyInfo>\n\t\t\t\t<ds:X509Data>\n\t\t\t\t\t<ds:X509Certificate>MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV\n                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD\n                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4\n                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI\n                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl\n                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B\n                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C\n                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe\n                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT\n                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614\n                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH\n                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G\n                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86\n                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl\n                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo\n                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN\n                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL\n                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo\n                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4\n                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj\n                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr\n                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==</ds:X509Certificate>\n\t\t\t\t</ds:X509Data>\n\t\t\t</ds:KeyInfo>\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes256-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes192-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#tripledes-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-1_5\" />\n\t\t</KeyDescriptor>\n\t\t<ArtifactResolutionService Binding=\"urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/ArtifactResolution\" index=\"1\" />\n\t\t<ArtifactResolutionService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/ArtifactResolution\" index=\"2\" />\n\t\t<NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>\n\t\t<NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>\n\t\t<SingleSignOnService Binding=\"urn:mace:shibboleth:1.0:profiles:AuthnRequest\" Location=\"https://idp.testshib.org/idp/profile/Shibboleth/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://idp.testshib.org/idp/profile/SAML2/POST/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" />\n\t\t<SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org/idp/profile/SAML2/SOAP/ECP\" />\n\t</IDPSSODescriptor>\n\t<AttributeAuthorityDescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:1.1:protocol urn:oasis:names:tc:SAML:2.0:protocol\">\n\t\t<KeyDescriptor>\n\t\t\t<ds:KeyInfo>\n\t\t\t\t<ds:X509Data>\n\t\t\t\t\t<ds:X509Certificate>MIIEDjCCAvagAwIBAgIBADANBgkqhkiG9w0BAQUFADBnMQswCQYDVQQGEwJVUzEV\n                            MBMGA1UECBMMUGVubnN5bHZhbmlhMRMwEQYDVQQHEwpQaXR0c2J1cmdoMREwDwYD\n                            VQQKEwhUZXN0U2hpYjEZMBcGA1UEAxMQaWRwLnRlc3RzaGliLm9yZzAeFw0wNjA4\n                            MzAyMTEyMjVaFw0xNjA4MjcyMTEyMjVaMGcxCzAJBgNVBAYTAlVTMRUwEwYDVQQI\n                            EwxQZW5uc3lsdmFuaWExEzARBgNVBAcTClBpdHRzYnVyZ2gxETAPBgNVBAoTCFRl\n                            c3RTaGliMRkwFwYDVQQDExBpZHAudGVzdHNoaWIub3JnMIIBIjANBgkqhkiG9w0B\n                            AQEFAAOCAQ8AMIIBCgKCAQEArYkCGuTmJp9eAOSGHwRJo1SNatB5ZOKqDM9ysg7C\n                            yVTDClcpu93gSP10nH4gkCZOlnESNgttg0r+MqL8tfJC6ybddEFB3YBo8PZajKSe\n                            3OQ01Ow3yT4I+Wdg1tsTpSge9gEz7SrC07EkYmHuPtd71CHiUaCWDv+xVfUQX0aT\n                            NPFmDixzUjoYzbGDrtAyCqA8f9CN2txIfJnpHE6q6CmKcoLADS4UrNPlhHSzd614\n                            kR/JYiks0K4kbRqCQF0Dv0P5Di+rEfefC6glV8ysC8dB5/9nb0yh/ojRuJGmgMWH\n                            gWk6h0ihjihqiu4jACovUZ7vVOCgSE5Ipn7OIwqd93zp2wIDAQABo4HEMIHBMB0G\n                            A1UdDgQWBBSsBQ869nh83KqZr5jArr4/7b+QazCBkQYDVR0jBIGJMIGGgBSsBQ86\n                            9nh83KqZr5jArr4/7b+Qa6FrpGkwZzELMAkGA1UEBhMCVVMxFTATBgNVBAgTDFBl\n                            bm5zeWx2YW5pYTETMBEGA1UEBxMKUGl0dHNidXJnaDERMA8GA1UEChMIVGVzdFNo\n                            aWIxGTAXBgNVBAMTEGlkcC50ZXN0c2hpYi5vcmeCAQAwDAYDVR0TBAUwAwEB/zAN\n                            BgkqhkiG9w0BAQUFAAOCAQEAjR29PhrCbk8qLN5MFfSVk98t3CT9jHZoYxd8QMRL\n                            I4j7iYQxXiGJTT1FXs1nd4Rha9un+LqTfeMMYqISdDDI6tv8iNpkOAvZZUosVkUo\n                            93pv1T0RPz35hcHHYq2yee59HJOco2bFlcsH8JBXRSRrJ3Q7Eut+z9uo80JdGNJ4\n                            /SJy5UorZ8KazGj16lfJhOBXldgrhppQBb0Nq6HKHguqmwRfJ+WkxemZXzhediAj\n                            Geka8nz8JjwxpUjAiSWYKLtJhGEaTqCYxCCX2Dw+dOTqUzHOZ7WKv4JXPK5G/Uhr\n                            8K/qhmFT2nIQi538n6rVYLeWj8Bbnl+ev0peYzxFyF5sQA==</ds:X509Certificate>\n\t\t\t\t</ds:X509Data>\n\t\t\t</ds:KeyInfo>\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes256-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes192-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#aes128-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#tripledes-cbc\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p\" />\n\t\t\t<EncryptionMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#rsa-1_5\" />\n\t\t</KeyDescriptor>\n\t\t<AttributeService Binding=\"urn:oasis:names:tc:SAML:1.0:bindings:SOAP-binding\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML1/SOAP/AttributeQuery\" />\n\t\t<AttributeService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:SOAP\" Location=\"https://idp.testshib.org:8443/idp/profile/SAML2/SOAP/AttributeQuery\" />\n\t\t<NameIDFormat>urn:mace:shibboleth:1.0:nameIdentifier</NameIDFormat>\n\t\t<NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>\n\t</AttributeAuthorityDescriptor>\n\t<Organization>\n\t\t<OrganizationName xml:lang=\"en\">TestShib Two Identity Provider</OrganizationName>\n\t\t<OrganizationDisplayName xml:lang=\"en\">TestShib Two</OrganizationDisplayName>\n\t\t<OrganizationURL xml:lang=\"en\">http://www.testshib.org/testshib-two/</OrganizationURL>\n\t</Organization>\n\t<ContactPerson contactType=\"technical\">\n\t\t<GivenName>Nate</GivenName>\n\t\t<SurName>Klingenstein</SurName>\n\t\t<EmailAddress>ndk@internet2.edu</EmailAddress>\n\t</ContactPerson>\n</EntityDescriptor>"
}

func (test *ServiceProviderTest) TestCanSetAuthenticationNameIDFormat(c *C) {
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
	}

	// defaults to "transient"
	req, err := s.MakeAuthenticationRequest("")
	c.Assert(err, IsNil)
	c.Assert(req.NameIDPolicy.Format, Equals, string(TransientNameIDFormat))

	// explicitly set to "transient"
	s.AuthnNameIDFormat = TransientNameIDFormat
	req, err = s.MakeAuthenticationRequest("")
	c.Assert(err, IsNil)
	c.Assert(req.NameIDPolicy.Format, Equals, string(TransientNameIDFormat))

	// explicitly set to "unspecified"
	s.AuthnNameIDFormat = UnspecifiedNameIDFormat
	req, err = s.MakeAuthenticationRequest("")
	c.Assert(err, IsNil)
	c.Assert(req.NameIDPolicy.Format, Equals, "")

	// explicitly set to "emailAddress"
	s.AuthnNameIDFormat = EmailAddressNameIDFormat
	req, err = s.MakeAuthenticationRequest("")
	c.Assert(err, IsNil)
	c.Assert(req.NameIDPolicy.Format, Equals, string(EmailAddressNameIDFormat))
}

func (test *ServiceProviderTest) TestCanProduceMetadata(c *C) {
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://example.com/saml2/metadata",
		AcsURL:      "https://example.com/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	spMetadata, err := xml.MarshalIndent(s.Metadata(), "", "  ")
	c.Assert(err, IsNil)
	c.Assert(string(spMetadata), DeepEquals, ""+
		"<EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" validUntil=\"2015-12-03T01:57:09Z\" entityID=\"https://example.com/saml2/metadata\">\n"+
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
		"    <AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://example.com/saml2/acs\" index=\"1\"></AssertionConsumerService>\n"+
		"  </SPSSODescriptor>\n"+
		"</EntityDescriptor>")
}

func (test *ServiceProviderTest) TestCanProduceRedirectRequest(c *C) {
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05.999999999 UTC 2006", "Mon Dec 1 01:31:21.123456789 UTC 2015")
		return rv
	}
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	redirectURL, err := s.MakeRedirectAuthenticationRequest("relayState")
	c.Assert(err, IsNil)

	decodedRequest, err := testsaml.ParseRedirectRequest(redirectURL)
	c.Assert(err, IsNil)
	c.Assert(redirectURL.Host, Equals, "idp.testshib.org")
	c.Assert(redirectURL.Path, Equals, "/idp/profile/SAML2/Redirect/SSO")
	c.Assert(string(decodedRequest), Equals, "<AuthnRequest xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" IssueInstant=\"2015-12-01T01:31:21.123Z\" AssertionConsumerServiceURL=\"https://15661444.ngrok.io/saml2/acs\" Destination=\"https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO\" ID=\"id-00020406080a0c0e10121416181a1c1e20222426\" ProtocolBinding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Version=\"2.0\"><Issuer xmlns=\"urn:oasis:names:tc:SAML:2.0:assertion\" Format=\"urn:oasis:names:tc:SAML:2.0:nameid-format:entity\">https://15661444.ngrok.io/saml2/metadata</Issuer><NameIDPolicy xmlns=\"urn:oasis:names:tc:SAML:2.0:protocol\" AllowCreate=\"true\">urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDPolicy></AuthnRequest>")
}

func (test *ServiceProviderTest) TestCanProducePostRequest(c *C) {
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 UTC 2006", "Mon Dec 1 01:31:21 UTC 2015")
		return rv
	}
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	form, err := s.MakePostAuthenticationRequest("relayState")
	c.Assert(err, IsNil)

	c.Assert(string(form), Equals, ``+
		`<form method="post" action="https://idp.testshib.org/idp/profile/SAML2/POST/SSO" id="SAMLRequestForm">`+
		`<input type="hidden" name="SAMLRequest" value="PEF1dGhuUmVxdWVzdCB4bWxucz0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBJc3N1ZUluc3RhbnQ9IjIwMTUtMTItMDFUMDE6MzE6MjFaIiBBc3NlcnRpb25Db25zdW1lclNlcnZpY2VVUkw9Imh0dHBzOi8vMTU2NjE0NDQubmdyb2suaW8vc2FtbDIvYWNzIiBEZXN0aW5hdGlvbj0iaHR0cHM6Ly9pZHAudGVzdHNoaWIub3JnL2lkcC9wcm9maWxlL1NBTUwyL1BPU1QvU1NPIiBJRD0iaWQtMDAwMjA0MDYwODBhMGMwZTEwMTIxNDE2MTgxYTFjMWUyMDIyMjQyNiIgUHJvdG9jb2xCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBWZXJzaW9uPSIyLjAiPjxJc3N1ZXIgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24iIEZvcm1hdD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOm5hbWVpZC1mb3JtYXQ6ZW50aXR5Ij5odHRwczovLzE1NjYxNDQ0Lm5ncm9rLmlvL3NhbWwyL21ldGFkYXRhPC9Jc3N1ZXI&#43;PE5hbWVJRFBvbGljeSB4bWxucz0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBbGxvd0NyZWF0ZT0idHJ1ZSI&#43;dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOm5hbWVpZC1mb3JtYXQ6dHJhbnNpZW50PC9OYW1lSURQb2xpY3k&#43;PC9BdXRoblJlcXVlc3Q&#43;" />`+
		`<input type="hidden" name="RelayState" value="relayState" />`+
		`<input id="SAMLSubmitButton" type="submit" value="Submit" /></form>`+
		`<script>document.getElementByID('SAMLSubmitButton').style.visibility="hidden";</script>`+
		`<script>document.getElementById('SAMLRequestForm').submit();</script>`)
}

func (test *ServiceProviderTest) TestCanHandleOneloginResponse(c *C) {
	// An actual response from onelogin
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 UTC 2006", "Tue Jan 5 17:53:12 UTC 2016")
		return rv
	}
	SamlResponse := `PHNhbWxwOlJlc3BvbnNlIHhtbG5zOnNhbWw9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24iIHhtbG5zOnNhbWxwPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6cHJvdG9jb2wiIElEPSJwZnhlZDg4YzQzZC02NTA0LWUxZjEtNWFmMC00MGJlN2YyNzlmYzUiIFZlcnNpb249IjIuMCIgSXNzdWVJbnN0YW50PSIyMDE2LTAxLTA1VDE3OjUzOjExWiIgRGVzdGluYXRpb249Imh0dHBzOi8vMjllZTZkMmUubmdyb2suaW8vc2FtbC9hY3MiIEluUmVzcG9uc2VUbz0iaWQtZDQwYzE1YzEwNGI1MjY5MWVjY2YwYTJhNWM4YTE1NTk1YmU3NTQyMyI+PHNhbWw6SXNzdWVyPmh0dHBzOi8vYXBwLm9uZWxvZ2luLmNvbS9zYW1sL21ldGFkYXRhLzUwMzk4Mzwvc2FtbDpJc3N1ZXI+PGRzOlNpZ25hdHVyZSB4bWxuczpkcz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+PGRzOlNpZ25lZEluZm8+PGRzOkNhbm9uaWNhbGl6YXRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMiLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI3JzYS1zaGExIi8+PGRzOlJlZmVyZW5jZSBVUkk9IiNwZnhlZDg4YzQzZC02NTA0LWUxZjEtNWFmMC00MGJlN2YyNzlmYzUiPjxkczpUcmFuc2Zvcm1zPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjZW52ZWxvcGVkLXNpZ25hdHVyZSIvPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMiLz48L2RzOlRyYW5zZm9ybXM+PGRzOkRpZ2VzdE1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNzaGExIi8+PGRzOkRpZ2VzdFZhbHVlPlNWQWFRZzh2bW1TUUw2L1lCbVMyeWRLUlA3ST08L2RzOkRpZ2VzdFZhbHVlPjwvZHM6UmVmZXJlbmNlPjwvZHM6U2lnbmVkSW5mbz48ZHM6U2lnbmF0dXJlVmFsdWU+c0JlVFZQMGJab1BSK2JmeUFrVnY2STNDVjdZOFhxbkoycjhmMStXbXIyZ0ZnblJGODVOdnZTUCtyMUJvN250dU9zd080ZkI0Uks0SHlTYnlsZzRiS0hLSDE5WDkxaFZBekpTeXNmbVMvZDV3ZzFDZmlXV3Q1UzJIQTUwOHRoWHVabndHM1h6NktuV0s4a1JkeDFkYytZUldnYUZ5ZDRnTEc5YUJUc1hPWjd2eC83UDRicnpORW00d1A5LzB0dWZ4Rytuc1k2RHB3bkVHQ2psK1ZVS3BnekVxd05OalFxWUZZU0FYRWsrVnQrWDNjMmQwSElyWlF2WW5OaDAyS3h1d1ZCVGhuM01helFOYU54Qy9zeWYza0RRQ1JyWkNZbytZdER1ZHpKVTlwM0EwWVhIVFFjc2RldHNIWlhDTWozbXV2emMwbUVCbHc0TGJjaEttbmJ5Wm1nPT08L2RzOlNpZ25hdHVyZVZhbHVlPjxkczpLZXlJbmZvPjxkczpYNTA5RGF0YT48ZHM6WDUwOUNlcnRpZmljYXRlPk1JSUVDRENDQXZDZ0F3SUJBZ0lVWHVuMDhDc2xMUldTTHFObkRFMU50R0plZmwwd0RRWUpLb1pJaHZjTkFRRUZCUUF3VXpFTE1Ba0dBMVVFQmhNQ1ZWTXhEREFLQmdOVkJBb01BMk4wZFRFVk1CTUdBMVVFQ3d3TVQyNWxURzluYVc0Z1NXUlFNUjh3SFFZRFZRUUREQlpQYm1WTWIyZHBiaUJCWTJOdmRXNTBJRE15TmpFME1CNFhEVEV6TURrek1ERTVNelUwTkZvWERURTRNVEF3TVRFNU16VTBORm93VXpFTE1Ba0dBMVVFQmhNQ1ZWTXhEREFLQmdOVkJBb01BMk4wZFRFVk1CTUdBMVVFQ3d3TVQyNWxURzluYVc0Z1NXUlFNUjh3SFFZRFZRUUREQlpQYm1WTWIyZHBiaUJCWTJOdmRXNTBJRE15TmpFME1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBME9HOFY4bWhvdmtqNHJoR2hqcmJFeFJZYnpLVjJaeGZ2R2ZFR1hHVXZYYzZEcWVqWUVkaFoybUlmQ0RvamhRamswQnl3aWlyQUtNT3QxR051SDdhV0lFNDdEMGV3dEs1eWxFQW03ZVZtb1k0a3hMQ2FXNXdZckMxU3pNbnBlaXRVeHF2c2JuS3ozalVLWUhSZ2dwZnZWajRzaUhEWmVJWmE5YTVyVXZwTW5uYk9vRmlaQ0lFTnBxM1RDMzNpdk9TWmhFTlJUem12bms1R0RvTEh3LzhxQWdRaXlUM0QxeENrU0JiNTRQSGdrUTVScTFvZExNL2hKK0wwanpDVVFINGd4cFdsRUFhYjRLOXM4ZnBCVUJCaDVnbUpDWWk4VWJJbGhxTzhOMm15bnVtMzNCVS92SjNQbmF3VDRZWWtUd1JVeDZZKzNmcG1SQkhxbDRoODNTTWV3SURBUUFCbzRIVE1JSFFNQXdHQTFVZEV3RUIvd1FDTUFBd0hRWURWUjBPQkJZRUZPZkZGakhGajlhNnhwbmdiMTFycmhnTWU5QXJNSUdRQmdOVkhTTUVnWWd3Z1lXQUZPZkZGakhGajlhNnhwbmdiMTFycmhnTWU5QXJvVmVrVlRCVE1Rc3dDUVlEVlFRR0V3SlZVekVNTUFvR0ExVUVDZ3dEWTNSMU1SVXdFd1lEVlFRTERBeFBibVZNYjJkcGJpQkpaRkF4SHpBZEJnTlZCQU1NRms5dVpVeHZaMmx1SUVGalkyOTFiblFnTXpJMk1UU0NGRjdwOVBBckpTMFZraTZqWnd4TlRiUmlYbjVkTUE0R0ExVWREd0VCL3dRRUF3SUhnREFOQmdrcWhraUc5dzBCQVFVRkFBT0NBUUVBTWdsbjROUE1RbjhHeXZxOENUUCtjMmU2Q1V6Y3ZSRUtuVGhqeFQ5V2N2VjFaVlhNQk5QbTRjVHFUMzYxRWRMelk1eVdMVVdYZDRBdkZuY2lxQjNNSFlhMm5xVG1udkxnbWhrV2UraGRGb05lNStJQThBeEduK25xVUlTbXlCZUN4dVVVQWJSTXVvd2lBcndISXB6cEV5UklZZFNaUk5GMGR2Z2lQWXlyL01pUFhJY3pwSDVuTGt2YkxwY0FGK1I4Wmg5bndZMGcxSlZ5YzZBQjZqN1lleHVVUVpwSEg0czBWZHgvbldtcmNGZUxaS0NUeGNhaEh2VTUwZTF5S1g1dGhmVmFKcUk4UVE3eFp4eXUwVFRzaWFYMHV3NTFKUE96UHVBUHBoMHo2eG9TOW9ZeHV6WjF5OXNOSEg2a0g4R0ZudlMyTXF5SGlOejBoMFNxL3E2bit3PT08L2RzOlg1MDlDZXJ0aWZpY2F0ZT48L2RzOlg1MDlEYXRhPjwvZHM6S2V5SW5mbz48L2RzOlNpZ25hdHVyZT48c2FtbHA6U3RhdHVzPjxzYW1scDpTdGF0dXNDb2RlIFZhbHVlPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6c3RhdHVzOlN1Y2Nlc3MiLz48L3NhbWxwOlN0YXR1cz48c2FtbDpBc3NlcnRpb24geG1sbnM6c2FtbD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmFzc2VydGlvbiIgeG1sbnM6eHM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvWE1MU2NoZW1hIiB4bWxuczp4c2k9Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvWE1MU2NoZW1hLWluc3RhbmNlIiBWZXJzaW9uPSIyLjAiIElEPSJBZDk0NWFlZGEzOGE1MDhmOGZhYzliYzk2MTNkNTk2NDJjMGQyZDhjYiIgSXNzdWVJbnN0YW50PSIyMDE2LTAxLTA1VDE3OjUzOjExWiI+PHNhbWw6SXNzdWVyPmh0dHBzOi8vYXBwLm9uZWxvZ2luLmNvbS9zYW1sL21ldGFkYXRhLzUwMzk4Mzwvc2FtbDpJc3N1ZXI+PHNhbWw6U3ViamVjdD48c2FtbDpOYW1lSUQgRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoxLjE6bmFtZWlkLWZvcm1hdDplbWFpbEFkZHJlc3MiPnJvc3NAa25kci5vcmc8L3NhbWw6TmFtZUlEPjxzYW1sOlN1YmplY3RDb25maXJtYXRpb24gTWV0aG9kPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6Y206YmVhcmVyIj48c2FtbDpTdWJqZWN0Q29uZmlybWF0aW9uRGF0YSBOb3RPbk9yQWZ0ZXI9IjIwMTYtMDEtMDVUMTc6NTY6MTFaIiBSZWNpcGllbnQ9Imh0dHBzOi8vMjllZTZkMmUubmdyb2suaW8vc2FtbC9hY3MiIEluUmVzcG9uc2VUbz0iaWQtZDQwYzE1YzEwNGI1MjY5MWVjY2YwYTJhNWM4YTE1NTk1YmU3NTQyMyIvPjwvc2FtbDpTdWJqZWN0Q29uZmlybWF0aW9uPjwvc2FtbDpTdWJqZWN0PjxzYW1sOkNvbmRpdGlvbnMgTm90QmVmb3JlPSIyMDE2LTAxLTA1VDE3OjUwOjExWiIgTm90T25PckFmdGVyPSIyMDE2LTAxLTA1VDE3OjU2OjExWiI+PHNhbWw6QXVkaWVuY2VSZXN0cmljdGlvbj48c2FtbDpBdWRpZW5jZT5odHRwczovLzI5ZWU2ZDJlLm5ncm9rLmlvL3NhbWwvbWV0YWRhdGE8L3NhbWw6QXVkaWVuY2U+PC9zYW1sOkF1ZGllbmNlUmVzdHJpY3Rpb24+PC9zYW1sOkNvbmRpdGlvbnM+PHNhbWw6QXV0aG5TdGF0ZW1lbnQgQXV0aG5JbnN0YW50PSIyMDE2LTAxLTA1VDE3OjUzOjEwWiIgU2Vzc2lvbk5vdE9uT3JBZnRlcj0iMjAxNi0wMS0wNlQxNzo1MzoxMVoiIFNlc3Npb25JbmRleD0iX2ViZGNiZTgwLTk1ZmYtMDEzMy1kODcxLTM4Y2EzYTY2MmYxYyI+PHNhbWw6QXV0aG5Db250ZXh0PjxzYW1sOkF1dGhuQ29udGV4dENsYXNzUmVmPnVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphYzpjbGFzc2VzOlBhc3N3b3JkUHJvdGVjdGVkVHJhbnNwb3J0PC9zYW1sOkF1dGhuQ29udGV4dENsYXNzUmVmPjwvc2FtbDpBdXRobkNvbnRleHQ+PC9zYW1sOkF1dGhuU3RhdGVtZW50PjxzYW1sOkF0dHJpYnV0ZVN0YXRlbWVudD48c2FtbDpBdHRyaWJ1dGUgTmFtZUZvcm1hdD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmF0dHJuYW1lLWZvcm1hdDpiYXNpYyIgTmFtZT0iVXNlci5lbWFpbCI+PHNhbWw6QXR0cmlidXRlVmFsdWUgeG1sbnM6eHNpPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxL1hNTFNjaGVtYS1pbnN0YW5jZSIgeHNpOnR5cGU9InhzOnN0cmluZyI+cm9zc0BrbmRyLm9yZzwvc2FtbDpBdHRyaWJ1dGVWYWx1ZT48L3NhbWw6QXR0cmlidXRlPjxzYW1sOkF0dHJpYnV0ZSBOYW1lRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXR0cm5hbWUtZm9ybWF0OmJhc2ljIiBOYW1lPSJtZW1iZXJPZiI+PHNhbWw6QXR0cmlidXRlVmFsdWUgeG1sbnM6eHNpPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxL1hNTFNjaGVtYS1pbnN0YW5jZSIgeHNpOnR5cGU9InhzOnN0cmluZyIvPjwvc2FtbDpBdHRyaWJ1dGU+PHNhbWw6QXR0cmlidXRlIE5hbWVGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphdHRybmFtZS1mb3JtYXQ6YmFzaWMiIE5hbWU9IlVzZXIuTGFzdE5hbWUiPjxzYW1sOkF0dHJpYnV0ZVZhbHVlIHhtbG5zOnhzaT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS9YTUxTY2hlbWEtaW5zdGFuY2UiIHhzaTp0eXBlPSJ4czpzdHJpbmciPktpbmRlcjwvc2FtbDpBdHRyaWJ1dGVWYWx1ZT48L3NhbWw6QXR0cmlidXRlPjxzYW1sOkF0dHJpYnV0ZSBOYW1lRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXR0cm5hbWUtZm9ybWF0OmJhc2ljIiBOYW1lPSJQZXJzb25JbW11dGFibGVJRCI+PHNhbWw6QXR0cmlidXRlVmFsdWUgeG1sbnM6eHNpPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxL1hNTFNjaGVtYS1pbnN0YW5jZSIgeHNpOnR5cGU9InhzOnN0cmluZyIvPjwvc2FtbDpBdHRyaWJ1dGU+PHNhbWw6QXR0cmlidXRlIE5hbWVGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphdHRybmFtZS1mb3JtYXQ6YmFzaWMiIE5hbWU9IlVzZXIuRmlyc3ROYW1lIj48c2FtbDpBdHRyaWJ1dGVWYWx1ZSB4bWxuczp4c2k9Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvWE1MU2NoZW1hLWluc3RhbmNlIiB4c2k6dHlwZT0ieHM6c3RyaW5nIj5Sb3NzPC9zYW1sOkF0dHJpYnV0ZVZhbHVlPjwvc2FtbDpBdHRyaWJ1dGU+PC9zYW1sOkF0dHJpYnV0ZVN0YXRlbWVudD48L3NhbWw6QXNzZXJ0aW9uPjwvc2FtbHA6UmVzcG9uc2U+Cgo=`
	test.IDPMetadata = `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://app.onelogin.com/saml/metadata/503983">
  <IDPSSODescriptor xmlns:ds="http://www.w3.org/2000/09/xmldsig#" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIIECDCCAvCgAwIBAgIUXun08CslLRWSLqNnDE1NtGJefl0wDQYJKoZIhvcNAQEF
BQAwUzELMAkGA1UEBhMCVVMxDDAKBgNVBAoMA2N0dTEVMBMGA1UECwwMT25lTG9n
aW4gSWRQMR8wHQYDVQQDDBZPbmVMb2dpbiBBY2NvdW50IDMyNjE0MB4XDTEzMDkz
MDE5MzU0NFoXDTE4MTAwMTE5MzU0NFowUzELMAkGA1UEBhMCVVMxDDAKBgNVBAoM
A2N0dTEVMBMGA1UECwwMT25lTG9naW4gSWRQMR8wHQYDVQQDDBZPbmVMb2dpbiBB
Y2NvdW50IDMyNjE0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OG8
V8mhovkj4rhGhjrbExRYbzKV2ZxfvGfEGXGUvXc6DqejYEdhZ2mIfCDojhQjk0By
wiirAKMOt1GNuH7aWIE47D0ewtK5ylEAm7eVmoY4kxLCaW5wYrC1SzMnpeitUxqv
sbnKz3jUKYHRggpfvVj4siHDZeIZa9a5rUvpMnnbOoFiZCIENpq3TC33ivOSZhEN
RTzmvnk5GDoLHw/8qAgQiyT3D1xCkSBb54PHgkQ5Rq1odLM/hJ+L0jzCUQH4gxpW
lEAab4K9s8fpBUBBh5gmJCYi8UbIlhqO8N2mynum33BU/vJ3PnawT4YYkTwRUx6Y
+3fpmRBHql4h83SMewIDAQABo4HTMIHQMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYE
FOfFFjHFj9a6xpngb11rrhgMe9ArMIGQBgNVHSMEgYgwgYWAFOfFFjHFj9a6xpng
b11rrhgMe9AroVekVTBTMQswCQYDVQQGEwJVUzEMMAoGA1UECgwDY3R1MRUwEwYD
VQQLDAxPbmVMb2dpbiBJZFAxHzAdBgNVBAMMFk9uZUxvZ2luIEFjY291bnQgMzI2
MTSCFF7p9PArJS0Vki6jZwxNTbRiXn5dMA4GA1UdDwEB/wQEAwIHgDANBgkqhkiG
9w0BAQUFAAOCAQEAMgln4NPMQn8Gyvq8CTP+c2e6CUzcvREKnThjxT9WcvV1ZVXM
BNPm4cTqT361EdLzY5yWLUWXd4AvFnciqB3MHYa2nqTmnvLgmhkWe+hdFoNe5+IA
8AxGn+nqUISmyBeCxuUUAbRMuowiArwHIpzpEyRIYdSZRNF0dvgiPYyr/MiPXIcz
pH5nLkvbLpcAF+R8Zh9nwY0g1JVyc6AB6j7YexuUQZpHH4s0Vdx/nWmrcFeLZKCT
xcahHvU50e1yKX5thfVaJqI8QQ7xZxyu0TTsiaX0uw51JPOzPuAPph0z6xoS9oYx
uzZ1y9sNHH6kH8GFnvS2MqyHiNz0h0Sq/q6n+w==</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </KeyDescriptor>
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</NameIDFormat>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://app.onelogin.com/trust/saml2/http-post/sso/503983"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://app.onelogin.com/trust/saml2/http-post/sso/503983"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP" Location="https://app.onelogin.com/trust/saml2/soap/sso/503983"/>
  </IDPSSODescriptor>
  <ContactPerson contactType="technical">
    <SurName>Support</SurName>
    <EmailAddress>support@onelogin.com</EmailAddress>
  </ContactPerson>
</EntityDescriptor>
`
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://29ee6d2e.ngrok.io/saml/metadata",
		AcsURL:      "https://29ee6d2e.ngrok.io/saml/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	req := http.Request{PostForm: url.Values{}}
	req.PostForm.Set("SAMLResponse", SamlResponse)
	assertion, err := s.ParseResponse(&req, []string{"id-d40c15c104b52691eccf0a2a5c8a15595be75423"})
	if err != nil {
		c.Logf("%s", err.(*InvalidResponseError).PrivateErr)
	}
	c.Assert(err, IsNil)

	c.Assert(assertion.Subject.NameID.Value, DeepEquals, "ross@kndr.org")
	c.Assert(assertion.AttributeStatement.Attributes, DeepEquals, []Attribute{
		{
			Name:       "User.email",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "ross@kndr.org",
				},
			},
		},
		{
			Name:       "memberOf",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "",
				},
			},
		},
		{
			Name:       "User.LastName",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Kinder",
				},
			},
		},
		{
			Name:       "PersonImmutableID",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "",
				},
			},
		},
		{
			Name:       "User.FirstName",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Ross",
				},
			},
		},
	})
}

func (test *ServiceProviderTest) TestCanHandlePlaintextResponse(c *C) {
	// An actual response from google
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 UTC 2006", "Tue Jan 5 16:55:39 UTC 2016")
		return rv
	}
	SamlResponse := "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiIHN0YW5kYWxvbmU9Im5vIj8+PHNhbWwycDpSZXNwb25zZSB4bWxuczpzYW1sMnA9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpwcm90b2NvbCIgRGVzdGluYXRpb249Imh0dHBzOi8vMjllZTZkMmUubmdyb2suaW8vc2FtbC9hY3MiIElEPSJfZmMxNDFkYjI4NGViMzA5ODYwNTM1MWJkZTRkOWJlNTkiIEluUmVzcG9uc2VUbz0iaWQtZmQ0MTlhNWFiMDQ3MjY0NTQyN2Y4ZTA3ZDg3YTNhNWRkMGIyZTlhNiIgSXNzdWVJbnN0YW50PSIyMDE2LTAxLTA1VDE2OjU1OjM5LjM0OFoiIFZlcnNpb249IjIuMCI+PHNhbWwyOklzc3VlciB4bWxuczpzYW1sMj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmFzc2VydGlvbiI+aHR0cHM6Ly9hY2NvdW50cy5nb29nbGUuY29tL28vc2FtbDI/aWRwaWQ9QzAyZGZsMXIxPC9zYW1sMjpJc3N1ZXI+PGRzOlNpZ25hdHVyZSB4bWxuczpkcz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+PGRzOlNpZ25lZEluZm8+PGRzOkNhbm9uaWNhbGl6YXRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMiLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxkc2lnLW1vcmUjcnNhLXNoYTI1NiIvPjxkczpSZWZlcmVuY2UgVVJJPSIjX2ZjMTQxZGIyODRlYjMwOTg2MDUzNTFiZGU0ZDliZTU5Ij48ZHM6VHJhbnNmb3Jtcz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI2VudmVsb3BlZC1zaWduYXR1cmUiLz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jIi8+PC9kczpUcmFuc2Zvcm1zPjxkczpEaWdlc3RNZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNzaGEyNTYiLz48ZHM6RGlnZXN0VmFsdWU+bHRNRUJLRzRZNVNLeERScUxHR2xFSGtPd3hla3dQOStybnA2WEtqdkJxVT08L2RzOkRpZ2VzdFZhbHVlPjwvZHM6UmVmZXJlbmNlPjwvZHM6U2lnbmVkSW5mbz48ZHM6U2lnbmF0dXJlVmFsdWU+SFBVV0pmYTlqdVdiKy9wZ0YrQklsc2pycE40NkE0RUNiT3hNdXhmWEFRUCtrMU5KMG9EdTJKYk1pZHpmclJBRkRHMjZaNjZWQWtkcwpBRmYwVFgzMWxvVjdaU0tGS0lVY0tuaFlXTHFuUTZLbmRydnJLbzF5UUhzUkdUNzJoVjl3SWdqTFRTZm5FV3QvOEMxaERQQi96R0txClhXZ3VvNFFHYlZUeVBoVVh3eEFzRmxBNjFDdkE5Q1pzU2xpeHBaY2pOVjUyQmMydzI5RUNRNStBcHZGWjVqRU1EN1JiQTVpMzdBbmgKUVBCeVYrZXo4ZU9Yc0hvQlhsR0drTjlDR201MFR6djZ3TW12WkdkT2pKWlhvRWZGUTA4UFJwbE9DQWpxSjM3QnhpWitLZWtUaE1KYgorelowcG1yeWR2V3lONEMzNWcycGVueGw2QUtxYnhMaXlJUkVaZz09PC9kczpTaWduYXR1cmVWYWx1ZT48ZHM6S2V5SW5mbz48ZHM6WDUwOURhdGE+PGRzOlg1MDlTdWJqZWN0TmFtZT5TVD1DYWxpZm9ybmlhLEM9VVMsT1U9R29vZ2xlIEZvciBXb3JrLENOPUdvb2dsZSxMPU1vdW50YWluIFZpZXcsTz1Hb29nbGUgSW5jLjwvZHM6WDUwOVN1YmplY3ROYW1lPjxkczpYNTA5Q2VydGlmaWNhdGU+TUlJRGREQ0NBbHlnQXdJQkFnSUdBVklTbElsWU1BMEdDU3FHU0liM0RRRUJDd1VBTUhzeEZEQVNCZ05WQkFvVEMwZHZiMmRzWlNCSgpibU11TVJZd0ZBWURWUVFIRXcxTmIzVnVkR0ZwYmlCV2FXVjNNUTh3RFFZRFZRUURFd1pIYjI5bmJHVXhHREFXQmdOVkJBc1REMGR2CmIyZHNaU0JHYjNJZ1YyOXlhekVMTUFrR0ExVUVCaE1DVlZNeEV6QVJCZ05WQkFnVENrTmhiR2xtYjNKdWFXRXdIaGNOTVRZd01UQTEKTVRZeE56UTVXaGNOTWpFd01UQXpNVFl4TnpRNVdqQjdNUlF3RWdZRFZRUUtFd3RIYjI5bmJHVWdTVzVqTGpFV01CUUdBMVVFQnhNTgpUVzkxYm5SaGFXNGdWbWxsZHpFUE1BMEdBMVVFQXhNR1IyOXZaMnhsTVJnd0ZnWURWUVFMRXc5SGIyOW5iR1VnUm05eUlGZHZjbXN4CkN6QUpCZ05WQkFZVEFsVlRNUk13RVFZRFZRUUlFd3BEWVd4cFptOXlibWxoTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEEKTUlJQkNnS0NBUUVBbVVmTVVQeEhTWS9aWVo4OGZVR0FsaFVQNE5pN3pqNTR2c3JzUERBNFVoUWlSZUVEUnVuTjFxM09Ic1NoUm9uZwpnZDRMdkE4My9lLzNwbS9WNjBSNnZ5TWZqM1ovSUdXWStlWjk3RUpVdmprdHQrVlJvQWkyNm9lWTlaVzZTODV5YXB2QTNpdWhFd0lRCk9jdVBtMU9xUlEweVE0c1VEK1d0TC9RU21sWXZEUDVUSzFkNndoVGlzTnNLU3FlRlpDYi9zOU9YMDFVZXhXMUJ1RE9MZVZ0MHJDVzEKa1JOY0JCTERtZDRobkRQMFNWcTduTGhORllYajJFYTZXc3lSQUl2Y2hhVUd5K0ltYTJva1htOTVZZTlrbjhlMTE4aS81clJleUtDbQpCbHNrTWtOYUE0S1dLdklRbTNEZGpnT05nRWQwSXZLRXh5THdZN2E1L0pJVXZCaGI5UUlEQVFBQk1BMEdDU3FHU0liM0RRRUJDd1VBCkE0SUJBUUFVRExNbkhwemZwNFNoZEJxQ3JlVzQ4ZjhyVTk0cTJxTXdyVStXNkRrT3JHSlRBU1ZHUzlSaWIvTUtBaVJZT21xbGFxRVkKTlA1N3BDckUvblJCNUZWZEUrQWxTeC9mUjNraHNRM3pmLzRkWXMyMVN2R2YrT2FzOTlYRWJXZlYwT21QTVltM0lyU0NPQkVWMzF3aAo0MXFSYzVRTG5SK1h1dE5QYlNCTit0bitnaVJDTEdDQkxlODFvVnc0ZlJHUWJna2Q4N3JmTE95M0c2MzBJNnMvSjVmZUZGVVQ4ZDdoCjltcE9lT3FMQ1ByS3BxK3dJM2FEM2xmNG1YcUtJRE5pSEhSb05sNjdBTlB1L04zZk5VMUhwbFZ0dnJvVnBpTnA4N2ZyZ2RsS1RFY2cKUFVrZmJhWUhRR1A2SVMwbHplQ2VEWDB3YWIzcVJvaDcvakp0NS9CUjhJd2Y8L2RzOlg1MDlDZXJ0aWZpY2F0ZT48L2RzOlg1MDlEYXRhPjwvZHM6S2V5SW5mbz48L2RzOlNpZ25hdHVyZT48c2FtbDJwOlN0YXR1cz48c2FtbDJwOlN0YXR1c0NvZGUgVmFsdWU9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpzdGF0dXM6U3VjY2VzcyIvPjwvc2FtbDJwOlN0YXR1cz48c2FtbDI6QXNzZXJ0aW9uIHhtbG5zOnNhbWwyPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXNzZXJ0aW9uIiBJRD0iXzllNzY0OTUyZTZhMjYxZTE5NDA5YTM4MjU1ODEwMzNkIiBJc3N1ZUluc3RhbnQ9IjIwMTYtMDEtMDVUMTY6NTU6MzkuMzQ4WiIgVmVyc2lvbj0iMi4wIj48c2FtbDI6SXNzdWVyPmh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL3NhbWwyP2lkcGlkPUMwMmRmbDFyMTwvc2FtbDI6SXNzdWVyPjxzYW1sMjpTdWJqZWN0PjxzYW1sMjpOYW1lSUQ+cm9zc0BvY3RvbGFicy5pbzwvc2FtbDI6TmFtZUlEPjxzYW1sMjpTdWJqZWN0Q29uZmlybWF0aW9uIE1ldGhvZD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmNtOmJlYXJlciI+PHNhbWwyOlN1YmplY3RDb25maXJtYXRpb25EYXRhIEluUmVzcG9uc2VUbz0iaWQtZmQ0MTlhNWFiMDQ3MjY0NTQyN2Y4ZTA3ZDg3YTNhNWRkMGIyZTlhNiIgTm90T25PckFmdGVyPSIyMDE2LTAxLTA1VDE3OjAwOjM5LjM0OFoiIFJlY2lwaWVudD0iaHR0cHM6Ly8yOWVlNmQyZS5uZ3Jvay5pby9zYW1sL2FjcyIvPjwvc2FtbDI6U3ViamVjdENvbmZpcm1hdGlvbj48L3NhbWwyOlN1YmplY3Q+PHNhbWwyOkNvbmRpdGlvbnMgTm90QmVmb3JlPSIyMDE2LTAxLTA1VDE2OjUwOjM5LjM0OFoiIE5vdE9uT3JBZnRlcj0iMjAxNi0wMS0wNVQxNzowMDozOS4zNDhaIj48c2FtbDI6QXVkaWVuY2VSZXN0cmljdGlvbj48c2FtbDI6QXVkaWVuY2U+aHR0cHM6Ly8yOWVlNmQyZS5uZ3Jvay5pby9zYW1sL21ldGFkYXRhPC9zYW1sMjpBdWRpZW5jZT48L3NhbWwyOkF1ZGllbmNlUmVzdHJpY3Rpb24+PC9zYW1sMjpDb25kaXRpb25zPjxzYW1sMjpBdHRyaWJ1dGVTdGF0ZW1lbnQ+PHNhbWwyOkF0dHJpYnV0ZSBOYW1lPSJwaG9uZSIvPjxzYW1sMjpBdHRyaWJ1dGUgTmFtZT0iYWRkcmVzcyIvPjxzYW1sMjpBdHRyaWJ1dGUgTmFtZT0iam9iVGl0bGUiLz48c2FtbDI6QXR0cmlidXRlIE5hbWU9ImZpcnN0TmFtZSI+PHNhbWwyOkF0dHJpYnV0ZVZhbHVlIHhtbG5zOnhzPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxL1hNTFNjaGVtYSIgeG1sbnM6eHNpPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxL1hNTFNjaGVtYS1pbnN0YW5jZSIgeHNpOnR5cGU9InhzOmFueVR5cGUiPlJvc3M8L3NhbWwyOkF0dHJpYnV0ZVZhbHVlPjwvc2FtbDI6QXR0cmlidXRlPjxzYW1sMjpBdHRyaWJ1dGUgTmFtZT0ibGFzdE5hbWUiPjxzYW1sMjpBdHRyaWJ1dGVWYWx1ZSB4bWxuczp4cz0iaHR0cDovL3d3dy53My5vcmcvMjAwMS9YTUxTY2hlbWEiIHhtbG5zOnhzaT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS9YTUxTY2hlbWEtaW5zdGFuY2UiIHhzaTp0eXBlPSJ4czphbnlUeXBlIj5LaW5kZXI8L3NhbWwyOkF0dHJpYnV0ZVZhbHVlPjwvc2FtbDI6QXR0cmlidXRlPjwvc2FtbDI6QXR0cmlidXRlU3RhdGVtZW50PjxzYW1sMjpBdXRoblN0YXRlbWVudCBBdXRobkluc3RhbnQ9IjIwMTYtMDEtMDVUMTY6NTU6MzguMDAwWiIgU2Vzc2lvbkluZGV4PSJfOWU3NjQ5NTJlNmEyNjFlMTk0MDlhMzgyNTU4MTAzM2QiPjxzYW1sMjpBdXRobkNvbnRleHQ+PHNhbWwyOkF1dGhuQ29udGV4dENsYXNzUmVmPnVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphYzpjbGFzc2VzOnVuc3BlY2lmaWVkPC9zYW1sMjpBdXRobkNvbnRleHRDbGFzc1JlZj48L3NhbWwyOkF1dGhuQ29udGV4dD48L3NhbWwyOkF1dGhuU3RhdGVtZW50Pjwvc2FtbDI6QXNzZXJ0aW9uPjwvc2FtbDJwOlJlc3BvbnNlPg=="
	test.IDPMetadata = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://accounts.google.com/o/saml2?idpid=C02dfl1r1" validUntil="2021-01-03T16:17:49.000Z">
  <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIIDdDCCAlygAwIBAgIGAVISlIlYMA0GCSqGSIb3DQEBCwUAMHsxFDASBgNVBAoTC0dvb2dsZSBJ
bmMuMRYwFAYDVQQHEw1Nb3VudGFpbiBWaWV3MQ8wDQYDVQQDEwZHb29nbGUxGDAWBgNVBAsTD0dv
b2dsZSBGb3IgV29yazELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWEwHhcNMTYwMTA1
MTYxNzQ5WhcNMjEwMTAzMTYxNzQ5WjB7MRQwEgYDVQQKEwtHb29nbGUgSW5jLjEWMBQGA1UEBxMN
TW91bnRhaW4gVmlldzEPMA0GA1UEAxMGR29vZ2xlMRgwFgYDVQQLEw9Hb29nbGUgRm9yIFdvcmsx
CzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAmUfMUPxHSY/ZYZ88fUGAlhUP4Ni7zj54vsrsPDA4UhQiReEDRunN1q3OHsShRong
gd4LvA83/e/3pm/V60R6vyMfj3Z/IGWY+eZ97EJUvjktt+VRoAi26oeY9ZW6S85yapvA3iuhEwIQ
OcuPm1OqRQ0yQ4sUD+WtL/QSmlYvDP5TK1d6whTisNsKSqeFZCb/s9OX01UexW1BuDOLeVt0rCW1
kRNcBBLDmd4hnDP0SVq7nLhNFYXj2Ea6WsyRAIvchaUGy+Ima2okXm95Ye9kn8e118i/5rReyKCm
BlskMkNaA4KWKvIQm3DdjgONgEd0IvKExyLwY7a5/JIUvBhb9QIDAQABMA0GCSqGSIb3DQEBCwUA
A4IBAQAUDLMnHpzfp4ShdBqCreW48f8rU94q2qMwrU+W6DkOrGJTASVGS9Rib/MKAiRYOmqlaqEY
NP57pCrE/nRB5FVdE+AlSx/fR3khsQ3zf/4dYs21SvGf+Oas99XEbWfV0OmPMYm3IrSCOBEV31wh
41qRc5QLnR+XutNPbSBN+tn+giRCLGCBLe81oVw4fRGQbgkd87rfLOy3G630I6s/J5feFFUT8d7h
9mpOeOqLCPrKpq+wI3aD3lf4mXqKIDNiHHRoNl67ANPu/N3fNU1HplVtvroVpiNp87frgdlKTEcg
PUkfbaYHQGP6IS0lzeCeDX0wab3qRoh7/jJt5/BR8Iwf</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://accounts.google.com/o/saml2/idp?idpid=C02dfl1r1"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://accounts.google.com/o/saml2/idp?idpid=C02dfl1r1"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://29ee6d2e.ngrok.io/saml/metadata",
		AcsURL:      "https://29ee6d2e.ngrok.io/saml/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	req := http.Request{PostForm: url.Values{}}
	req.PostForm.Set("SAMLResponse", SamlResponse)
	assertion, err := s.ParseResponse(&req, []string{"id-fd419a5ab0472645427f8e07d87a3a5dd0b2e9a6"})
	if err != nil {
		c.Logf("%s", err.(*InvalidResponseError).PrivateErr)
	}
	c.Assert(err, IsNil)

	c.Assert(assertion.Subject.NameID.Value, DeepEquals, "ross@octolabs.io")
	c.Assert(assertion.AttributeStatement.Attributes, DeepEquals, []Attribute{
		{
			Name:   "phone",
			Values: nil,
		},
		{
			Name:   "address",
			Values: nil,
		},
		{
			Name:   "jobTitle",
			Values: nil,
		},
		{
			Name: "firstName",
			Values: []AttributeValue{
				{
					Type:  "xs:anyType",
					Value: "Ross",
				},
			},
		},
		{
			Name: "lastName",
			Values: []AttributeValue{
				{
					Type:  "xs:anyType",
					Value: "Kinder",
				},
			},
		},
	})
}

func (test *ServiceProviderTest) TestCanParseResponse(c *C) {
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	req := http.Request{PostForm: url.Values{}}
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	assertion, err := s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err, IsNil)

	c.Assert(assertion.AttributeStatement.Attributes, DeepEquals, []Attribute{
		{
			FriendlyName: "uid",
			Name:         "urn:oid:0.9.2342.19200300.100.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "myself",
				},
			},
		},
		{
			FriendlyName: "eduPersonAffiliation",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Member",
				},
				{
					Type:  "xs:string",
					Value: "Staff",
				},
			},
		},
		{
			FriendlyName: "eduPersonPrincipalName",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.6",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "myself@testshib.org",
				},
			},
		},
		{
			FriendlyName: "sn",
			Name:         "urn:oid:2.5.4.4",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "And I",
				},
			},
		},
		{
			FriendlyName: "eduPersonScopedAffiliation",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.9",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Member@testshib.org",
				},
				{
					Type:  "xs:string",
					Value: "Staff@testshib.org",
				},
			},
		},
		{
			FriendlyName: "givenName",
			Name:         "urn:oid:2.5.4.42",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Me Myself",
				},
			},
		},
		{
			FriendlyName: "eduPersonEntitlement",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.7",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "urn:mace:dir:entitlement:common-lib-terms",
				},
			},
		},
		{
			FriendlyName: "cn",
			Name:         "urn:oid:2.5.4.3",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "Me Myself And I",
				},
			},
		},
		{
			FriendlyName: "eduPersonTargetedID",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.10",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					NameID: &NameID{Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", NameQualifier: "https://idp.testshib.org/idp/shibboleth", SPNameQualifier: "https://15661444.ngrok.io/saml2/metadata", Value: "8F+M9ovyaYNwCId0pVkVsnZYRDo="},
				},
			},
		},
		{
			FriendlyName: "telephoneNumber",
			Name:         "urn:oid:2.5.4.20",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{
				{
					Type:  "xs:string",
					Value: "555-5555",
				},
			},
		},
	})
}

func (test *ServiceProviderTest) TestInvalidResponses(c *C) {
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	req := http.Request{PostForm: url.Values{}}
	req.PostForm.Set("SAMLResponse", "???")
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr, ErrorMatches, "cannot parse base64: illegal base64 data at input byte 0")

	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte("<hello>World!</hello>")))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr, ErrorMatches, "cannot unmarshal response: expected element type <Response> but have <hello>")

	s.AcsURL = "https://wrong/saml2/acs"
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr.Error(), Equals, "`Destination` does not match AcsURL (expected \"https://wrong/saml2/acs\")")
	s.AcsURL = "https://15661444.ngrok.io/saml2/acs"

	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"wrongRequestID"})
	c.Assert(err.(*InvalidResponseError).PrivateErr.Error(), Equals, "`InResponseTo` does not match any of the possible request IDs (expected [wrongRequestID])")

	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 MST 2006", "Mon Nov 30 20:57:09 UTC 2016")
		return rv
	}
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr.Error(), Equals, "IssueInstant expired at 2015-12-01 01:57:51.375 +0000 UTC")
	TimeNow = func() time.Time {
		rv, _ := time.Parse("Mon Jan 2 15:04:05 MST 2006", "Mon Dec 1 01:57:09 UTC 2015")
		return rv
	}

	s.IDPMetadata.EntityID = "http://snakeoil.com"
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr.Error(), Equals, "Issuer does not match the IDP metadata (expected \"http://snakeoil.com\")")
	s.IDPMetadata.EntityID = "https://idp.testshib.org/idp/shibboleth"

	oldSpStatusSuccess := StatusSuccess
	StatusSuccess = "not:the:success:value"
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr.Error(), Equals, "Status code was not not:the:success:value")
	StatusSuccess = oldSpStatusSuccess

	s.Key = "invalid"
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr, ErrorMatches, "failed to decrypt response: .*PEM_read_bio_PrivateKey.*")
	s.Key = test.Key

	s.IDPMetadata.IDPSSODescriptor.KeyDescriptor[0].KeyInfo.Certificate = "invalid"
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	c.Assert(err.(*InvalidResponseError).PrivateErr, ErrorMatches, "failed to verify signature on response: .*xmlSecOpenSSLAppKeyLoadMemory.*")
}

func (test *ServiceProviderTest) TestInvalidAssertions(c *C) {
	s := ServiceProvider{
		Key:         test.Key,
		Certificate: test.Certificate,
		MetadataURL: "https://15661444.ngrok.io/saml2/metadata",
		AcsURL:      "https://15661444.ngrok.io/saml2/acs",
		IDPMetadata: &Metadata{},
	}
	err := xml.Unmarshal([]byte(test.IDPMetadata), &s.IDPMetadata)
	c.Assert(err, IsNil)

	req := http.Request{PostForm: url.Values{}}
	req.PostForm.Set("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(test.SamlResponse)))
	s.IDPMetadata.IDPSSODescriptor.KeyDescriptor[0].KeyInfo.Certificate = "invalid"
	_, err = s.ParseResponse(&req, []string{"id-9e61753d64e928af5a7a341a97f420c9"})
	assertionBuf := []byte(err.(*InvalidResponseError).Response)

	assertion := Assertion{}
	err = xml.Unmarshal(assertionBuf, &assertion)
	c.Assert(err, IsNil)

	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow().Add(time.Hour))
	c.Assert(err.Error(), Equals, "expired on 2015-12-01 01:57:51.375 +0000 UTC")

	assertion.Issuer.Value = "bob"
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "issuer is not \"https://idp.testshib.org/idp/shibboleth\"")
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Subject.NameID.NameQualifier = "bob"
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err, IsNil) // not verified
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Subject.NameID.SPNameQualifier = "bob"
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err, IsNil) // not verified
	xml.Unmarshal(assertionBuf, &assertion)

	err = s.validateAssertion(&assertion, []string{"any request id"}, TimeNow())
	c.Assert(err.Error(), Equals,
		"SubjectConfirmation one of the possible request IDs ([any request id])")

	assertion.Subject.SubjectConfirmation.SubjectConfirmationData.Recipient = "wrong/acs/url"
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "SubjectConfirmation Recipient is not https://15661444.ngrok.io/saml2/acs")
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Subject.SubjectConfirmation.SubjectConfirmationData.NotOnOrAfter = TimeNow().Add(-1 * time.Hour)
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "SubjectConfirmationData is expired")
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Conditions.NotBefore = TimeNow().Add(time.Hour)
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "Conditions is not yet valid")
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Conditions.NotOnOrAfter = TimeNow().Add(-1 * time.Hour)
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "Conditions is expired")
	xml.Unmarshal(assertionBuf, &assertion)

	assertion.Conditions.AudienceRestriction.Audience.Value = "not/our/metadata/url"
	err = s.validateAssertion(&assertion, []string{"id-9e61753d64e928af5a7a341a97f420c9"}, TimeNow())
	c.Assert(err.Error(), Equals, "Conditions AudienceRestriction is not \"https://15661444.ngrok.io/saml2/metadata\"")
	xml.Unmarshal(assertionBuf, &assertion)
}
