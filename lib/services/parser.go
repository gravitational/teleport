/*
Copyright 2017 Gravitational, Inc.

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
	"strings"
	"time"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/predicate"
)

// RuleContext specifies context passed to the
// rule processing matcher, and contains information
// about current session, e.g. current user
type RuleContext interface {
	// GetIdentifier returns identifier defined in a context
	GetIdentifier(fields []string) (interface{}, error)
	// String returns human friendly representation of a context
	String() string
}

// NewWhereParser returns standard parser for `where` section in access rules
func NewWhereParser(ctx RuleContext) (predicate.Parser, error) {
	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
		},
		Functions: map[string]interface{}{
			"equals":   predicate.Equals,
			"contains": predicate.Contains,
		},
		GetIdentifier: ctx.GetIdentifier,
		GetProperty:   predicate.GetStringMapValue,
	})
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
	return (&LogAction{ctx: ctx}).Log
}

// LogAction represents action that will emit log entry
// when specified in the actions of a matched rule
type LogAction struct {
	ctx RuleContext
}

// Log logs with specified level and formatting string with arguments
func (l *LogAction) Log(level, format string, args ...interface{}) predicate.BoolPredicate {
	return func() bool {
		ilevel, err := log.ParseLevel(level)
		if err != nil {
			ilevel = log.DebugLevel
		}
		writer := log.StandardLogger().WriterLevel(ilevel)
		writer.Write([]byte(fmt.Sprintf(format, args...)))
		return true
	}
}

// Context is a default rule context used in teleport
type Context struct {
	// User is currently authenticated user
	User User
	// Resource is an optional resource, in case if the rule
	// checks access to the resource
	Resource Resource
}

// String returns user friendly representation of this context
func (ctx *Context) String() string {
	return fmt.Sprintf("user %v, resource: %v", ctx.User, ctx.Resource)
}

const (
	// UserIdentifier represents user registered identifier in the rules
	UserIdentifier = "user"
	// ResourceIdentifier represents resource registered identifer in the rules
	ResourceIdentifier = "resource"
)

// GetIdentifier returns identifier defined in a context
func (ctx *Context) GetIdentifier(fields []string) (interface{}, error) {
	switch fields[0] {
	case UserIdentifier:
		var user User
		if ctx.User == nil {
			user = emptyUser
		} else {
			user = ctx.User
		}
		return predicate.GetFieldByTag(user, teleport.JSON, fields[1:])
	case ResourceIdentifier:
		var resource Resource
		if ctx.Resource == nil {
			resource = emptyResource
		} else {
			resource = ctx.Resource
		}
		return predicate.GetFieldByTag(resource, "json", fields[1:])
	default:
		return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
	}
}

// NewParserFn returns function that creates parser of 'where' section
// in access rules
type NewParserFn func(ctx RuleContext) (predicate.Parser, error)

var whereParser = NewWhereParser
var actionsParser = NewActionsParser

// GetWhereParserFn returns global function that creates where parsers
// this function is used in external tools to override and extend 'where' in rules
func GetWhereParserFn() NewParserFn {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return whereParser
}

// SetWhereParserFn sets global function that creates where parsers
// this function is used in external tools to override and extend 'where' in rules
func SetWhereParserFn(fn NewParserFn) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	whereParser = fn
}

// GetActionsParserFn returns global function that creates where parsers
// this function is used in external tools to override and extend actions in rules
func GetActionsParserFn() NewParserFn {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return actionsParser
}

// SetActionsParserFn sets global function that creates actions  parsers
// this function is used in external tools to override and extend actions in rules
func SetActionsParserFn(fn NewParserFn) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	actionsParser = fn
}

// emptyResource is used when no resource is specified
var emptyResource = &EmptyResource{}

// emptyUser is used when no user is specified
var emptyUser = &UserV2{}

// EmptyResource is used to represent a use case when no resource
// is specified in the rules matcher
type EmptyResource struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
}

// SetExpiry sets expiry time for the object.
func (r *EmptyResource) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *EmptyResource) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets TTL header using realtime clock.
func (r *EmptyResource) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
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
func (r *EmptyResource) GetMetadata() Metadata {
	return r.Metadata
}
