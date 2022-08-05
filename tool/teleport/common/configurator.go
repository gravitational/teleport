// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/config"
	dbconfigurators "github.com/gravitational/teleport/lib/configurators/databases"
	"github.com/gravitational/teleport/lib/utils/prompt"

	"github.com/gravitational/trace"
)

// awsDatabaseTypes list of databases supported on the configurator.
var awsDatabaseTypes = []string{types.DatabaseTypeRDS, types.DatabaseTypeRedshift}

type createDatabaseConfigFlags struct {
	config.DatabaseSampleFlags
	// output is the destination to write the configuration to.
	output string
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

// configureDatabaseBootstrapFlags database configure bootstrap flags.
type configureDatabaseBootstrapFlags struct {
	config  dbconfigurators.BootstrapFlags
	confirm bool
}

// onConfigureDatabaseBootstrap subcommand that bootstraps configuration for
// database agents.
func onConfigureDatabaseBootstrap(flags configureDatabaseBootstrapFlags) error {
	ctx := context.TODO()
	configurators, err := dbconfigurators.BuildConfigurators(flags.config)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Reading configuration at %q...\n\n", flags.config.ConfigPath)
	if len(configurators) == 0 {
		fmt.Println("The agent doesn’t require any extra configuration.")
		return nil
	}

	for _, configurator := range configurators {
		fmt.Println(configurator.Name())
		printDBConfiguratorActions(configurator.Actions())
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
		err = executeDBConfiguratorActions(ctx, configurator.Name(), configurator.Actions())
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
}

func (f *configureDatabaseAWSFlags) CheckAndSetDefaults() error {
	if f.types == "" {
		return trace.BadParameter("at least one --types should be provided: %s", strings.Join(awsDatabaseTypes, ","))
	}

	f.typesList = strings.Split(f.types, ",")
	for _, dbType := range f.typesList {
		if !apiutils.SliceContainsStr(awsDatabaseTypes, dbType) {
			return trace.BadParameter("--types %q not supported. supported types are: %s", dbType, strings.Join(awsDatabaseTypes, ", "))
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
	boundaryOnly bool
}

// buildAWSConfigurator builds the database configurator used on AWS-specific
// commands.
func buildAWSConfigurator(manual bool, flags configureDatabaseAWSFlags) (dbconfigurators.Configurator, error) {
	err := flags.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fileConfig := &config.FileConfig{}
	configuratorFlags := dbconfigurators.BootstrapFlags{
		Manual:       manual,
		PolicyName:   flags.policyName,
		AttachToUser: flags.user,
		AttachToRole: flags.role,
	}

	for _, dbType := range flags.typesList {
		switch dbType {
		case types.DatabaseTypeRDS:
			configuratorFlags.ForceRDSPermissions = true
		}
	}

	configurator, err := dbconfigurators.NewAWSConfigurator(dbconfigurators.AWSConfiguratorConfig{
		Flags:      configuratorFlags,
		FileConfig: fileConfig,
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
		fmt.Println("The agent doesn’t require any extra configuration.")
		return nil
	}

	actions := configurator.Actions()
	if flags.policyOnly {
		// Policy is present at the details of the first action.
		fmt.Println(actions[0].Details())
		return nil
	}

	if flags.boundaryOnly {
		// Policy boundary is present at the details of the second instruction.
		fmt.Println(actions[1].Details())
		return nil
	}

	printDBConfiguratorActions(actions)
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

	actions := configurator.Actions()
	printDBConfiguratorActions(actions)
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

	// Check if configurator actions is empty.
	if configurator.IsEmpty() {
		fmt.Println("The agent doesn’t require any extra configuration.")
		return nil
	}

	err = executeDBConfiguratorActions(ctx, configurator.Name(), actions)
	if err != nil {
		return trace.Errorf("bootstrap failed to execute, check logs above to see the cause")
	}

	return nil
}

// printDBConfiguratorActions prints the database configurator actions.
func printDBConfiguratorActions(actions []dbconfigurators.ConfiguratorAction) {
	for i, action := range actions {
		fmt.Printf("%d. %s", i+1, action.Description())
		if len(action.Details()) > 0 {
			fmt.Printf(":\n%s\n\n", action.Details())
		} else {
			fmt.Println(".")
		}
	}
}

// executeDBConfiguratorActions iterate over all actions, executing and priting
// their results.
func executeDBConfiguratorActions(ctx context.Context, configuratorName string, actions []dbconfigurators.ConfiguratorAction) error {
	actionContext := &dbconfigurators.ConfiguratorActionContext{}
	for _, action := range actions {
		err := action.Execute(ctx, actionContext)
		printDBBootstrapActionResult(configuratorName, action, err)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// printDBBootstrapActionResult human-readable print of the action result (error
// or success).
func printDBBootstrapActionResult(configuratorName string, action dbconfigurators.ConfiguratorAction, err error) {
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
