// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package expression

import (
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

func NewBotInstanceExpressionParser() (*typical.Parser[*Environment, bool], error) {
	spec := expression.DefaultParserSpec[*Environment]()

	spec.Variables = map[string]typical.Variable{
		"name": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetMetadata().GetName(), nil
		}),
		"metadata.name": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetMetadata().GetName(), nil
		}),
		"spec.bot_name": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetSpec().GetBotName(), nil
		}),
		"spec.instance_id": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetSpec().GetInstanceId(), nil
		}),
		"status.latest_heartbeat.architecture": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetLatestHeartbeat().GetArchitecture(), nil
		}),
		"status.latest_heartbeat.os": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetLatestHeartbeat().GetOs(), nil
		}),
		"status.latest_heartbeat.hostname": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetLatestHeartbeat().GetHostname(), nil
		}),
		"status.latest_heartbeat.one_shot": typical.DynamicVariable(func(env *Environment) (bool, error) {
			return env.GetLatestHeartbeat().GetOneShot(), nil
		}),
		"status.latest_heartbeat.version": typical.DynamicVariable(func(env *Environment) (*semver.Version, error) {
			if env.GetLatestHeartbeat().GetVersion() == "" {
				return nil, nil
			}
			return semver.NewVersion(env.LatestHeartbeat.Version)
		}),
		"status.latest_authentication.join_method": typical.DynamicVariable(func(env *Environment) (string, error) {
			return env.GetLatestAuthentication().GetJoinMethod(), nil
		}),
	}

	// e.g. `newer_than(status.latest_heartbeat.version, "19.0.0")`
	spec.Functions["newer_than"] = typical.BinaryFunction[*Environment](semverGt)
	// e.g. `older_than(status.latest_heartbeat.version, "19.0.2")`
	spec.Functions["older_than"] = typical.BinaryFunction[*Environment](semverLt)
	// e.g. `between(status.latest_heartbeat.version, "19.0.0", "19.0.2")`
	spec.Functions["between"] = typical.TernaryFunction[*Environment](semverBetween)
	// e.g. `exact_version(status.latest_heartbeat.version, "19.1.0")`
	spec.Functions["exact_version"] = typical.BinaryFunction[*Environment](semverEq)

	return typical.NewParser[*Environment, bool](spec)
}

func semverGt(a, b any) (bool, error) {
	va, err := toSemver(a)
	if va == nil || err != nil {
		return false, err
	}
	vb, err := toSemver(b)
	if vb == nil || err != nil {
		return false, err
	}
	return va.Compare(*vb) > 0, nil
}

func semverLt(a, b any) (bool, error) {
	va, err := toSemver(a)
	if va == nil || err != nil {
		return false, err
	}
	vb, err := toSemver(b)
	if vb == nil || err != nil {
		return false, err
	}
	return va.Compare(*vb) < 0, nil
}

func semverEq(a, b any) (bool, error) {
	va, err := toSemver(a)
	if va == nil || err != nil {
		return false, err
	}
	vb, err := toSemver(b)
	if vb == nil || err != nil {
		return false, err
	}
	return va.Compare(*vb) == 0, nil
}

func semverBetween(c, a, b any) (bool, error) {
	gt, err := semverGt(c, a)
	if err != nil {
		return false, err
	}
	eq, err := semverEq(c, a)
	if err != nil {
		return false, err
	}
	lt, err := semverLt(c, b)
	if err != nil {
		return false, err
	}
	return (gt || eq) && lt, nil
}

func toSemver(anyV any) (*semver.Version, error) {
	switch v := anyV.(type) {
	case *semver.Version:
		return v, nil
	case string:
		return semver.NewVersion(v)
	default:
		return nil, trace.BadParameter("type %T cannot be parsed as semver.Version", v)
	}
}
