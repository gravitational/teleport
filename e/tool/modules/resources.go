package modules

import (
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// register marshaler/unmarshaler pairs for enterprise-only resources.
func init() {
	// Register marshaler for role resources.
	services.RegisterResourceMarshaler(services.KindRole, func(r services.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(services.Role)
		if !ok {
			return nil, trace.BadParameter("expected Role, got %T", r)
		}
		raw, err := services.GetRoleMarshaler().MarshalRole(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for role resources.
	services.RegisterResourceUnmarshaler(services.KindRole, func(b []byte, opts ...services.MarshalOption) (services.Resource, error) {
		rsc, err := services.GetRoleMarshaler().UnmarshalRole(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	// Register marshaler for oidc connector resources.
	services.RegisterResourceMarshaler(services.KindOIDCConnector, func(r services.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(services.OIDCConnector)
		if !ok {
			return nil, trace.BadParameter("expected OIDCConnector, got %T", r)
		}
		raw, err := services.GetOIDCConnectorMarshaler().MarshalOIDCConnector(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for oidc connector resources.
	services.RegisterResourceUnmarshaler(services.KindOIDCConnector, func(b []byte, opts ...services.MarshalOption) (services.Resource, error) {
		rsc, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	// Register marshaler for saml connector resources.
	services.RegisterResourceMarshaler(services.KindSAMLConnector, func(r services.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(services.SAMLConnector)
		if !ok {
			return nil, trace.BadParameter("expected SAMLConnector, got %T", r)
		}
		raw, err := services.GetSAMLConnectorMarshaler().MarshalSAMLConnector(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for saml connector resources.
	services.RegisterResourceUnmarshaler(services.KindSAMLConnector, func(b []byte, opts ...services.MarshalOption) (services.Resource, error) {
		rsc, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})
}
