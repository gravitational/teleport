package resource

import (
	"context"
	"fmt"
	types "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	var saml, oidc, github collections.ResourceCollection

	sc, scErr := rc.getSAMLConnectors(ctx, client)
	if scErr == nil {
		saml = sc
	}
	oc, ocErr := rc.getOIDCConnectors(ctx, client)
	if ocErr == nil {
		oidc = oc
	}
	gc, gcErr := rc.getGithubConnectors(ctx, client)
	if gcErr == nil {
		github = gc
	}
	errs := []error{scErr, ocErr, gcErr}

	connectors, err := collections.NewConnectorsCollection(oidc, saml, github)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	allEmpty := len(connectors.Resources()) == 0
	reportErr := false
	for _, err := range errs {
		if err != nil && !trace.IsNotFound(err) {
			reportErr = true
			break
		}
	}
	var finalErr error
	if allEmpty || reportErr {
		finalErr = trace.NewAggregate(errs...)
	}
	return connectors, finalErr
}

func (rc *ResourceCommand) createOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	conn, err := services.UnmarshalOIDCConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.force {
		upserted, err := client.UpsertOIDCConnector(ctx, conn)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("authentication connector %q has been updated\n", upserted.GetName())
		return nil
	}

	created, err := client.CreateOIDCConnector(ctx, conn)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("connector %q already exists, use -f flag to override", conn.GetName())
		}

		return trace.Wrap(err)
	}

	fmt.Printf("authentication connector %q has been created\n", created.GetName())
	return nil
}

// updateGithubConnector updates an existing OIDC connector.
func (rc *ResourceCommand) updateOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	connector, err := services.UnmarshalOIDCConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateOIDCConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", connector.GetName())
	return nil
}

func (rc *ResourceCommand) deleteOIDCConnector(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteOIDCConnector(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("OIDC connector %v has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) createSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	// Create services.SAMLConnector from raw YAML to extract the connector name.
	conn, err := services.UnmarshalSAMLConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	connectorName := conn.GetName()
	foundConn, err := client.GetSAMLConnector(ctx, connectorName, true)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)
	if !rc.IsForced() && exists {
		return trace.AlreadyExists("connector %q already exists, use -f flag to override", connectorName)
	}

	// If the connector being pushed to the backend does not have a signing key
	// in it and an existing connector was found in the backend, extract the
	// signing key from the found connector and inject it into the connector
	// being injected into the backend.
	if conn.GetSigningKeyPair() == nil && exists {
		conn.SetSigningKeyPair(foundConn.GetSigningKeyPair())
	}

	if _, err = client.UpsertSAMLConnector(ctx, conn); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been %s\n", connectorName, UpsertVerb(exists, rc.IsForced()))
	return nil
}

func (rc *ResourceCommand) updateSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	// Create services.SAMLConnector from raw YAML to extract the connector name.
	conn, err := services.UnmarshalSAMLConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.UpdateSAMLConnector(ctx, conn); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", conn.GetName())
	return nil
}

func (rc *ResourceCommand) deleteSAMLConnector(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteSAMLConnector(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML connector %v has been deleted\n", rc.ref.Name)
	return nil
}

// createGithubConnector creates a Github connector
func (rc *ResourceCommand) createGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	connector, err := services.UnmarshalGithubConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.force {
		upserted, err := client.UpsertGithubConnector(ctx, connector)
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("authentication connector %q has been updated\n", upserted.GetName())
		return nil
	}

	created, err := client.CreateGithubConnector(ctx, connector)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("authentication connector %q already exists", connector.GetName())
		}
		return trace.Wrap(err)
	}

	fmt.Printf("authentication connector %q has been created\n", created.GetName())

	return nil
}

// updateGithubConnector updates an existing Github connector.
func (rc *ResourceCommand) updateGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	connector, err := services.UnmarshalGithubConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateGithubConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", connector.GetName())
	return nil
}

func (rc *ResourceCommand) deleteGithubConnector(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteGithubConnector(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("github connector %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) getSAMLConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetSAMLConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSAMLCollection(connectors), nil
	}
	connector, err := client.GetSAMLConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSAMLCollection([]types.SAMLConnector{connector}), nil
}

func (rc *ResourceCommand) getOIDCConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetOIDCConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewOIDCCollection(connectors), nil
	}
	connector, err := client.GetOIDCConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewOIDCCollection([]types.OIDCConnector{connector}), nil
}

func (rc *ResourceCommand) getGithubConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetGithubConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewGithubCollection(connectors), nil
	}
	connector, err := client.GetGithubConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewGithubCollection([]types.GithubConnector{connector}), nil
}
