/*
Copyright 2014-2018 Vulcand Authors

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

// package builder is used to construct predicate
// expressions using builder functions.
package builder

import (
	"fmt"
	"strings"
)

// Expr is an expression builder,
// used to create expressions in rules definitions
type Expr interface {
	// String serializes expression into format parsed by rules engine
	// (golang based syntax)
	String() string
}

// IdentiferExpr is identifer expression
type IdentifierExpr string

// String serializes identifer expression into format parsed by rules engine
func (i IdentifierExpr) String() string {
	return string(i)
}

// Identifer returns identifier expression
func Identifier(v string) IdentifierExpr {
	return IdentifierExpr(v)
}

// String returns string expression
func String(v string) StringExpr {
	return StringExpr(v)
}

// StringExpr is a string expression
type StringExpr string

func (s StringExpr) String() string {
	return fmt.Sprintf("%q", string(s))
}

// StringsExpr is a slice of strings
type StringsExpr []string

func (s StringsExpr) String() string {
	var out []string
	for _, val := range s {
		out = append(out, fmt.Sprintf("%q", val))
	}
	return strings.Join(out, ",")
}

// Equals returns equals expression
func Equals(left, right Expr) EqualsExpr {
	return EqualsExpr{Left: left, Right: right}
}

// EqualsExpr constructs function expression used in rules specifications
// that checks if one value is equal to another
// e.g. equals("a", "b") where Left is "a" and right is "b"
type EqualsExpr struct {
	// Left is a left argument of Equals expression
	Left Expr
	// Value to check
	Right Expr
}

// String returns function call expression used in rules
func (i EqualsExpr) String() string {
	return fmt.Sprintf("equals(%v, %v)", i.Left, i.Right)
}

// Not returns ! expression
func Not(expr Expr) NotExpr {
	return NotExpr{Expr: expr}
}

// NotExpr constructs function expression used in rules specifications
// that negates the result of the boolean predicate
// e.g. ! equals"a", "b") where Left is "a" and right is "b"
type NotExpr struct {
	// Expr is an expression to negate
	Expr Expr
}

// String returns function call expression used in rules
func (n NotExpr) String() string {
	return fmt.Sprintf("!%v", n.Expr)
}

// Contains returns contains function call expression
func Contains(a, b Expr) ContainsExpr {
	return ContainsExpr{Left: a, Right: b}
}

// ContainsExpr constructs function expression used in rules specifications
// that checks if one value contains the other, e.g.
// contains([]string{"a"}, "b") where left is []string{"a"} and right is "b"
type ContainsExpr struct {
	// Left is a left argument of Contains expression
	Left Expr
	// Right is a right argument of Contains expression
	Right Expr
}

// String rturns function call expression used in rules
func (i ContainsExpr) String() string {
	return fmt.Sprintf("contains(%v, %v)", i.Left, i.Right)
}

// And returns && expression
func And(left, right Expr) AndExpr {
	return AndExpr{
		Left:  left,
		Right: right,
	}
}

// AndExpr returns && expression
type AndExpr struct {
	// Left is a left argument of && operator expression
	Left Expr
	// Right is a right argument of && operator expression
	Right Expr
}

// String returns expression text used in rules
func (a AndExpr) String() string {
	return fmt.Sprintf("%v && %v", a.Left, a.Right)
}

// Or returns || expression
func Or(left, right Expr) OrExpr {
	return OrExpr{
		Left:  left,
		Right: right,
	}
}

// OrExpr returns || expression
type OrExpr struct {
	// Left is a left argument of || operator expression
	Left Expr
	// Right is a right argument of || operator expression
	Right Expr
}

// String returns expression text used in rules
func (a OrExpr) String() string {
	return fmt.Sprintf("%v || %v", a.Left, a.Right)
}
