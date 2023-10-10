package typestest

import (
	fmt "fmt"
	"net/url"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// setStaticFields sets static resource header and metadata fields.
func (a *AppV3) setStaticFields() {
	a.Kind = KindApp
	a.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (a *AppV3) CheckAndSetDefaults() error {
	a.setStaticFields()
	if err := a.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for key := range a.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("app %q invalid label key: %q", a.GetName(), key)
		}
	}
	if a.Spec.URI == "" {
		if a.Spec.Cloud != "" {
			a.Spec.URI = fmt.Sprintf("cloud://%v", a.Spec.Cloud)
		} else {
			return trace.BadParameter("app %q URI is empty", a.GetName())
		}
	}
	if a.Spec.Cloud == "" && a.IsAWSConsole() {
		a.Spec.Cloud = CloudAWS
	}
	switch a.Spec.Cloud {
	case "", CloudAWS, CloudAzure, CloudGCP:
		break
	default:
		return trace.BadParameter("app %q has unexpected Cloud value %q", a.GetName(), a.Spec.Cloud)
	}
	url, err := url.Parse(a.Spec.PublicAddr)
	if err != nil {
		return trace.BadParameter("invalid PublicAddr format: %v", err)
	}
	host := a.Spec.PublicAddr
	if url.Host != "" {
		host = url.Host
	}

	if strings.HasPrefix(host, constants.KubeTeleportProxyALPNPrefix) {
		return trace.BadParameter("app %q DNS prefix found in %q public_url is reserved for internal usage",
			constants.KubeTeleportProxyALPNPrefix, a.Spec.PublicAddr)
	}

	if a.Spec.Rewrite != nil {
		switch a.Spec.Rewrite.JWTClaims {
		case "", JWTClaimsRewriteRolesAndTraits, JWTClaimsRewriteRoles, JWTClaimsRewriteNone:
		default:
			return trace.BadParameter("app %q has unexpected JWT rewrite value %q", a.GetName(), a.Spec.Rewrite.JWTClaims)

		}
	}

	return nil
}
