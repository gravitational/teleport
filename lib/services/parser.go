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

package services

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/predicate"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/typical"
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

// predicateAllEndWith is a custom function to test if a string ends with a
// particular suffix. If given a `[]string` as the first argument, all values
// must have the given suffix (2nd argument).
func predicateAllEndWith(a interface{}, b interface{}) predicate.BoolPredicate {
	return func() bool {
		// bval is the suffix and must always be a plain string.
		bval, ok := b.(string)
		if !ok {
			return false
		}

		switch aval := a.(type) {
		case string:
			return strings.HasSuffix(aval, bval)
		case []string:
			for _, val := range aval {
				if !strings.HasSuffix(val, bval) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
}

// predicateAllEqual is a custom function to test if all entries in a []string
// are equal to a certain value. This is primarily useful for comparing string
// fields that are only expected to contain a single, specific value.
func predicateAllEqual(a interface{}, b interface{}) predicate.BoolPredicate {
	return func() bool {
		// bval is the suffix and must always be a plain string.
		bval, ok := b.(string)
		if !ok {
			return false
		}

		switch aval := a.(type) {
		case string:
			return aval == bval
		case []string:
			for _, val := range aval {
				if val != bval {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
}

// predicateIsSubset determines if the first parameter is contained within the
// variadic args. The first argument may either by `string` or `[]string`, and
// the variadic args may only be `string`.
func predicateIsSubset(a interface{}, b ...interface{}) predicate.BoolPredicate {
	return func() bool {
		// Populate the set.
		set := map[string]bool{}
		for _, bval := range b {
			s, ok := bval.(string)
			if !ok {
				return false
			}

			set[s] = true
		}

		switch aval := a.(type) {
		case string:
			return set[aval]
		case []string:
			for _, v := range aval {
				if !set[v] {
					return false
				}
			}

			return true
		default:
			return false
		}
	}
}

// NewWhereParser returns standard parser for `where` section in access rules.
func NewWhereParser(ctx RuleContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
		},
		Functions: map[string]interface{}{
			"equals":       predicate.Equals,
			"contains":     predicate.Contains,
			"all_end_with": predicateAllEndWith,
			"all_equal":    predicateAllEqual,
			"is_subset":    predicateIsSubset,
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
	User UserState
	// Resource is an optional resource, in case if the rule
	// checks access to the resource
	Resource types.Resource
	// Session is an optional session.end or windows.desktop.session.end event.
	// These events hold information about session recordings.
	Session events.AuditEvent
	// SSHSession is an optional (active) SSH session.
	SSHSession *session.Session
	// HostCert is an optional host certificate.
	HostCert *HostCertContext
	// SessionTracker is an optional session tracker, in case if the rule checks access to the tracker.
	SessionTracker types.SessionTracker
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
	// ResourceNameIdentifier refers to two different fields depending on the kind of resource:
	//   - KindNode will refer to its resource.spec.hostname field
	//   - All other kinds will refer to its resource.metadata.name field
	// It refers to two different fields because the way this shorthand is being used,
	// implies it will return the name of the resource where users identifies nodes
	// by its hostname and all other resources that can be `ls` queried is identified
	// by its metadata name.
	ResourceNameIdentifier = "name"
	// SessionIdentifier refers to a session (recording) in the rules.
	SessionIdentifier = "session"
	// SSHSessionIdentifier refers to an (active) SSH session in the rules.
	SSHSessionIdentifier = "ssh_session"
	// ImpersonateRoleIdentifier is a role to impersonate
	ImpersonateRoleIdentifier = "impersonate_role"
	// ImpersonateUserIdentifier is a user to impersonate
	ImpersonateUserIdentifier = "impersonate_user"
	// HostCertIdentifier refers to a host certificate being created.
	HostCertIdentifier = "host_cert"
	// SessionTrackerIdentifier refers to a session tracker in the rules.
	SessionTrackerIdentifier = "session_tracker"
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
		var user UserState
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
		var session events.AuditEvent = &events.SessionEnd{}
		switch ctx.Session.(type) {
		case *events.SessionEnd, *events.WindowsDesktopSessionEnd:
			session = ctx.Session
		}
		return predicate.GetFieldByTag(session, teleport.JSON, fields[1:])
	case SSHSessionIdentifier:
		// Do not expose the original session.Session, instead transform it into a
		// ctxSession so the exposed fields match our desired API.
		return predicate.GetFieldByTag(toCtxSession(ctx.SSHSession), teleport.JSON, fields[1:])
	case HostCertIdentifier:
		var hostCert *HostCertContext
		if ctx.HostCert == nil {
			hostCert = emptyHostCert
		} else {
			hostCert = ctx.HostCert
		}
		return predicate.GetFieldByTag(hostCert, teleport.JSON, fields[1:])
	case SessionTrackerIdentifier:
		return predicate.GetFieldByTag(toCtxTracker(ctx.SessionTracker), teleport.JSON, fields[1:])
	default:
		return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
	}
}

// ctxSession represents the public contract of a session.Session, as exposed
// to a Context rule.
// See RFD 82: https://github.com/gravitational/teleport/blob/master/rfd/0082-session-tracker-resource-rbac.md
type ctxTracker struct {
	SessionID    string   `json:"session_id"`
	Kind         string   `json:"kind"`
	Participants []string `json:"participants"`
	State        string   `json:"state"`
	Hostname     string   `json:"hostname"`
	Address      string   `json:"address"`
	Login        string   `json:"login"`
	Cluster      string   `json:"cluster"`
	KubeCluster  string   `json:"kube_cluster"`
	HostUser     string   `json:"host_user"`
	HostRoles    []string `json:"host_roles"`
}

func toCtxTracker(t types.SessionTracker) ctxTracker {
	if t == nil {
		return ctxTracker{}
	}

	getParticipants := func(s types.SessionTracker) []string {
		participants := s.GetParticipants()
		names := make([]string, len(participants))
		for i, participant := range participants {
			names[i] = participant.User
		}

		return names
	}

	getHostRoles := func(s types.SessionTracker) []string {
		policySets := s.GetHostPolicySets()
		roles := make([]string, len(policySets))
		for i, policySet := range policySets {
			roles[i] = policySet.Name
		}

		return roles
	}

	return ctxTracker{
		SessionID:    t.GetSessionID(),
		Kind:         t.GetKind(),
		Participants: getParticipants(t),
		State:        string(t.GetState()),
		Hostname:     t.GetHostname(),
		Address:      t.GetAddress(),
		Login:        t.GetLogin(),
		Cluster:      t.GetClusterName(),
		KubeCluster:  t.GetKubeCluster(),
		HostUser:     t.GetHostUser(),
		HostRoles:    getHostRoles(t),
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

// HostCertContext is used to evaluate the `where` condition on a `host_cert`
// pseudo-resource. These resources only exist for RBAC purposes and do not
// exist in the database.
type HostCertContext struct {
	// HostID is the host ID in the cert request.
	HostID string `json:"host_id"`
	// NodeName is the node name in the cert request.
	NodeName string `json:"node_name"`
	// Principals is the list of requested certificate principals.
	Principals []string `json:"principals"`
	// ClusterName is the name of the cluster for which the certificate should
	// be issued.
	ClusterName string `json:"cluster_name"`
	// Role is the name of the Teleport role for which the cert should be
	// issued.
	Role types.SystemRole `json:"role"`
	// TTL is the requested certificate TTL.
	TTL time.Duration `json:"ttl"`
}

// emptyResource is used when no resource is specified
var emptyResource = &EmptyResource{}

// emptyUser is used when no user is specified
var emptyUser = &types.UserV2{}

// emptyHostCert is an empty host certificate used when no host cert is
// specified
var emptyHostCert = &HostCertContext{}

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

// GetRevision returns the revision
func (r *EmptyResource) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *EmptyResource) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
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
		return false, trace.BadParameter("expected boolean predicate, got unsupported type: %T", ifn)
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

// NewResourceExpression returns a [typical.Expression] that is to be evaluated against a
// [types.ResourceWithLabels]. It is customized to allow short identifiers common in all
// resources:
//   - shorthand `name` refers to `resource.spec.hostname` for node resources, or it refers
//     to `resource.metadata.name` for all other resources eg: `name == "app-name-jenkins"`
//   - shorthand `labels` refers to resource `resource.metadata.labels + resource.spec.dynamic_labels`
//     eg: `labels.env == "prod"`
//
// All other fields can be referenced by starting expression with identifier `resource`
// followed by the names of the json fields ie: `resource.spec.public_addr`.
func NewResourceExpression(expression string) (typical.Expression[types.ResourceWithLabels, bool], error) {
	parser, err := typical.NewParser[types.ResourceWithLabels, bool](typical.ParserSpec[types.ResourceWithLabels]{
		Variables: map[string]typical.Variable{
			"resource.metadata.labels": typical.DynamicVariable(func(r types.ResourceWithLabels) (map[string]string, error) {
				return r.GetStaticLabels(), nil
			}),
			"resource.metadata.name": typical.DynamicVariable(func(r types.ResourceWithLabels) (string, error) {
				return r.GetName(), nil
			}),
			"labels": typical.DynamicMapFunction(func(r types.ResourceWithLabels, key string) (string, error) {
				val, _ := r.GetLabel(key)
				return val, nil
			}),
			"name": typical.DynamicVariable(func(r types.ResourceWithLabels) (string, error) {
				// For nodes, the resource "name" that user expects is the
				// nodes hostname, not its UUID. Currently, for other resources,
				// the metadata.name returns the name as expected.
				if server, ok := r.(types.Server); ok {
					return server.GetHostname(), nil
				}

				return r.GetName(), nil
			}),
		},
		Functions: map[string]typical.Function{
			"hasPrefix": typical.BinaryFunction[types.ResourceWithLabels](func(s, suffix string) (bool, error) {
				return strings.HasPrefix(s, suffix), nil
			}),
			"equals": typical.BinaryFunction[types.ResourceWithLabels](func(a, b string) (bool, error) {
				return strings.Compare(a, b) == 0, nil
			}),
			"search": typical.UnaryVariadicFunctionWithEnv(func(r types.ResourceWithLabels, v ...string) (bool, error) {
				return r.MatchSearch(v), nil
			}),
			"exists": typical.UnaryFunction[types.ResourceWithLabels](func(value string) (bool, error) {
				return value != "", nil
			}),
			"split": typical.BinaryFunction[types.ResourceWithLabels](func(value string, delimiter string) ([]string, error) {
				return strings.Split(value, delimiter), nil
			}),
			"contains": typical.BinaryFunction[types.ResourceWithLabels](func(list []string, value string) (bool, error) {
				return slices.Contains(list, value), nil
			}),
		},
		GetUnknownIdentifier: func(env types.ResourceWithLabels, fields []string) (any, error) {
			if fields[0] == ResourceIdentifier {
				if f, err := predicate.GetFieldByTag(env, teleport.JSON, fields[1:]); err == nil {
					return f, nil
				}
			}

			identifier := strings.Join(fields, ".")
			return nil, trace.BadParameter("identifier %s is not defined", identifier)
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	expr, err := parser.Parse(expression)
	return expr, trace.Wrap(err)
}
