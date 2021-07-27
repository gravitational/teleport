/*
Copyright 2017-2021 Gravitational, Inc.
This file implements the enterprise version of `tctl create` or `tctl rm` subcommands

*/

package main

import (
	"context"
	"fmt"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"
	"github.com/gravitational/trace"
)

// ResourceCommandE implements common.CLICommand interface
type ResourceCommandE struct {
	// OSS implementation of 'tctl users'
	base common.ResourceCommand
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *ResourceCommandE) Initialize(app *kingpin.Application, cfg *service.Config) {
	// nothing to do here... enterprise version uses the same flags
	cmd.base.Initialize(app, cfg)

	// plug our enterprise resource creators:
	cmd.base.CreateHandlers[common.ResourceKind(types.KindOIDCConnector)] = cmd.createConnector
	cmd.base.CreateHandlers[common.ResourceKind(types.KindSAMLConnector)] = cmd.createConnector
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *ResourceCommandE) TryRun(selectedCommand string, c auth.ClientI) (match bool, err error) {
	return cmd.base.TryRun(selectedCommand, c)
}

// createConnector implements 'tctl create connector.yaml' command
func (cmd *ResourceCommandE) createConnector(client auth.ClientI, raw services.UnknownResource) error {
	var (
		connectorName string
		exists        bool
		ctx           = context.TODO()
	)
	switch raw.Kind {

	// SAML
	case types.KindSAMLConnector:
		// Create services.SAMLConnector from raw YAML to extract the connector name.
		conn, err := services.UnmarshalSAMLConnector(raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		connectorName = conn.GetName()

		// Check if this connector is already in the backend. If it is, and the force
		// flag was not supplied, return an "connector already exists" error.
		foundConn, err := client.GetSAMLConnector(ctx, connectorName, true)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		exists = (err == nil)
		if cmd.base.IsForced() == false && exists {
			return trace.AlreadyExists("connector '%s' already exists, use -f flag to override", connectorName)
		}

		// If the connector being pushed to the backend does not have a signing key
		// in it and an existing connector was found in the backend, extract the
		// signing key from the found connector and inject it into the connector
		// being injected into the backend.
		if conn.GetSigningKeyPair() == nil && exists {
			conn.SetSigningKeyPair(foundConn.GetSigningKeyPair())
		}
		if err := conn.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		if err = client.UpsertSAMLConnector(ctx, conn); err != nil {
			return trace.Wrap(err)
		}

	// OpenID connect
	case types.KindOIDCConnector:
		conn, err := services.UnmarshalOIDCConnector(raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := conn.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		connectorName = conn.GetName()
		_, err = client.GetOIDCConnector(ctx, connectorName, false)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		exists = (err == nil)
		if cmd.base.IsForced() == false && exists {
			return trace.AlreadyExists("connector '%s' already exists, use -f flag to override", connectorName)
		}
		if err = client.UpsertOIDCConnector(ctx, conn); err != nil {
			return trace.Wrap(err)
		}

	// unknown connector type
	default:
		return trace.BadParameter("unknown connector type: '%s'", raw.Kind)
	}

	fmt.Printf("authentication connector '%s' has been %s\n", connectorName, common.UpsertVerb(exists, cmd.base.IsForced()))
	return nil
}
