package modules

import (
	"github.com/gravitational/trace"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	etypes "github.com/gravitational/teleport/e/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// register marshaler/unmarshaler pairs for enterprise-only resources.
func init() {
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
	services.RegisterResourceUnmarshaler(types.KindInferenceModel, func(b []byte, options ...services.MarshalOption) (types.Resource, error) {
		model, err := services.UnmarshalProtoResource[*summarizerv1.InferenceModel](b, options...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(model), nil
	})
	services.RegisterResourceUnmarshaler(types.KindInferencePolicy, func(b []byte, options ...services.MarshalOption) (types.Resource, error) {
		policy, err := services.UnmarshalProtoResource[*summarizerv1.InferencePolicy](b, options...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(policy), nil
	})
	services.RegisterResourceUnmarshaler(types.KindInferenceSecret, func(b []byte, options ...services.MarshalOption) (types.Resource, error) {
		secret, err := services.UnmarshalProtoResource[*summarizerv1.InferenceSecret](b, options...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(secret), nil
	})
	services.RegisterResourceUnmarshaler(types.KindRetrievalModel, func(b []byte, options ...services.MarshalOption) (types.Resource, error) {
		model, err := services.UnmarshalProtoResource[*summarizerv1.RetrievalModel](b, options...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(model), nil
	})
	// Register functions to create enterprise GitHub auth connectors.
	services.RegisterGithubAuthCreator(etypes.NewGithubConnectorE)
	// Register function to convert OSS GitHub auth connectors to
	// enterprise connectors so endpoint_url will be respected.
	services.RegisterGithubAuthInitializer(func(c types.GithubConnector) (types.GithubConnector, error) {
		switch connector := c.(type) {
		case *etypes.GithubConnector:
			return connector, nil
		case *types.GithubConnectorV3:
			return &etypes.GithubConnector{
				GithubConnectorV3: connector,
			}, nil
		default:
			return nil, trace.BadParameter("unrecognized github connector version %T", c)
		}
	})
	// Register function to convert enterprise GitHub auth connectors to
	// OSS connectors so they can be sent over gRPC.
	services.RegisterGithubAuthConverter(func(c types.GithubConnector) (*types.GithubConnectorV3, error) {
		switch connector := c.(type) {
		case *etypes.GithubConnector:
			return connector.GithubConnectorV3, nil
		case *types.GithubConnectorV3:
			return connector, nil
		default:
			return nil, trace.BadParameter("unrecognized github connector version %T", c)
		}
	})
	// Register marshaler for enterprise GitHub auth connector.
	services.RegisterResourceMarshaler(types.KindGithubConnector, func(resource types.Resource, opts ...services.MarshalOption) ([]byte, error) {
		githubConnector, ok := resource.(types.GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", resource)
		}
		bytes, err := etypes.MarshalGithubConnector(githubConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	// Register unmarshaler for enterprise GitHub auth connector.
	services.RegisterResourceUnmarshaler(types.KindGithubConnector, func(bytes []byte, opts ...services.MarshalOption) (types.Resource, error) {
		githubConnector, err := etypes.UnmarshalGithubConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return githubConnector, nil
	})

	// Register marshaler for Trusted Devices.
	services.RegisterResourceMarshaler(types.KindDevice, func(resource types.Resource, opts ...services.MarshalOption) ([]byte, error) {
		device, ok := resource.(*types.DeviceV1)
		if !ok {
			return nil, trace.BadParameter("expected Device, got %T", resource)
		}
		bytes, err := services.MarshalDevice(device)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})

	// Register unmarshaler for Trusted Devices.
	services.RegisterResourceUnmarshaler(types.KindDevice, func(bytes []byte, opts ...services.MarshalOption) (types.Resource, error) {
		device, err := services.UnmarshalDevice(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return device, nil
	})
}
