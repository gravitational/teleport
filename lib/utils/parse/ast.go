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

package parse

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// Node is a node in the AST.
type Node interface {
	// Str returns the string value if the node is StrNode.
	Str() *string
	// Var returns the only variable in the AST (if any).
	Var() *VarNode
	// Eval evaluates the AST given the variable value (if any).
	Eval(varValue *string) (string, error)
}

// StrNode encodes a string literal.
type StrNode struct {
	value string
}

// VarNode encodes a variable expression with the form "namespace.name".
type VarNode struct {
	namespace string
	name      string
}

// EmailLocalNode encodes an email local expression with the form "email.local(expr)".
type EmailLocalNode struct {
	email Node
}

// RegexpReplaceNode encodes an email local expression with the form "regexp.replace(expr, string, string)".
type RegexpReplaceNode struct {
	source      Node
	re          *regexp.Regexp
	replacement string
}

// Namespace returns the variable namespace.
func (e *VarNode) Namespace() string {
	return e.namespace
}

// Namespace returns the variable name.
func (e *VarNode) Name() string {
	return e.name
}

// Str returns the string value if the node is StrNode.
func (e *StrNode) Str() *string {
	return &e.value
}

// Str returns the string value if the node is StrNode.
func (e *VarNode) Str() *string {
	return nil
}

// Str returns the string value if the node is StrNode.
func (e *EmailLocalNode) Str() *string {
	return nil
}

// Str returns the string value if the node is StrNode.
func (e *RegexpReplaceNode) Str() *string {
	return nil
}

// Var returns the only variable in the AST (if any).
func (e *StrNode) Var() *VarNode {
	return nil
}

// Var returns the only variable in the AST (if any).
func (e *VarNode) Var() *VarNode {
	return e
}

// Var returns the only variable in the AST (if any).
func (e *EmailLocalNode) Var() *VarNode {
	return e.email.Var()
}

// Var returns the only variable in the AST (if any).
func (e *RegexpReplaceNode) Var() *VarNode {
	return e.source.Var()
}

// Eval evaluates the StrNode, which is always itself.
func (e *StrNode) Eval(_ *string) (string, error) {
	return e.value, nil
}

// Eval evaluates the VarNode given the variable value (if any).
func (e *VarNode) Eval(varValue *string) (string, error) {
	if varValue == nil {
		return "", trace.Errorf("variable value is nil. this is a bug!")
	}
	return *varValue, nil
}

// Eval evaluates the EmailLocalNode given the variable value (if any).
func (e *EmailLocalNode) Eval(varValue *string) (string, error) {
	email, err := e.email.Eval(varValue)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if email == "" {
		return "", trace.BadParameter("email address is empty")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", trace.BadParameter("failed to parse email address %q: %q", email, err)
	}
	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 {
		return "", trace.BadParameter("could not find local part in email address %q", addr.Address)
	}
	return parts[0], nil
}

// Eval evaluates the RegexpReplaceNode given the variable value (if any).
func (e *RegexpReplaceNode) Eval(varValue *string) (string, error) {
	in, err := e.source.Eval(varValue)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// filter out inputs which do not match the regexp at all
	if !e.re.MatchString(in) {
		return "", nil
	}
	return e.re.ReplaceAllString(in, e.replacement), nil
}

// String is the string representation of StrNode.
func (e *StrNode) String() string {
	return fmt.Sprintf("%q", e.value)
}

// String is the string representation of VarNode.
func (e *VarNode) String() string {
	return fmt.Sprintf("%s.%s", e.namespace, e.name)
}

// String is the string representation of EmailLocalNode.
func (e *EmailLocalNode) String() string {
	return fmt.Sprintf("email.local(%s)", e.email)
}

// String is the string representation of RegexpReplaceNode.
func (e *RegexpReplaceNode) String() string {
	return fmt.Sprintf("regexp.replace(%s, %q, %q)", e.source, e.re, e.replacement)
}
