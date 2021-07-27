package modules

import (
	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// register marshaler/unmarshaler pairs for enterprise-only resources.
func init() {
	// Register marshaler for role resources.
	services.RegisterResourceMarshaler(types.KindRole, func(r types.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(types.Role)
		if !ok {
			return nil, trace.BadParameter("expected Role, got %T", r)
		}
		raw, err := services.MarshalRole(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for role resources.
	services.RegisterResourceUnmarshaler(types.KindRole, func(b []byte, opts ...services.MarshalOption) (types.Resource, error) {
		rsc, err := services.UnmarshalRole(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	// Register marshaler for oidc connector resources.
	services.RegisterResourceMarshaler(types.KindOIDCConnector, func(r types.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(types.OIDCConnector)
		if !ok {
			return nil, trace.BadParameter("expected OIDCConnector, got %T", r)
		}
		raw, err := services.MarshalOIDCConnector(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for oidc connector resources.
	services.RegisterResourceUnmarshaler(types.KindOIDCConnector, func(b []byte, opts ...services.MarshalOption) (types.Resource, error) {
		rsc, err := services.UnmarshalOIDCConnector(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	// Register marshaler for saml connector resources.
	services.RegisterResourceMarshaler(types.KindSAMLConnector, func(r types.Resource, opts ...services.MarshalOption) ([]byte, error) {
		rsc, ok := r.(types.SAMLConnector)
		if !ok {
			return nil, trace.BadParameter("expected SAMLConnector, got %T", r)
		}
		raw, err := services.MarshalSAMLConnector(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	// Register unmarshaler for saml connector resources.
	services.RegisterResourceUnmarshaler(types.KindSAMLConnector, func(b []byte, opts ...services.MarshalOption) (types.Resource, error) {
		rsc, err := services.UnmarshalSAMLConnector(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})
}
