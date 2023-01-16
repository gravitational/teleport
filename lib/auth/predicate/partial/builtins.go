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

package partial

import (
	"fmt"

	"github.com/aclements/go-z3/z3"
	"github.com/gravitational/trace"
)

func builtinUpper(ctx *z3.Context) (z3.FuncDecl, error) {
	fnUpper := ctx.FuncDeclRec("string_upper", []z3.Sort{ctx.StringSort()}, ctx.StringSort())
	element := ctx.StringConst("string_upper_input")
	zero := ctx.FromInt(0, ctx.IntSort()).(z3.Int)
	one := ctx.FromInt(1, ctx.IntSort()).(z3.Int)

	charUpper := func(char z3.String) z3.String {
		zc := ctx.FromString("z").ToCode()
		ac := ctx.FromString("a").ToCode()
		Ac := ctx.FromString("A").ToCode()

		code := char.ToCode()
		isLower := code.GE(ac).And(code.LE(zc))
		upper := ctx.StringFromCode(code.Add(Ac.Sub(ac)))
		return ctx.If(isLower, upper.AsAST(), char.AsAST()).AsValue().(z3.String)
	}

	rem := fnUpper.Apply(element.Substring(one, element.Length().Sub(one))).(z3.String)

	fnUpper.DefineRec(
		[]z3.Value{element},
		ctx.If(
			element.Length().Eq(zero),
			element.AsAST(),
			charUpper(element.Substring(zero, one)).Concat(rem).AsAST(),
		))

	return fnUpper, nil
}

func builtinLower(ctx *z3.Context) (z3.FuncDecl, error) {
	fnLower := ctx.FuncDeclRec("string_lower", []z3.Sort{ctx.StringSort()}, ctx.StringSort())
	element := ctx.StringConst("string_lower_input")
	zero := ctx.FromInt(0, ctx.IntSort()).(z3.Int)
	one := ctx.FromInt(1, ctx.IntSort()).(z3.Int)

	charUpper := func(char z3.String) z3.String {
		Zc := ctx.FromString("Z").ToCode()
		Ac := ctx.FromString("A").ToCode()
		ac := ctx.FromString("a").ToCode()

		code := char.ToCode()
		isUpper := code.GE(Ac).And(code.LE(Zc))
		upper := ctx.StringFromCode(code.Sub(Ac.Sub(ac)))
		return ctx.If(isUpper, upper.AsAST(), char.AsAST()).AsValue().(z3.String)
	}

	rem := fnLower.Apply(element.Substring(one, element.Length().Sub(one))).(z3.String)

	fnLower.DefineRec(
		[]z3.Value{element},
		ctx.If(
			element.Length().Eq(zero),
			element.AsAST(),
			charUpper(element.Substring(zero, one)).Concat(rem).AsAST(),
		))

	return fnLower, nil
}

func builtinSplit(ctx *z3.Context) (z3.FuncDecl, error) {
	fnSplit := ctx.FuncDeclRec("string_split", []z3.Sort{ctx.StringSort(), ctx.StringSort(), ctx.BoolSort()}, ctx.StringSort())
	input := ctx.StringConst("string_split_input")
	separator := ctx.StringConst("string_split_separator")
	before := ctx.BoolConst("string_split_before")
	zero := ctx.FromInt(0, ctx.IntSort()).(z3.Int)
	one := ctx.FromInt(1, ctx.IntSort()).(z3.Int)
	indexEnd := input.IndexOf(separator, zero)

	applyAfter := func() z3.AST {
		hasSep := indexEnd.GT(zero)
		empty := ctx.FromString("").AsAST()

		return ctx.If(
			hasSep,
			input.Substring(indexEnd.Add(one), input.Length()).AsAST(),
			empty,
		)
	}

	fnSplit.DefineRec(
		[]z3.Value{input, separator, before},
		ctx.If(
			before,
			input.Substring(zero, indexEnd).AsAST(),
			applyAfter(),
		))

	return fnSplit, nil
}

func builtinStringListContains(ctx *z3.Context) (z3.FuncDecl, error) {
	listSort := ctx.SequenceSort(ctx.StringSort())
	fnContains := ctx.FuncDeclRec("string_list_contains", []z3.Sort{listSort, ctx.StringSort()}, ctx.BoolSort())
	list := z3.Sequence(ctx.Const("string_list_contains_list", listSort).(z3.String))
	matcher := ctx.StringConst("string_list_contains_matcher")

	fnContains.DefineRec(
		[]z3.Value{list, matcher},
		list.Contains(matcher).AsAST(),
	)

	return fnContains, nil
}

func builtinFirst(ctx *z3.Context) (z3.FuncDecl, error) {
	return z3.FuncDecl{}, trace.NotImplemented("not implemented")
}

func builtinStringListAppend(ctx *z3.Context) (z3.FuncDecl, error) {
	listSort := ctx.SequenceSort(ctx.StringSort())
	fnAppend := ctx.FuncDeclRec("string_list_append", []z3.Sort{listSort, ctx.StringSort()}, listSort)
	list := ctx.Const("string_list_append_list", listSort).(z3.Sequence)
	element := ctx.StringConst("string_list_append_element")

	unit := ctx.SequenceUnit(element)
	newList := list.Concat(unit)

	fnAppend.DefineRec(
		[]z3.Value{list, element},
		newList.AsAST(),
	)

	return fnAppend, nil
}

func builtinStringListArray(args int) func(*z3.Context) (z3.FuncDecl, error) {
	return func(ctx *z3.Context) (z3.FuncDecl, error) {
		listSort := ctx.SequenceSort(ctx.StringSort())
		paramSorts := make([]z3.Sort, args)
		for i := 0; i < args; i++ {
			paramSorts[i] = ctx.StringSort()
		}

		fnArray := ctx.FuncDeclRec("string_list_array", paramSorts, listSort)
		empty := ctx.SequenceEmpty(listSort)
		params := make([]z3.Value, args)
		sequences := make([]z3.Sequence, args)

		for i := 0; i < args; i++ {
			param := ctx.StringConst(fmt.Sprintf("string_list_array_%d", i))
			params[i] = param
			sequences[i] = ctx.SequenceUnit(param)
		}

		assembled := empty.Concat(sequences...)
		fnArray.DefineRec(
			params,
			assembled.AsAST(),
		)

		return fnArray, nil
	}
}

func builtinReplace(ctx *z3.Context) (z3.FuncDecl, error) {
	return z3.FuncDecl{}, trace.NotImplemented("not implemented")
}

func builtinLenString(ctx *z3.Context) (z3.FuncDecl, error) {
	fnLen := ctx.FuncDeclRec("string_len", []z3.Sort{ctx.StringSort()}, ctx.IntSort())
	input := ctx.StringConst("string_len_input")

	fnLen.DefineRec(
		[]z3.Value{input},
		input.Length().AsAST(),
	)

	return fnLen, nil
}

func builtinLenStringList(ctx *z3.Context) (z3.FuncDecl, error) {
	sort := ctx.SequenceSort(ctx.StringSort())
	fnLen := ctx.FuncDeclRec("string_list_len", []z3.Sort{sort}, ctx.IntSort())
	input := ctx.Const("string_list_len_input", sort)

	fnLen.DefineRec(
		[]z3.Value{input},
		z3.Sequence(input.(z3.String)).Length().AsAST(),
	)

	return fnLen, nil
}

func builtinRegex(ctx *z3.Context) (z3.FuncDecl, error) {
	return z3.FuncDecl{}, trace.NotImplemented("not implemented")
}

func builtinMatches(ctx *z3.Context) (z3.FuncDecl, error) {
	return z3.FuncDecl{}, trace.NotImplemented("not implemented")
}

func builtinContainsRegex(ctx *z3.Context) (z3.FuncDecl, error) {
	return z3.FuncDecl{}, trace.NotImplemented("not implemented")
}
