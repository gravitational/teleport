/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/predicate"
	"github.com/vulcand/predicate/builder"
)

// RuleContext specifies context passed to the
// rule processing matcher, and contains information
// about current session, e.g. current user
type RuleContext interface {
	// GetIdentifier returns identifier defined in a context
	GetIdentifier(fields []string) (interface{}, error)
	// GetResource returns resource if specified in the context,
	// if unspecified, returns error.
	GetResource() (types.Resource, error)
}

var (
	// ResourceNameExpr is the identifier that specifies resource name.
	ResourceNameExpr = builder.Identifier("resource.metadata.name")
	// CertAuthorityTypeExpr is a function call that returns
	// cert authority type.
	CertAuthorityTypeExpr = builder.Identifier(`system.catype()`)
)

// NewWhereParser returns standard parser for `where` section in access rules.
func NewWhereParser(ctx RuleContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
		},
		Functions: map[string]interface{}{
			"equals":   predicate.Equals,
			"contains": predicate.Contains,
			// system.catype is a function that returns cert authority type,
			// it returns empty values for unrecognized values to
			// pass static rule checks.
			"system.catype": func() (interface{}, error) {
				resource, err := ctx.GetResource()
				if err != nil {
					if trace.IsNotFound(err) {
						return "", nil
					}
					return nil, trace.Wrap(err)
				}
				ca, ok := resource.(types.CertAuthority)
				if !ok {
					return "", nil
				}
				return string(ca.GetType()), nil
			},
		},
		GetIdentifier: ctx.GetIdentifier,
		GetProperty:   GetStringMapValue,
	})
}

// GetStringMapValue is a helper function that returns property
// from map[string]string or map[string][]string
// the function returns empty value in case if key not found
// In case if map is nil, returns empty value as well
func GetStringMapValue(mapVal, keyVal interface{}) (interface{}, error) {
	key, ok := keyVal.(string)
	if !ok {
		return nil, trace.BadParameter("only string keys are supported")
	}
	switch m := mapVal.(type) {
	case map[string][]string:
		if len(m) == 0 {
			// to return nil with a proper type
			var n []string
			return n, nil
		}
		return m[key], nil
	case wrappers.Traits:
		if len(m) == 0 {
			// to return nil with a proper type
			var n []string
			return n, nil
		}
		return m[key], nil
	case map[string]string:
		if len(m) == 0 {
			return "", nil
		}
		return m[key], nil
	default:
		_, ok := mapVal.(map[string][]string)
		return nil, trace.BadParameter("type %T is not supported, but %v %#v", m, ok, mapVal)
	}
}

// NewActionsParser returns standard parser for 'actions' section in access rules
func NewActionsParser(ctx RuleContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{},
		Functions: map[string]interface{}{
			"log": NewLogActionFn(ctx),
		},
		GetIdentifier: ctx.GetIdentifier,
		GetProperty:   predicate.GetStringMapValue,
	})
}

// NewLogActionFn creates logger functions
func NewLogActionFn(ctx RuleContext) interface{} {
	l := &LogAction{ctx: ctx}
	writer, ok := ctx.(io.Writer)
	if ok && writer != nil {
		l.writer = writer
	}
	return l.Log
}

// LogAction represents action that will emit log entry
// when specified in the actions of a matched rule
type LogAction struct {
	ctx    RuleContext
	writer io.Writer
}

// Log logs with specified level and formatting string with arguments
func (l *LogAction) Log(level, format string, args ...interface{}) predicate.BoolPredicate {
	return func() bool {
		ilevel, err := log.ParseLevel(level)
		if err != nil {
			ilevel = log.DebugLevel
		}
		var writer io.Writer
		if l.writer != nil {
			writer = l.writer
		} else {
			writer = log.StandardLogger().WriterLevel(ilevel)
		}
		writer.Write([]byte(fmt.Sprintf(format, args...)))
		return true
	}
}

// Context is a default rule context used in teleport
type Context struct {
	// User is currently authenticated user
	User types.User
	// Resource is an optional resource, in case if the rule
	// checks access to the resource
	Resource types.Resource
	// Session is an optional session.end event. These events hold information
	// about session recordings.
	Session *events.SessionEnd
	// SSHSession is an optional (active) SSH session.
	SSHSession *session.Session
}

// String returns user friendly representation of this context
func (ctx *Context) String() string {
	return fmt.Sprintf("user %v, resource: %v", ctx.User, ctx.Resource)
}

const (
	// UserIdentifier represents user registered identifier in the rules
	UserIdentifier = "user"
	// ResourceIdentifier represents resource registered identifier in the rules
	ResourceIdentifier = "resource"
	// ResourceLabelsIdentifier refers to the static and dynamic labels in a resource.
	ResourceLabelsIdentifier = "labels"
	// ResourceNameIdentifier refers to the metadata name field for a resource.
	ResourceNameIdentifier = "name"
	// SessionIdentifier refers to a session (recording) in the rules.
	SessionIdentifier = "session"
	// SSHSessionIdentifier refers to an (active) SSH session in the rules.
	SSHSessionIdentifier = "ssh_session"
	// ImpersonateRoleIdentifier is a role to impersonate
	ImpersonateRoleIdentifier = "impersonate_role"
	// ImpersonateUserIdentifier is a user to impersonate
	ImpersonateUserIdentifier = "impersonate_user"
)

// GetResource returns resource specified in the context,
// returns error if not specified.
func (ctx *Context) GetResource() (types.Resource, error) {
	if ctx.Resource == nil {
		return nil, trace.NotFound("resource is not set in the context")
	}
	return ctx.Resource, nil
}

// GetIdentifier returns identifier defined in a context
func (ctx *Context) GetIdentifier(fields []string) (interface{}, error) {
	switch fields[0] {
	case UserIdentifier:
		var user types.User
		if ctx.User == nil {
			user = emptyUser
		} else {
			user = ctx.User
		}
		return predicate.GetFieldByTag(user, teleport.JSON, fields[1:])
	case ResourceIdentifier:
		var resource types.Resource
		if ctx.Resource == nil {
			resource = emptyResource
		} else {
			resource = ctx.Resource
		}
		return predicate.GetFieldByTag(resource, teleport.JSON, fields[1:])
	case SessionIdentifier:
		session := &events.SessionEnd{}
		if ctx.Session != nil {
			session = ctx.Session
		}
		return predicate.GetFieldByTag(session, teleport.JSON, fields[1:])
	case SSHSessionIdentifier:
		// Do not expose the original session.Session, instead transform it into a
		// ctxSession so the exposed fields match our desired API.
		return predicate.GetFieldByTag(toCtxSession(ctx.SSHSession), teleport.JSON, fields[1:])
	default:
		return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
	}
}

// ctxSession represents the public contract of a session.Session, as exposed
// to a Context rule.
// See RFD 45:
// https://github.com/gravitational/teleport/blob/master/rfd/0045-ssh_session-where-condition.md#replacing-parties-by-usernames.
type ctxSession struct {
	// Namespace is a session namespace, separating sessions from each other.
	Namespace string `json:"namespace"`
	// Login is a login used by all parties joining the session.
	Login string `json:"login"`
	// Created records the information about the time when session was created.
	Created time.Time `json:"created"`
	// LastActive holds the information about when the session was last active.
	LastActive time.Time `json:"last_active"`
	// ServerID of session.
	ServerID string `json:"server_id"`
	// ServerHostname of session.
	ServerHostname string `json:"server_hostname"`
	// ServerAddr of session.
	ServerAddr string `json:"server_addr"`
	// ClusterName is the name of cluster that this session belongs to.
	ClusterName string `json:"cluster_name"`
	// Participants is a list of session participants expressed as usernames.
	Participants []string `json:"participants"`
}

func toCtxSession(s *session.Session) ctxSession {
	if s == nil {
		return ctxSession{}
	}
	return ctxSession{
		Namespace:      s.Namespace,
		Login:          s.Login,
		Created:        s.Created,
		LastActive:     s.LastActive,
		ServerID:       s.ServerID,
		ServerHostname: s.ServerHostname,
		ServerAddr:     s.ServerAddr,
		ClusterName:    s.ClusterName,
		Participants:   s.Participants(),
	}
}

// emptyResource is used when no resource is specified
var emptyResource = &EmptyResource{}

// emptyUser is used when no user is specified
var emptyUser = &types.UserV2{}

// EmptyResource is used to represent a use case when no resource
// is specified in the rules matcher
type EmptyResource struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata types.Metadata `json:"metadata"`
}

// GetVersion returns resource version
func (r *EmptyResource) GetVersion() string {
	return r.Version
}

// GetSubKind returns resource sub kind
func (r *EmptyResource) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *EmptyResource) SetSubKind(s string) {
	r.SubKind = s
}

// GetKind returns resource kind
func (r *EmptyResource) GetKind() string {
	return r.Kind
}

// GetResourceID returns resource ID
func (r *EmptyResource) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *EmptyResource) SetResourceID(id int64) {
	r.Metadata.ID = id
}

// SetExpiry sets expiry time for the object.
func (r *EmptyResource) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *EmptyResource) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetName sets the role name and is a shortcut for SetMetadata().Name.
func (r *EmptyResource) SetName(s string) {
	r.Metadata.Name = s
}

// GetName gets the role name and is a shortcut for GetMetadata().Name.
func (r *EmptyResource) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata.
func (r *EmptyResource) GetMetadata() types.Metadata {
	return r.Metadata
}

func (r *EmptyResource) CheckAndSetDefaults() error { return nil }

// BoolPredicateParser extends predicate.Parser with a convenience method
// for evaluating bool predicates.
type BoolPredicateParser interface {
	predicate.Parser
	EvalBoolPredicate(string) (bool, error)
}

type boolPredicateParser struct {
	predicate.Parser
}

func (p boolPredicateParser) EvalBoolPredicate(expr string) (bool, error) {
	ifn, err := p.Parse(expr)
	if err != nil {
		return false, trace.Wrap(err)
	}

	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("unsupported type: %T", ifn)
	}

	return fn(), nil
}

// NewJSONBoolParser returns a generic parser for boolean expressions based on a
// json-serializable context.
func NewJSONBoolParser(ctx interface{}) (BoolPredicateParser, error) {
	p, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
		},
		Functions: map[string]interface{}{
			"equals":   predicate.Equals,
			"contains": predicate.Contains,
		},
		GetIdentifier: func(fields []string) (interface{}, error) {
			return predicate.GetFieldByTag(ctx, teleport.JSON, fields)
		},
		GetProperty: GetStringMapValue,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return boolPredicateParser{Parser: p}, nil
}

// newParserForIdentifierSubcondition returns a parser customized for
// extracting the largest admissible subexpression of a `where` condition that
// involves the given identifier.
//
// For example, consider the `where` condition
// `contains(session.participants, "user") && equals(user.metadata.name, "user")`.
// Given a RuleContext where user.metadata.name is equal to "user", its largest
// admissible subcondition involving the identifier "session" is
// `contains(session.participants, "user")`. With another RuleContext the
// largest such subcondition is the empty expression.
func newParserForIdentifierSubcondition(ctx RuleContext, identifier string) (predicate.Parser, error) {
	binaryPred := func(predFn func(a, b interface{}) predicate.BoolPredicate, exprFn func(a, b types.WhereExpr) types.WhereExpr) func(a, b interface{}) types.WhereExpr {
		return func(a, b interface{}) types.WhereExpr {
			an, aOK := a.(types.WhereExpr)
			if !aOK {
				an = types.WhereExpr{Literal: a}
			}
			bn, bOK := b.(types.WhereExpr)
			if !bOK {
				bn = types.WhereExpr{Literal: b}
			}
			if an.Literal != nil && bn.Literal != nil {
				return types.WhereExpr{Literal: predFn(an.Literal, bn.Literal)()}
			}
			return exprFn(an, bn)
		}
	}
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: func(a, b types.WhereExpr) types.WhereExpr {
				aVal, aOK := a.Literal.(bool)
				bVal, bOK := b.Literal.(bool)
				switch {
				case aOK && bOK:
					return types.WhereExpr{Literal: aVal && bVal}
				case aVal:
					return b
				case bVal:
					return a
				case aOK || bOK:
					return types.WhereExpr{Literal: false}
				default:
					return types.WhereExpr{And: types.WhereExpr2{L: &a, R: &b}}
				}
			},
			OR: func(a, b types.WhereExpr) types.WhereExpr {
				aVal, aOK := a.Literal.(bool)
				bVal, bOK := b.Literal.(bool)
				switch {
				case aOK && bOK:
					return types.WhereExpr{Literal: aVal || bVal}
				case aVal || bVal:
					return types.WhereExpr{Literal: true}
				case aOK:
					return b
				case bOK:
					return a
				default:
					return types.WhereExpr{Or: types.WhereExpr2{L: &a, R: &b}}
				}
			},
			NOT: func(expr types.WhereExpr) types.WhereExpr {
				if val, ok := expr.Literal.(bool); ok {
					return types.WhereExpr{Literal: !val}
				}
				return types.WhereExpr{Not: &expr}
			},
		},
		Functions: map[string]interface{}{
			"equals": binaryPred(predicate.Equals, func(a, b types.WhereExpr) types.WhereExpr {
				return types.WhereExpr{Equals: types.WhereExpr2{L: &a, R: &b}}
			}),
			"contains": binaryPred(predicate.Contains, func(a, b types.WhereExpr) types.WhereExpr {
				return types.WhereExpr{Contains: types.WhereExpr2{L: &a, R: &b}}
			}),
		},
		GetIdentifier: func(fields []string) (interface{}, error) {
			if fields[0] == identifier {
				// TODO: Session events have only one level of attributes. Support for
				// more nested levels may be added when needed for other objects.
				if len(fields) != 2 {
					return nil, trace.BadParameter("only exactly two fields are supported with identifier %q, got %d: %v", identifier, len(fields), fields)
				}
				return types.WhereExpr{Field: fields[1]}, nil
			}
			lit, err := ctx.GetIdentifier(fields)
			return types.WhereExpr{Literal: lit}, trace.Wrap(err)
		},
		GetProperty: func(mapVal, keyVal interface{}) (interface{}, error) {
			mapExpr, mapOK := mapVal.(types.WhereExpr)
			if !mapOK {
				mapExpr = types.WhereExpr{Literal: mapVal}
			}
			keyExpr, keyOK := keyVal.(types.WhereExpr)
			if !keyOK {
				keyExpr = types.WhereExpr{Literal: keyVal}
			}
			if mapExpr.Literal == nil || keyExpr.Literal == nil {
				// TODO: Add support for general WhereExpr.
				return nil, trace.BadParameter("GetProperty is implemented only for literals")
			}
			return GetStringMapValue(mapExpr.Literal, keyExpr.Literal)
		},
	})
}

// NewResourceParser returns a parser made for boolean expressions based on a
// json-serialiable resource. Customized to allow short identifiers common in all
// resources:
//  - `metadata.name` can be referenced with `name` ie: `name == "jenkins"``
//  - `metadata.labels + spec.dynamic_labels` can be referenced with `labels`
//     ie: `labels.env == "prod"`
// All other fields can be referenced by starting expression with identifier `resource`
// followed by the names of the json fields ie: `resource.spec.public_addr`.
func NewResourceParser(resource types.ResourceWithLabels) (BoolPredicateParser, error) {
	p, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
			EQ: func(a interface{}, b interface{}) predicate.BoolPredicate {
				return func() bool {
					switch aval := a.(type) {
					case label:
						bval, ok := b.(string)
						return ok && aval.value == bval
					case string:
						bval, ok := b.(string)
						return ok && aval == bval
					default:
						return false
					}
				}
			},
			NEQ: func(a interface{}, b interface{}) predicate.BoolPredicate {
				return func() bool {
					switch aval := a.(type) {
					case label:
						bval, ok := b.(string)
						return ok && aval.value != bval
					case string:
						bval, ok := b.(string)
						return ok && aval != bval
					default:
						return false
					}
				}
			},
		},
		Functions: map[string]interface{}{
			"equals": func(a interface{}, b interface{}) predicate.BoolPredicate {
				aval, ok := a.(label)
				if !ok {
					return predicate.Equals(a, b)
				}
				bval, ok := b.(string)
				return func() bool {
					return ok && aval.value == bval
				}
			},
			// search allows fuzzy matching against select field values.
			"search": func(searchVals ...string) predicate.BoolPredicate {
				return func() bool {
					return resource.MatchSearch(searchVals)
				}
			},
			// exists checks for an existence of a label by checking
			// if a key exists. Label value are unchecked.
			"exists": func(l label) predicate.BoolPredicate {
				return func() bool {
					return len(l.key) > 0
				}
			},
		},
		GetIdentifier: func(fields []string) (interface{}, error) {
			switch fields[0] {
			case ResourceLabelsIdentifier:
				combinedLabels := resource.GetAllLabels()
				switch {
				// Field length of 1 means the user is using
				// an index expression ie: labels["env"], which the
				// parser will expect a map for lookup in `GetProperty`.
				case len(fields) == 1:
					return labels(combinedLabels), nil
				default:
					key := fields[1]
					val, ok := combinedLabels[key]
					if ok {
						return label{key: key, value: val}, nil
					}
					return label{}, nil
				}

			case ResourceNameIdentifier:
				return resource.GetName(), nil
			case ResourceIdentifier:
				return predicate.GetFieldByTag(resource, teleport.JSON, fields[1:])
			default:
				return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
			}
		},
		GetProperty: func(mapVal, keyVal interface{}) (interface{}, error) {
			m, ok := mapVal.(labels)
			if !ok {
				return GetStringMapValue(mapVal, keyVal)
			}

			key, ok := keyVal.(string)
			if !ok {
				return nil, trace.BadParameter("only string keys are supported")
			}

			val, ok := m[key]
			if ok {
				return label{key: key, value: val}, nil
			}
			return label{}, nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return boolPredicateParser{Parser: p}, nil
}

type label struct {
	key, value string
}

type labels map[string]string
