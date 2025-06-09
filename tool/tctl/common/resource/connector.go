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

var connector = resource{
	getHandler: getConnectors,
}

func getConnectors(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	var saml, oidc, github collections.ResourceCollection

	sc, scErr := getSAMLConnectors(ctx, client, ref, opts)
	if scErr == nil {
		saml = sc
	}
	oc, ocErr := getOIDCConnector(ctx, client, ref, opts)
	if ocErr == nil {
		oidc = oc
	}
	gc, gcErr := getGithubConnectors(ctx, client, ref, opts)
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

var oidcConnector = resource{
	getHandler:    getOIDCConnector,
	createHandler: createOIDCConnector,
	updateHandler: updateOIDCConnector,
	deleteHandler: deleteOIDCConnector,
}

func getOIDCConnector(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		connectors, err := client.GetOIDCConnectors(ctx, opts.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewOIDCCollection(connectors), nil
	}
	connector, err := client.GetOIDCConnector(ctx, ref.Name, opts.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewOIDCCollection([]types.OIDCConnector{connector}), nil
}

func createOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	conn, err := services.UnmarshalOIDCConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.force {
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
func updateOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteOIDCConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteOIDCConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("OIDC connector %v has been deleted\n", ref.Name)
	return nil
}

var samlConnector = resource{
	getHandler:    getSAMLConnectors,
	createHandler: createSAMLConnector,
	updateHandler: updateSAMLConnector,
	deleteHandler: deleteSAMLConnector,
}

func getSAMLConnectors(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		connectors, err := client.GetSAMLConnectors(ctx, opts.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSAMLCollection(connectors), nil
	}
	connector, err := client.GetSAMLConnector(ctx, ref.Name, opts.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSAMLCollection([]types.SAMLConnector{connector}), nil
}

func createSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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
	if !opts.force && exists {
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
	fmt.Printf("authentication connector %q has been %s\n", connectorName, UpsertVerb(exists, opts.force))
	return nil
}

func updateSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteSAMLConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteSAMLConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML connector %v has been deleted\n", ref.Name)
	return nil
}

var githubConnector = resource{
	getHandler:    getGithubConnectors,
	createHandler: createGithubConnector,
	updateHandler: updateGithubConnector,
	deleteHandler: deleteGithubConnector,
}

func getGithubConnectors(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		connectors, err := client.GetGithubConnectors(ctx, opts.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewGithubCollection(connectors), nil
	}
	connector, err := client.GetGithubConnector(ctx, ref.Name, opts.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewGithubCollection([]types.GithubConnector{connector}), nil
}

// createGithubConnector creates a Github connector
func createGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	connector, err := services.UnmarshalGithubConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.force {
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
func updateGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteGithubConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteGithubConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("github connector %q has been deleted\n", ref.Name)
	return nil
}
