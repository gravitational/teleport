/*
Copyright 2017 Gravitational, Inc.
This file implements the enterprise version of `tctl create` or `tctl rm` subcommands

*/

package main

import (
	"fmt"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"
	"github.com/gravitational/trace"
)

// implements common.CLICommand interface
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
	cmd.base.CreateHandlers[common.ResourceKind(services.KindRole)] = cmd.createRole
	cmd.base.CreateHandlers[common.ResourceKind(services.KindOIDCConnector)] = cmd.createConnector
	cmd.base.CreateHandlers[common.ResourceKind(services.KindSAMLConnector)] = cmd.createConnector
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *ResourceCommandE) TryRun(selectedCommand string, c *auth.TunClient) (match bool, err error) {
	ref := cmd.base.GetRef()

	// implement 'tctl rm role/xxx' (OSS lacks this)
	if cmd.base.IsDeleteSubcommand(selectedCommand) && ref.Kind == services.KindRole {
		if err := c.DeleteRole(ref.Name); err != nil {
			return true, trace.Wrap(err)
		}
		fmt.Printf("role %s has been deleted\n", ref.Name)
	}

	// call the OSS implementation:
	return cmd.base.TryRun(selectedCommand, c)
}

// createConnector implements 'tctl create role.yaml' command
func (cmd *ResourceCommandE) createRole(client *auth.TunClient, raw services.UnknownResource) error {
	role, err := services.GetRoleMarshaler().UnmarshalRole(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	err = role.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.UpsertRole(role, backend.Forever); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("created role: %v\n", role.GetName())
	return nil
}

// createConnector implements 'tctl create connector.yaml' command
func (cmd *ResourceCommandE) createConnector(client *auth.TunClient, raw services.UnknownResource) error {
	var (
		connectorName string
		exists        bool
		err           error
	)
	switch raw.Kind {

	// SAML
	case services.KindSAMLConnector:
		conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := conn.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		connectorName = conn.GetName()
		_, err = client.GetSAMLConnector(connectorName, false)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		exists = (err == nil)
		if cmd.base.IsForced() == false && exists {
			return trace.AlreadyExists("connector '%s' already exists", connectorName)
		}
		err = client.UpsertSAMLConnector(conn)

		// OpenID connect
	case services.KindOIDCConnector:
		conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := conn.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		connectorName = conn.GetName()
		_, err = client.GetOIDCConnector(connectorName, false)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		exists = (err == nil)
		if cmd.base.IsForced() == false && exists {
			return trace.AlreadyExists("connector '%s' already exists", connectorName)
		}
		err = client.UpsertOIDCConnector(conn)

		// unknown connector type
	default:
		err = trace.BadParameter("unknown connector type: '%s'", raw.Kind)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector '%s' has been %s\n", connectorName, common.UpsertVerb(exists))
	return nil
}
