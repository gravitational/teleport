// Package saml contains a partial implementation of the SAML standard in golang.
// SAML is a standard for identity federation, i.e. either allowing a third party to authenticate your users or allowing third parties to rely on us to authenticate their users.
//
// In SAML parlance an Identity Provider (IDP) is a service that knows how to authenticate users. A Service Provider (SP) is a service that delegates authentication to an IDP. If you are building a service where users log in with someone else's credentials, then you are a Service Provider. This package supports implementing both service providers and identity providers.
//
// The core package contains the implementation of SAML. The package samlsp provides helper middleware suitable for use in Service Provider applications. The package samlidp provides a rudimentary IDP service that is useful for testing or as a starting point for other integrations.
//
// Getting Started as a Service Provider
//
// Let us assume we have a simple web appliation to protect. We'll modify this application so it uses SAML to authenticate users.
//
//     package main
//
//     import "net/http"
//
//      func hello(w http.ResponseWriter, r *http.Request) {
//         fmt.Fprintf(w, "Hello, World!")
//     })
//
//     func main() {
//         app := http.HandlerFunc(hello)
//         http.Handle("/hello", app)
//         http.ListenAndServe(":8000", nil)
//     }
//
// Each service provider must have an self-signed X.509 key pair established. You can generate your own with something like this:
//
//     openssl req -x509 -newkey rsa:2048 -keyout myservice.key -out myservice.cert -days 365 -nodes -subj "/CN=myservice.example.com"
//
// We will use `samlsp.Middleware` to wrap the endpoint we want to protect. Middleware provides both an `http.Handler` to serve the SAML specific URLs and a set of wrappers to require the user to be logged in. We also provide the URL where the service provider can fetch the metadata from the IDP at startup. In our case, we'll use [testshib.org](testshib.org), an identity provider designed for testing.
//
//     package main
//
//     import (
//         "fmt"
//         "io/ioutil"
//         "net/http"
//
//         "github.com/crewjam/saml/samlsp"
//     )
//
//     func hello(w http.ResponseWriter, r *http.Request) {
//         fmt.Fprintf(w, "Hello, %s!", r.Header.Get("X-Saml-Cn"))
//     }
//
//     func main() {
//         key, _ := ioutil.ReadFile("myservice.key")
//         cert, _ := ioutil.ReadFile("myservice.cert")
//         samlSP, _ := samlsp.New(samlsp.Options{
//             IDPMetadataURL: "https://www.testshib.org/metadata/testshib-providers.xml",
//             URL:            "http://localhost:8000",
//             Key:            string(key),
//             Certificate:    string(cert),
//         })
//         app := http.HandlerFunc(hello)
//         http.Handle("/hello", samlSP.RequireAccount(app))
//         http.Handle("/saml/", samlSP)
//         http.ListenAndServe(":8000", nil)
//     }
//
//
// Next we'll have to register our service provider with the identiy provider to establish trust from the service provider to the IDP. For [testshib.org](testshib.org), you can do something like:
//
//     mdpath=saml-test-$USER-$HOST.xml
//     curl localhost:8000/saml/metadata > $mdpath
//     curl -i -F userfile=@$mdpath https://www.testshib.org/procupload.php
//
// Now you should be able to authenticate. The flow should look like this:
//
// 1. You browse to `localhost:8000/hello`
//
// 2. The middleware redirects you to `https://idp.testshib.org/idp/profile/SAML2/Redirect/SSO`
//
// 3. testshib.org prompts you for a username and password.
//
// 4. testshib.org returns you an HTML document which contains an HTML form setup to POST to `localhost:8000/saml/acs`. The form is automatically submitted if you have javascript enabled.
//
// 5. The local service validates the response, issues a session cookie, and redirects you to the original URL, `localhost:8000/hello`.
//
// 6. This time when `localhost:8000/hello` is requested there is a valid session and so the main content is served.
//
// Getting Started as an Identity Provider
//
// Please see `examples/idp/` for a substantially complete example of how to use the library and helpers to be an identity provider.
//
// Support
//
// The SAML standard is huge and complex with many dark corners and strange, unused features. This package implements the most commonly used subset of these features required to provide a single sign on experience. The package supports at least the subset of SAML known as [interoperable SAML](http://saml2int.org).
//
// This package supports the Web SSO profile. Message flows from the service provider to the IDP are supported using the HTTP Redirect binding and the HTTP POST binding. Message flows fromthe IDP to the service provider are supported vai the HTTP POST binding.
//
// The package supports signed and encrypted SAML assertions. It does not support signed or encrypted requests.
//
// RelayState
//
// The *RelayState* parameter allows you to pass user state information across the authentication flow. The most common use for this is to allow a user to request a deep link into your site, be redirected through the SAML login flow, and upon successful completion, be directed to the originally requested link, rather than the root.
//
// Unfortunately, *RelayState* is less useful than it could be. Firstly, it is not authenticated, so anything you supply must be signed to avoid XSS or CSRF. Secondly, it is limited to 80 bytes in length, which precludes signing. (See section 3.6.3.1 of SAMLProfiles.)
//
// References
//
// The SAML specification is a collection of PDFs (sadly):
//
// - [SAMLCore](http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf) defines data types.
//
// - [SAMLBindings](http://docs.oasis-open.org/security/saml/v2.0/saml-bindings-2.0-os.pdf) defines the details of the HTTP requests in play.
//
// - [SAMLProfiles](http://docs.oasis-open.org/security/saml/v2.0/saml-profiles-2.0-os.pdf) describes data flows.
//
// - [SAMLConformance](http://docs.oasis-open.org/security/saml/v2.0/saml-conformance-2.0-os.pdf) includes a support matrix for various parts of the protocol.
//
// [TestShib](http://www.testshib.org/) is a testing ground for SAML service and identity providers.
package saml
