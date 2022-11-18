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

type astNode interface {
	predicateGuard()
}

type eqString struct {
	value string
}

func (*eqString) predicateGuard() {}

type eqBool struct {
	value bool
}

func (*eqBool) predicateGuard() {}

type eqInt struct {
	value int
}

func (*eqInt) predicateGuard() {}

type eqEq struct {
	left  astNode
	right astNode
}

func (*eqEq) predicateGuard() {}

type eqNot struct {
	inner astNode
}

func (*eqNot) predicateGuard() {}

type eqOr struct {
	left  astNode
	right astNode
}

func (*eqOr) predicateGuard() {}

type eqAnd struct {
	left  astNode
	right astNode
}

func (*eqAnd) predicateGuard() {}

type eqXor struct {
	left  astNode
	right astNode
}

func (*eqXor) predicateGuard() {}

type eqLt struct {
	left  astNode
	right astNode
}

func (*eqLt) predicateGuard() {}

type eqLeq struct {
	left  astNode
	right astNode
}

func (*eqLeq) predicateGuard() {}

type eqIndex struct {
	inner astNode
	index astNode
}

func (*eqIndex) predicateGuard() {}

type eqSelector struct {
	inner astNode
	field string
}

func (*eqSelector) predicateGuard() {}

type eqIdent struct {
	name string
}

func (*eqIdent) predicateGuard() {}

type eqConcat struct {
	left  astNode
	right astNode
}

func (*eqConcat) predicateGuard() {}

type eqSplit struct {
	inner astNode
	sep   astNode
}

func (*eqSplit) predicateGuard() {}

type eqArray struct {
	values []astNode
}

func (*eqArray) predicateGuard() {}

type eqUpper struct {
	inner astNode
}

func (*eqUpper) predicateGuard() {}

type eqLower struct {
	inner astNode
}

func (*eqLower) predicateGuard() {}

type eqAppend struct {
	list  astNode
	value astNode
}

func (*eqAppend) predicateGuard() {}

type eqContains struct {
	within astNode
	sub    astNode
}

func (*eqContains) predicateGuard() {}

type eqReplace struct {
	inner astNode
	from  astNode
	to    astNode
}

func (*eqReplace) predicateGuard() {}

type eqMatches struct {
	target astNode
	regex  astNode
}

func (*eqMatches) predicateGuard() {}

type eqMatchesAny struct {
	target astNode
	regex  astNode
}

func (*eqMatchesAny) predicateGuard() {}

type eqLen struct {
	inner astNode
}

func (*eqLen) predicateGuard() {}

type eqGetOrEmpty struct {
	inner astNode
	index astNode
}

func (*eqGetOrEmpty) predicateGuard() {}

type eqMapInsert struct {
	ma    astNode
	key   astNode
	value astNode
}

func (*eqMapInsert) predicateGuard() {}

type eqMapRemove struct {
	ma  astNode
	key astNode
}

func (*eqMapRemove) predicateGuard() {}
