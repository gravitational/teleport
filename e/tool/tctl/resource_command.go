/*
Copyright 2017 Gravitational, Inc.
This file implements the enterprise version of `tctl create` or `tctl rm` subcommands

*/

package main

import (
	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/tool/tctl/common"
)

// implements common.CLICommand interface
type ResourceCommandE struct {
	// OSS implementation of 'tctl users'
	common.ResourceCommand
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *ResourceCommandE) Initialize(app *kingpin.Application, cfg *service.Config) {
	// nothing to do here... enterprise version uses the same flags
	cmd.ResourceCommand.Initialize(app, cfg)
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *ResourceCommandE) TryRun(selectedCommand string, c *auth.TunClient) (match bool, err error) {
	return false, nil
}

/*
func deleteResource(ref services.Ref, client *auth.TunClient) error {
	switch ref.Kind {
	case services.KindRole:
		if err := client.DeleteRole(ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("role %s has been deleted\n", ref.Name)
	}
	return nil
}
func createResource(raw *services.UnknownResource, client *auth.TunClient) error {
	switch raw.Kind {
	case services.KindRole:
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
	default:
		return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
	}
	return nil
}
*/
