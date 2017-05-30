package saml2

import (
	"sync"
	"time"

	dsig "github.com/russellhaering/goxmldsig"
)

type SAMLServiceProvider struct {
	IdentityProviderSSOURL string
	IdentityProviderIssuer string

	AssertionConsumerServiceURL string
	ServiceProviderIssuer       string

	SignAuthnRequests              bool
	SignAuthnRequestsAlgorithm     string
	SignAuthnRequestsCanonicalizer dsig.Canonicalizer

	// RequestedAuthnContext allows service providers to require that the identity
	// provider use specific authentication mechanisms. Leaving this unset will
	// permit the identity provider to choose the auth method. To maximize compatibility
	// with identity providers it is recommended to leave this unset.
	RequestedAuthnContext   *RequestedAuthnContext
	AudienceURI             string
	IDPCertificateStore     dsig.X509CertificateStore
	SPKeyStore              dsig.X509KeyStore
	NameIdFormat            string
	SkipSignatureValidation bool
	AllowMissingAttributes  bool
	Clock                   *dsig.Clock
	signingContextMu        sync.RWMutex
	signingContext          *dsig.SigningContext
}

// RequestedAuthnContext controls which authentication mechanisms are requested of
// the identity provider. It is generally sufficient to omit this and let the
// identity provider select an authentication mechansim.
type RequestedAuthnContext struct {
	// The RequestedAuthnContext comparison policy to use. See the section 3.3.2.2.1
	// of the SAML 2.0 specification for details. Constants named AuthnPolicyMatch*
	// contain standardized values.
	Comparison string

	// Contexts will be passed as AuthnContextClassRefs. For example, to force password
	// authentication on some identity providers, Contexts should have a value of
	// []string{AuthnContextPasswordProtectedTransport}, and Comparison should have a
	// value of AuthnPolicyMatchExact.
	Contexts []string
}

func (sp *SAMLServiceProvider) SigningContext() *dsig.SigningContext {
	sp.signingContextMu.RLock()
	signingContext := sp.signingContext
	sp.signingContextMu.RUnlock()

	if signingContext != nil {
		return signingContext
	}

	sp.signingContextMu.Lock()
	defer sp.signingContextMu.Unlock()

	sp.signingContext = dsig.NewDefaultSigningContext(sp.SPKeyStore)
	sp.signingContext.SetSignatureMethod(sp.SignAuthnRequestsAlgorithm)
	if sp.SignAuthnRequestsCanonicalizer != nil {
		sp.signingContext.Canonicalizer = sp.SignAuthnRequestsCanonicalizer
	}

	return sp.signingContext
}

type ProxyRestriction struct {
	Count    int
	Audience []string
}

type WarningInfo struct {
	OneTimeUse       bool
	ProxyRestriction *ProxyRestriction
	NotInAudience    bool
	InvalidTime      bool
}

type AssertionInfo struct {
	NameID              string
	Values              Values
	WarningInfo         *WarningInfo
	AuthnInstant        *time.Time
	SessionNotOnOrAfter *time.Time
}
