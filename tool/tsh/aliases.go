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

package main

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

const tshAliasEnvKey = "TSH_ALIAS"

// tryRunAlias inspects the arguments to see if the alias command should be run, and does so if required.
func tryRunAlias(ctx context.Context, currentExecPath string, aliases map[string]string, args []string, setEnv func(key, value string) error, getEnv func(key string) string) (bool, string, error) {
	// find the alias to use
	aliasCmd, aliasIx := findCommand(args)

	// ignore aliases found in TSH_ALIAS list
	aliasesSeen := getSeenAliases(getEnv)
	for _, usedAlias := range aliasesSeen {
		if usedAlias == aliasCmd {
			return false, aliasCmd, nil
		}
	}
	aliasesSeen = append(aliasesSeen, aliasCmd)

	// match?
	aliasDef, ok := aliases[aliasCmd]
	if !ok {
		return false, "", nil
	}

	runtimeArgs := append(args[:aliasIx], args[aliasIx+1:]...)
	aliasExpanded, err := expandAliasDefinition(aliasDef, runtimeArgs)
	if err != nil {
		return true, aliasCmd, trace.Wrap(err)
	}

	if len(aliasExpanded) == 0 {
		return true, aliasCmd, trace.BadParameter("invalid alias: expanded to empty list.")
	}

	executable := aliasExpanded[0]
	aliasArgs := aliasExpanded[1:]

	return true, aliasCmd, runAliasCommand(ctx, currentExecPath, setEnv, aliasesSeen, executable, aliasArgs)
}

// runAliasCommand actually runs requested alias command.
func runAliasCommand(ctx context.Context, currentExecPath string, setEnv func(key, value string) error, aliasesSeen []string, executable string, arguments []string) error {
	execPath, err := exec.LookPath(executable)
	if err != nil {
		return trace.Wrap(err, "failed to find a executable %q", executable)
	}

	err = setEnv(tshAliasEnvKey, strings.Join(aliasesSeen, ","))
	if err != nil {
		return trace.Wrap(err, "failed to set env variable %q", tshAliasEnvKey)
	}

	// if execPath is our path, skip re-execution and run main directly instead.
	// this makes for better error messages in case of failures.
	if execPath == currentExecPath {
		log.Debugf("self re-exec command: tsh %v", arguments)
		return trace.Wrap(Run(ctx, arguments))
	}

	cmd := exec.Command(execPath, arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debugf("running external command: %v", cmd)

	err = cmd.Run()
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return trace.Wrap(exitErr)
	}

	return trace.Wrap(err, "failed to run command: %v %v", execPath, strings.Join(arguments, " "))
}

// findCommand inspects the argument list to find first non-option (i.e. command), returning it along with the index it was found at.
func findCommand(args []string) (string, int) {
	aliasCmd := ""
	aliasIx := -1

	for i, arg := range args {
		if arg == "" {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			continue
		}

		aliasCmd = arg
		aliasIx = i
		break
	}

	return aliasCmd, aliasIx
}

// getSeenAliases fetches TSH_ALIAS env variable and parses it, to produce the list of already executed aliases.
func getSeenAliases(getEnv func(key string) string) []string {
	var aliasesSeen []string

	for _, val := range strings.Split(getEnv(tshAliasEnvKey), ",") {
		if strings.TrimSpace(val) != "" {
			aliasesSeen = append(aliasesSeen, val)
		}
	}

	return aliasesSeen
}

// expandAliasDefinition expands $0, $1, ... within alias definition. Arguments not referenced in alias are appended at the end.
func expandAliasDefinition(aliasDef string, runtimeArgs []string) ([]string, error) {
	var appendArgs []string

	for i, arg := range runtimeArgs {
		variable := "$" + strconv.Itoa(i)
		if strings.Contains(aliasDef, variable) {
			aliasDef = strings.ReplaceAll(aliasDef, variable, arg)
		} else {
			appendArgs = append(appendArgs, arg)
		}
	}

	missingRefs := regexp.MustCompile(`\$\d`)
	if missingRefs.MatchString(aliasDef) {
		return nil, trace.BadParameter("unsatisfied argument variables for alias: %v", aliasDef)
	}

	split, err := shlex.Split(aliasDef)
	if err != nil {
		return nil, err
	}

	out := append(split, appendArgs...)
	return out, nil
}
