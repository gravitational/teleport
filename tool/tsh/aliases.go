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

// aliasRunner coordinates alias running as well as provides a suitable testing target.
type aliasRunner struct {
	getEnv func(key string) string
	setEnv func(key, value string) error

	aliases map[string]string

	runTshMain         func(ctx context.Context, args []string, opts ...cliOption) error
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

// findAliasCommand inspects the argument list to find first non-option (i.e. command). The command is returned along with the argument list from which the command was removed.
func findAliasCommand(args []string) (string, []string) {
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

	if aliasCmd == "" {
		return "", nil
	}

	runtimeArgs := make([]string, 0)
	runtimeArgs = append(runtimeArgs, args[:aliasIx]...)
	runtimeArgs = append(runtimeArgs, args[aliasIx+1:]...)

	return aliasCmd, runtimeArgs
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
		return nil, trace.Wrap(err)
	}

	out := append(split, appendArgs...)
	return out, nil
}

// getAliasDefinition returns the alias definition if it exists and the alias is still eligible for running.
func (ar *aliasRunner) getAliasDefinition(aliasCmd string) (string, bool) {
	// ignore aliases found in TSH_ALIAS list
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

// getSeenAliases fetches TSH_ALIAS env variable and parses it, to produce the list of already executed aliases.
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
func (ar *aliasRunner) runAliasCommand(ctx context.Context, currentExecPath string, executable string, arguments []string) error {
	execPath, err := exec.LookPath(executable)
	if err != nil {
		return trace.Wrap(err, "failed to find the executable %q", executable)
	}

	// if execPath is our path, skip re-execution and run main directly instead.
	// this makes for better error messages in case of failures.
	if execPath == currentExecPath {
		log.Debugf("self re-exec command: tsh %v", arguments)
		return trace.Wrap(ar.runTshMain(ctx, arguments))
	}

	cmd := exec.Command(execPath, arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debugf("running external command: %v", cmd)
	err = ar.runExternalCommand(cmd)
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return trace.Wrap(exitErr)
	}

	return trace.Wrap(err, "failed to run command: %v %v", execPath, strings.Join(arguments, " "))
}
