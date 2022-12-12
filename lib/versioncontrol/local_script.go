/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package versioncontrol

import (
	"strings"

	"github.com/gravitational/teleport/api/types"
	vc "github.com/gravitational/teleport/api/versioncontrol"
	"github.com/gravitational/trace"
)

// versionInterpolation is the interpolation expression for extracting the version string
// from a teleport install target.
const versionInterpolation = `{{target.version}}`

// verionVarName is the default teleport version variable.
const versionVarName = `TELEPORT_VERSION`

// InstallScriptMsgForInstance builds an installer.sh script exec message, customized for a specific instance/target.
func InstallScriptMsgForInstance(installer types.LocalScriptInstaller, instance types.Instance, target vc.Target) (types.ExecScript, error) {
	return execMsgForInstance(installer.BaseInstallMsg(), instance, target)
}

// RestartScriptMsgForInstance builds a restart.sh script exec message, customized for a specific instance/target.
func RestartScriptMsgForInstance(installer types.LocalScriptInstaller, instance types.Instance, target vc.Target) (types.ExecScript, error) {
	return execMsgForInstance(installer.BaseRestartMsg(), instance, target)
}

// execMsgForInstance sets up default env/expect parameters, and performs variable interpolation.
func execMsgForInstance(msg types.ExecScript, instance types.Instance, target vc.Target) (types.ExecScript, error) {
	if len(msg.Env) == 0 {
		msg.Env = map[string]string{
			versionVarName: versionInterpolation,
		}
	}

	var err error
	msg.Env, err = interpolateEnv(msg.Env, target)
	if err != nil {
		return types.ExecScript{}, trace.Wrap(err)
	}

	if len(msg.Expect) == 0 {
		msg.Expect = vc.NewTarget(vc.Normalize(instance.GetTeleportVersion()))
	}

	return msg, nil
}

// interpolateEnv performs variable-substitution on the supplied env, returning a separate copy.
// the only currently supported interpolation expression is `{{target.version}}`, but more powerful
// expressions will be added in the future.
func interpolateEnv(env map[string]string, target vc.Target) (map[string]string, error) {
	const versionInterpolation = `{{target.version}}`
	if env == nil {
		return nil, nil
	}

	out := make(map[string]string, len(env))

	if !target.Ok() {
		return nil, trace.Errorf("cannot interpolate env with invalid target: %+v", target)
	}

	for key, val := range env {
		// we currently support exactly one interpolation expression (`{{target.version}}`). additional expressions
		// will be supported, but that will likely require some changes to utils/parse, which is currently fairly
		// specialized toward trait interpolation. Ideally, we should support a compatible subset of the expressions
		// supported there.
		if strings.Contains(val, "{{") || strings.Contains(val, "}}") {
			if val != versionInterpolation {
				return nil, trace.BadParameter("unsupported env interpolation %q (only %q is currently supported)", val, versionInterpolation)
			}
			val = target.Version()
		}
		out[key] = val
	}
	return out, nil
}
