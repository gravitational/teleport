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

import "github.com/aclements/go-z3/z3"

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
