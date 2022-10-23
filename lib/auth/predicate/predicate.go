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

package predicate

import (
	"reflect"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

func getIdentifier(obj any, selectors []string) (any, error) {
	for _, s := range selectors {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			return nil, trace.BadParameter("cannot take field of nil")
		}

		if m, ok := obj.(map[string]any); ok {
			obj = m[s]
		} else {
			return nil, trace.BadParameter("cannot take field of type: %T", obj)
		}
	}

	return obj, nil
}

func getProperty(m any, k any) (any, error) {
	switch mT := m.(type) {
	case map[string]any:
		kS, ok := k.(string)
		if !ok {
			return nil, trace.BadParameter("unsupported key type: %T", k)
		}

		return mT[kS], nil
	default:
		return nil, trace.BadParameter("cannot take property of type: %T", m)
	}
}

type NamedParameter interface {
	GetName() string
	GetMap() map[string]any
}

func buildEnv(objects ...NamedParameter) map[string]any {
	env := make(map[string]any)
	for _, obj := range objects {
		env[obj.GetName()] = obj.GetMap()
	}

	return env
}

type PredicateAccessChecker struct {
	polices []types.Policy
}

func NewPredicateAccessChecker(policies []types.Policy) *PredicateAccessChecker {
	return &PredicateAccessChecker{policies}
}

func (c *PredicateAccessChecker) CheckAccessToNode(node *Node, user *User) (bool, error) {
	env := buildEnv(node, user)
	return c.checkPolicyExprs("node", env)
}

func (c *PredicateAccessChecker) checkPolicyExprs(scope string, env map[string]any) (bool, error) {
	getIdentifierInEnv := func(selector []string) (any, error) {
		return getIdentifier(env, selector)
	}

	parser, err := predicate.NewParser(predicate.Def{
		// todo: fix this
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
			EQ:  predicate.Equals,
		},
		// todo: fix this
		Functions:     map[string]any{},
		GetIdentifier: getIdentifierInEnv,
		GetProperty:   getProperty,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	evaluate := func(expr string) (bool, error) {
		ifn, err := parser.Parse(expr)
		if err != nil {
			return false, trace.Wrap(err)
		}

		fn, ok := ifn.(predicate.BoolPredicate)
		if !ok {
			return false, trace.BadParameter("unsupported type: %T", ifn)
		}

		return fn(), nil
	}

	for _, policy := range c.polices {
		if expr, ok := policy.GetDeny()[scope]; ok {
			denied, err := evaluate(expr)
			if err != nil {
				return false, trace.Wrap(err)
			}

			if denied {
				return false, nil
			}
		}
	}

	for _, policy := range c.polices {
		if expr, ok := policy.GetAllow()[scope]; ok {
			allowed, err := evaluate(expr)
			if err != nil {
				return false, trace.Wrap(err)
			}

			if allowed {
				return true, nil
			}
		}
	}

	return false, nil
}

type Node struct {
	login  string
	labels map[string]string
}

func (n *Node) GetName() string {
	return "node"
}

func (n *Node) GetMap() map[string]any {
	return map[string]any{
		"login":  n.login,
		"labels": n.labels,
	}
}

type User struct {
	name   string
	traits map[string][]string
}

func (u *User) GetName() string {
	return "user"
}

func (u *User) GetMap() map[string]any {
	return map[string]any{
		"name":   u.name,
		"traits": u.traits,
	}
}
