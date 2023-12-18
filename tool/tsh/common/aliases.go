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
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/shlex"
	"github.com/gravitational/trace"
)

// tshAliasEnvKey is an env variable storing the aliases that, so far, has been expanded, and should not be expanded again.
// This is primarily to avoid infinite loops with ill-defined aliases, but can also be used to disable a particular alias on demand.
const tshAliasEnvKey = "TSH_UNALIAS"

// aliasRunner coordinates alias running as well as provides a suitable testing target.
type aliasRunner struct {
	// getEnv is a function to retrieve env variable, for example os.Getenv.
	getEnv func(key string) string
	// setEnv is a function to set env variable, for example os.Setenv.
	setEnv func(key, value string) error

	// aliases is a list of alias definitions, keyed by alias name; typically loaded from config file.
	aliases map[string]string

	// runTshMain is a function to run tsh; for example Run().
	runTshMain func(ctx context.Context, args []string, opts ...CliOption) error
	// runExternalCommand is a function to execute the command passed in as argument; outside of test, it should simply invoke the Run() method.
	runExternalCommand func(cmd *exec.Cmd) error
}

// newAliasRunner returns regular alias runner; the tests are using a different function.
func newAliasRunner(aliases map[string]string) *aliasRunner {
	return &aliasRunner{
		getEnv:     os.Getenv,
		setEnv:     os.Setenv,
		aliases:    aliases,
		runTshMain: Run,
		runExternalCommand: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
	}
}

// findAliasCommand inspects the argument list to find first non-option (i.e. command).
// The command is returned along with the argument list from which the command was removed.
// Examples:
// - []string{"foo", "bar", "baz"}   => "foo", []string{"bar", "baz"}
// - []string{"--foo", "bar", "baz"} => "bar", []string{"--foo", "baz"}
// - []string{"--foo", "", "baz"}    => "baz", []string{"--foo", ""}
func findAliasCommand(args []string) (string, []string) {
	aliasCmd := ""
	aliasIx := -1

	for i, arg := range args {
		if arg == "" {
			continue
		}

		// we are looking for the first non-flag argument.
		if strings.HasPrefix(arg, "-") {
			continue
		}

		aliasCmd = arg
		aliasIx = i
		break
	}

	if aliasCmd == "" {
		return "", nil
	}

	runtimeArgs := make([]string, 0)
	runtimeArgs = append(runtimeArgs, args[:aliasIx]...)
	runtimeArgs = append(runtimeArgs, args[aliasIx+1:]...)

	return aliasCmd, runtimeArgs
}

// numVarRegex is a regex for variables such as $0, $1 ...  $100 ... that can be used in alias definitions.
var numVarRegex = regexp.MustCompile(`(\$\d+)|(\$TSH)`)

// expandAliasDefinition expands variables within alias definition.
// Typically these are $0, $1, ... corresponding to runtime arguments given.
// Arguments not referenced in alias are appended at the end.
// As a special case, we also support $TSH variable: it is replaced with path to the current tsh executable.
func expandAliasDefinition(executablePath, aliasName, aliasDef string, runtimeArgs []string) ([]string, error) {
	// prepare maps for all arguments
	varMap := map[string]string{}
	unusedVars := map[string]int{}
	for i, value := range runtimeArgs {
		variable := "$" + strconv.Itoa(i)
		varMap[variable] = value
		unusedVars[variable] = i
	}

	varMap["$TSH"] = executablePath

	// keep count of maximum missing variable
	maxMissing := -1

	expanded := numVarRegex.ReplaceAllStringFunc(aliasDef, func(variable string) string {
		if value, ok := varMap[variable]; ok {
			delete(unusedVars, variable)
			return value
		}

		ix, err := strconv.Atoi(variable[1:])
		if err != nil {
			return variable
		}

		if maxMissing < ix {
			maxMissing = ix
		}

		return variable
	})

	// report missing variables
	if maxMissing > -1 {
		return nil, trace.BadParameter("tsh alias %q requires %v arguments, but was invoked with %v", aliasName, maxMissing+1, len(runtimeArgs))
	}

	split, err := shlex.Split(expanded)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var appendArgs []string
	for i, value := range runtimeArgs {
		variable := "$" + strconv.Itoa(i)
		if _, ok := unusedVars[variable]; ok {
			appendArgs = append(appendArgs, value)
		}
	}

	out := append(split, appendArgs...)
	return out, nil
}

// getAliasDefinition returns the alias definition if it exists and the alias is still eligible for running.
func (ar *aliasRunner) getAliasDefinition(aliasCmd string) (string, bool) {
	// ignore aliases found in TSH_UNALIAS list
	for _, usedAlias := range ar.getSeenAliases() {
		if usedAlias == aliasCmd {
			return "", false
		}
	}

	aliasDef, ok := ar.aliases[aliasCmd]
	return aliasDef, ok
}

// markAliasSeen adds another alias to the list of aliases seen.
func (ar *aliasRunner) markAliasSeen(alias string) error {
	aliasesSeen := ar.getSeenAliases()
	aliasesSeen = append(aliasesSeen, alias)
	return ar.setEnv(tshAliasEnvKey, strings.Join(aliasesSeen, ","))
}

// getSeenAliases fetches TSH_UNALIAS env variable and parses it, to produce the list of already executed aliases.
func (ar *aliasRunner) getSeenAliases() []string {
	var aliasesSeen []string

	for _, val := range strings.Split(ar.getEnv(tshAliasEnvKey), ",") {
		if strings.TrimSpace(val) != "" {
			aliasesSeen = append(aliasesSeen, val)
		}
	}

	return aliasesSeen
}

// runAliasCommand actually runs requested alias command. If the executable resolves to the process itself it will directly call `Run()`, otherwise a new process will be spawned.
func (ar *aliasRunner) runAliasCommand(ctx context.Context, currentExecPath, executable string, arguments []string) error {
	execPath, err := exec.LookPath(executable)
	if err != nil {
		return trace.Wrap(err, "failed to find the executable %q", executable)
	}

	// if execPath is our path, skip re-execution and run main directly instead.
	// this makes for better error messages in case of failures.
	if execPath == currentExecPath {
		log.Debugf("Self re-exec command: tsh %v.", arguments)
		return trace.Wrap(ar.runTshMain(ctx, arguments))
	}

	cmd := exec.Command(execPath, arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debugf("Running external command: %v", cmd)
	err = ar.runExternalCommand(cmd)
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return trace.Wrap(exitErr)
	}

	return trace.Wrap(err, "failed to run command: %v %v", execPath, strings.Join(arguments, " "))
}

func (ar *aliasRunner) runAlias(ctx context.Context, aliasCommand, aliasDefinition, executablePath string, runtimeArgs []string) error {
	err := ar.markAliasSeen(aliasCommand)
	if err != nil {
		return trace.Wrap(err)
	}

	newArgs, err := expandAliasDefinition(executablePath, aliasCommand, aliasDefinition, runtimeArgs)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ar.runAliasCommand(ctx, executablePath, newArgs[0], newArgs[1:])
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
