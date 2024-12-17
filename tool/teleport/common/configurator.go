/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/configurators"
	awsconfigurators "github.com/gravitational/teleport/lib/configurators/aws"
	"github.com/gravitational/teleport/lib/configurators/configuratorbuilder"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// awsDatabaseTypes list of databases supported on the configurator.
var awsDatabaseTypes = []string{
	types.DatabaseTypeRDS,
	types.DatabaseTypeRDSProxy,
	types.DatabaseTypeRedshift,
	types.DatabaseTypeRedshiftServerless,
	types.DatabaseTypeElastiCache,
	types.DatabaseTypeMemoryDB,
	types.DatabaseTypeAWSKeyspaces,
	types.DatabaseTypeDynamoDB,
	types.DatabaseTypeOpenSearch,
	types.DatabaseTypeDocumentDB,
}

type installSystemdFlags struct {
	config.SystemdFlags
	// output is the destination to write the systemd unit file to.
	output string
}

type createDatabaseConfigFlags struct {
	config.DatabaseSampleFlags
	// output is the destination to write the configuration to.
	output string
}

// CheckAndSetDefaults checks and sets the defaults
func (flags *installSystemdFlags) CheckAndSetDefaults() error {
	flags.output = normalizeOutput(flags.output)
	return nil
}

// onDumpSystemdUnitFile is the handler of the "install systemd" CLI command.
func onDumpSystemdUnitFile(flags installSystemdFlags) error {
	if err := flags.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	buf := new(bytes.Buffer)
	err := config.WriteSystemdUnitFile(flags.SystemdFlags, buf)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = dumpConfigFile(flags.output, buf.String(), "")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CheckAndSetDefaults checks and sets the defaults
func (flags *createDatabaseConfigFlags) CheckAndSetDefaults() error {
	flags.output = normalizeOutput(flags.output)
	return nil
}

// onDumpDatabaseConfig is the handler of "db configure create" CLI command.
func onDumpDatabaseConfig(flags createDatabaseConfigFlags) error {
	if err := flags.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	sfc, err := config.MakeDatabaseAgentConfigString(flags.DatabaseSampleFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	configPath, err := dumpConfigFile(flags.output, sfc, "")
	if err != nil {
		return trace.Wrap(err)
	}

	if configPath != "" {
		fmt.Printf("Wrote config to file %q. Now you can start the server. Happy Teleporting!\n", configPath)
	}
	return nil
}

// configureDiscoveryBootstrapFlags database configure bootstrap flags.
type configureDiscoveryBootstrapFlags struct {
	config  configurators.BootstrapFlags
	confirm bool

	databaseServiceRole       string
	databaseServicePolicyName string
}

func makeDatabaseServiceBootstrapFlagsWithDiscoveryServiceConfig(flags configureDiscoveryBootstrapFlags) configurators.BootstrapFlags {
	config := flags.config
	config.Service = configurators.DatabaseServiceByDiscoveryServiceConfig
	config.AttachToUser = ""
	config.AttachToRole = flags.databaseServiceRole
	config.PolicyName = flags.databaseServicePolicyName
	return config
}

// onConfigureDiscoveryBootstrap subcommand that bootstraps configuration for
// discovery  agents.
func onConfigureDiscoveryBootstrap(flags configureDiscoveryBootstrapFlags) error {
	fmt.Printf("Reading configuration at %q...\n", flags.config.ConfigPath)

	ctx := context.TODO()
	configurators, err := configuratorbuilder.BuildConfigurators(flags.config)
	if err != nil {
		return trace.Wrap(err)
	}

	// If database service role is specified while bootstrap discovery service,
	// generate configurator actions for database service using the discovery
	// service config.
	if flags.config.Service.IsDiscovery() && flags.databaseServiceRole != "" {
		config := makeDatabaseServiceBootstrapFlagsWithDiscoveryServiceConfig(flags)
		dbConfigurators, err := configuratorbuilder.BuildConfigurators(config)
		if err != nil {
			return trace.Wrap(err)
		}
		configurators = append(configurators, dbConfigurators...)
	}

	if len(configurators) == 0 {
		fmt.Println("The agent doesn't require any extra configuration.")
		return nil
	}

	for _, configurator := range configurators {
		fmt.Println()
		fmt.Println(configurator.Description())
		printDiscoveryConfiguratorActions(configurator.Actions())
	}

	if flags.config.Manual {
		return nil
	}

	fmt.Print("\n")
	if !flags.confirm {
		confirmed, err := prompt.Confirmation(ctx, os.Stdout, prompt.Stdin(), "Confirm?")
		if err != nil {
			return trace.Wrap(err)
		}

		if !confirmed {
			return nil
		}
	}

	for _, configurator := range configurators {
		err = executeDiscoveryConfiguratorActions(ctx, configurator.Name(), configurator.Actions())
		if err != nil {
			return trace.Errorf("bootstrap failed to execute, check logs above to see the cause")
		}
	}

	return nil
}

// configureDatabaseAWSFlags common flags provided to aws DB configurators.
type configureDatabaseAWSFlags struct {
	// types comma-separated list of database types that the policies will give
	// access to.
	types string
	// typesList parsed `types` into list of types.
	typesList []string
	// role the AWS role that policies will be attached to.
	role string
	// user the AWS user that policies will be attached to.
	user string
	// policyName name of the generated policy.
	policyName string
	// assumesRoles comma-separated list of external AWS IAM role ARNs that the policy
	// will include in sts:AssumeRole statement.
	assumesRoles string
}

func (f *configureDatabaseAWSFlags) CheckAndSetDefaults() error {
	if f.types == "" && f.assumesRoles == "" {
		return trace.BadParameter("at least one of --assumes-roles or --types should be provided. Valid --types: %s",
			strings.Join(awsDatabaseTypes, ","))
	}

	if f.types != "" {
		f.typesList = strings.Split(f.types, ",")
		for _, dbType := range f.typesList {
			if !slices.Contains(awsDatabaseTypes, dbType) {
				return trace.BadParameter("--types %q not supported. supported types are: %s", dbType, strings.Join(awsDatabaseTypes, ", "))
			}
		}
	}

	return nil
}

// configureDatabaseAWSPrintFlags flags of the "db configure aws print-iam"
// subcommand.
type configureDatabaseAWSPrintFlags struct {
	configureDatabaseAWSFlags
	// policyOnly if "true" will only prints the policy JSON.
	policyOnly bool
	// boundaryOnly if "true" will only prints the policy boundary JSON.
	// TODO(gavin): DELETE IN 18.0.0
	boundaryOnly bool
}

// buildAWSConfigurator builds the database configurator used on AWS-specific
// commands.
func buildAWSConfigurator(manual bool, flags configureDatabaseAWSFlags) (configurators.Configurator, error) {
	err := flags.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configuratorFlags := configurators.BootstrapFlags{
		Manual:            manual,
		PolicyName:        flags.policyName,
		AttachToUser:      flags.user,
		AttachToRole:      flags.role,
		ForceAssumesRoles: flags.assumesRoles,
	}

	for _, dbType := range flags.typesList {
		switch dbType {
		case types.DatabaseTypeRDS:
			configuratorFlags.ForceRDSPermissions = true
		case types.DatabaseTypeRDSProxy:
			configuratorFlags.ForceRDSProxyPermissions = true
		case types.DatabaseTypeRedshift:
			configuratorFlags.ForceRedshiftPermissions = true
		case types.DatabaseTypeRedshiftServerless:
			configuratorFlags.ForceRedshiftServerlessPermissions = true
		case types.DatabaseTypeElastiCache:
			configuratorFlags.ForceElastiCachePermissions = true
		case types.DatabaseTypeMemoryDB:
			configuratorFlags.ForceMemoryDBPermissions = true
		case types.DatabaseTypeAWSKeyspaces:
			configuratorFlags.ForceAWSKeyspacesPermissions = true
		case types.DatabaseTypeDynamoDB:
			configuratorFlags.ForceDynamoDBPermissions = true
		case types.DatabaseTypeOpenSearch:
			configuratorFlags.ForceOpenSearchPermissions = true
		case types.DatabaseTypeDocumentDB:
			configuratorFlags.ForceDocumentDBPermissions = true
		}
	}

	configurator, err := awsconfigurators.NewAWSConfigurator(awsconfigurators.ConfiguratorConfig{
		Flags:         configuratorFlags,
		ServiceConfig: &servicecfg.Config{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return configurator, nil
}

// onConfigureDatabasesAWSPrint is a subcommand used to print AWS IAM access
// Teleport requires to run databases discovery on AWS.
func onConfigureDatabasesAWSPrint(flags configureDatabaseAWSPrintFlags) error {
	configurator, err := buildAWSConfigurator(true, flags.configureDatabaseAWSFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if configurator actions is empty.
	if configurator.IsEmpty() {
		fmt.Println("The agent doesn't require any extra configuration.")
		return nil
	}

	actions := configurator.Actions()
	if flags.policyOnly {
		// Policy is present at the details of the first action.
		fmt.Println(actions[0].Details())
		return nil
	}

	if flags.boundaryOnly {
		fmt.Println("The --boundary flag is deprecated. The IAM permissions model that Teleport uses no longer requires a boundary policy.")
		return nil
	}

	printDiscoveryConfiguratorActions(actions)
	return nil
}

// configureDatabaseAWSPrintFlags flags of the "db configure aws create-iam"
// subcommand.
type configureDatabaseAWSCreateFlags struct {
	configureDatabaseAWSFlags
	attach  bool
	confirm bool
}

// onConfigureDatabasesAWSCreates is a subcommand used to create AWS IAM access
// for Teleport to run databases discovery on AWS.
func onConfigureDatabasesAWSCreate(flags configureDatabaseAWSCreateFlags) error {
	ctx := context.TODO()
	configurator, err := buildAWSConfigurator(false, flags.configureDatabaseAWSFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if configurator actions is empty.
	if configurator.IsEmpty() {
		fmt.Println("The agent doesn't require any extra configuration.")
		return nil
	}

	actions := configurator.Actions()
	printDiscoveryConfiguratorActions(actions)
	fmt.Print("\n")

	if !flags.confirm {
		confirmed, err := prompt.Confirmation(ctx, os.Stdout, prompt.Stdin(), "Confirm?")
		if err != nil {
			return trace.Wrap(err)
		}

		if !confirmed {
			return nil
		}
	}

	err = executeDiscoveryConfiguratorActions(ctx, configurator.Name(), actions)
	if err != nil {
		return trace.Errorf("bootstrap failed to execute, check logs above to see the cause")
	}

	return nil
}

// printDiscoveryConfiguratorActions prints the database configurator actions.
func printDiscoveryConfiguratorActions(actions []configurators.ConfiguratorAction) {
	for i, action := range actions {
		fmt.Printf("%d. %s", i+1, action.Description())
		if len(action.Details()) > 0 {
			fmt.Printf(":\n%s\n\n", action.Details())
		} else {
			fmt.Println(".")
		}
	}
}

// executeDiscoveryConfiguratorActions iterate over all actions, executing and printing
// their results.
func executeDiscoveryConfiguratorActions(ctx context.Context, configuratorName string, actions []configurators.ConfiguratorAction) error {
	actionContext := &configurators.ConfiguratorActionContext{}
	for _, action := range actions {
		err := action.Execute(ctx, actionContext)
		printDiscoveryBootstrapActionResult(configuratorName, action, err)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// printDiscoveryBootstrapActionResult human-readable print of the action result (error
// or success).
func printDiscoveryBootstrapActionResult(configuratorName string, action configurators.ConfiguratorAction, err error) {
	leadSymbol := "✅"
	endText := "done"
	if err != nil {
		leadSymbol = "❌"
		endText = "failed"
	}

	fmt.Printf("%s[%s] %s... %s.\n", leadSymbol, configuratorName, action.Description(), endText)
	if err != nil {
		fmt.Printf("Failure reason: %s\n", err)
	}
}
