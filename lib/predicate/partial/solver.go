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
	"context"
	"go/parser"
	"sync"

	"github.com/aclements/go-z3/z3"
	"github.com/gravitational/trace"
)

var cachedSolvers = &sync.Pool{New: func() any { return NewSolver() }}

// GetCachedSolver returns a solver from a global cache or creates a new one if the cache is empty.
// One must call PutCachedSolver to return the solver to the cache once it is no longer needed.
func GetCachedSolver() *Solver {
	return cachedSolvers.Get().(*Solver)
}

// PutCachedSolver returns a solver instance to the global cache.
func PutCachedSolver(solver *Solver) {
	cachedSolvers.Put(solver)
}

// Solver is a solver instance which can be used to solve partial predicate expressions.
// This is rather expensive to create so it should preferably be reused when possible.
type Solver struct {
	def    *z3.Context
	solver *z3.Solver
}

// NewSolver creates a new solver instance.
func NewSolver() *Solver {
	config := z3.NewContextConfig()
	def := z3.NewContext(config)
	solver := z3.NewSolver(def)
	return &Solver{def, solver}
}

// Constraint represents a predicate expression which must be satisfied to either true or false
type Constraint struct {
	// Allow is true if the expression must be satisfies to true, false otherwise.
	Allow bool

	// Expr is a predicate expression.
	Expr string
}

// PartialSolveForAll solves a given set of partial predicate expression for all possible values of the given identifier.
// The expressions must have a boolean output type and the type of the unknown identifier must be specified and be shared across all expressions.
// On timeout, no solutions are returned.
func (s *Solver) PartialSolveForAll(ctx context.Context, constraints []Constraint, resolveIdentifier Resolver, querying string, to Type, maxSolutions int) ([]z3.Value, error) {
	outCh := make(chan []z3.Value, 1)
	errCh := make(chan error, 1)

	// Run the solver in a separate goroutine to allow the caller to handle timeouts.
	go func() {
		defer close(outCh)
		defer close(errCh)

		out, err := s.partialSolveForAllImpl(constraints, resolveIdentifier, querying, to, maxSolutions)
		if err != nil {
			errCh <- err
			return
		}

		outCh <- out
	}()

	select {
	case out := <-outCh:
		return out, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		s.def.Interrupt()
		// wait for the other goroutine to cleanup
		<-outCh
		return nil, trace.LimitExceeded("timeout")
	}
}

// partialSolveForAllImpl is the implementation of PartialSolveForAll that runs logic directly.
func (s *Solver) partialSolveForAllImpl(constraints []Constraint, resolveIdentifier Resolver, querying string, to Type, maxSolutions int) (out []z3.Value, err error) {
	// create a context struct which is used to share various information obtained during lowering
	ctx := &ctx{s.def, s.solver, make(map[string]z3.Value), z3.Sort{}, resolveIdentifier}

	// reset the state of the Z3 solver on exit so that it can be reused
	defer ctx.solver.Reset()

	// create a Z3 identifier for the unknown value
	switch to {
	case TypeInt:
		ctx.unkTy = s.def.IntSort()
	case TypeBool:
		ctx.unkTy = s.def.BoolSort()
	case TypeString:
		ctx.unkTy = s.def.StringSort()
	default:
		return nil, trace.BadParameter("unsupported output type %v", to)
	}

	for _, constraint := range constraints {
		// parse the predicate expression into a Go AST
		ast, err := parser.ParseExpr(constraint.Expr)
		if err != nil {
			return nil, err
		}

		// lower the Go AST into a Z3 AST, see the lower godoc for more details
		cond, err := lower(ctx, ast)
		if err != nil {
			return nil, err
		}

		// assert the expression must evaluate to a boolean
		boolCond, ok := cond.(z3.Bool)
		if !ok {
			return nil, trace.BadParameter("predicate must evaluate to a boolean value")
		}

		// if the constraint is of form deny, invert the condition
		if !constraint.Allow {
			boolCond = boolCond.Not()
		}

		// assert the boolean must be equal to true
		ctx.solver.Assert(boolCond)
	}

	// defer a recover here to catch any panics from Z3, this can happen when we call call Interrupt()
	// on a timeout. The bindings have this unfortunate behaviour but this seems like a reasonable fix.
	// We don't need to do any additional cleanup here, the defer above will reset the solver state to a usable form.
	defer func() {
		if err := recover(); err != nil {
			err = trace.Errorf("panic from Z3, likely a timeout: %v", err)
		}
	}()

	// retrieve all possible values for the unknown identifier
	for {
		// solve the model
		sat, err := ctx.solver.Check()
		if err != nil {
			return nil, err
		}

		// if the model is unsatisfiable, we are done
		if !sat {
			return out, nil
		}

		// retrieve the model containing the output value
		model := ctx.solver.Model()

		// retrieve the value of the unknown identifier, this will error if the identifier is not found within the expression
		val, ok := ctx.idents[querying]
		if !ok {
			return nil, trace.NotFound("identifier %v not found", querying)
		}

		// retrieve the value of the unknown identifier from the model
		last := model.Eval(val, true)
		out = append(out, last)

		// assert that the value of the next possible is not equal to the current value, this allows us to retrieve all possible values
		neq := ctx.def.Distinct(val, last)
		ctx.solver.Assert(neq)

		// if we have reached the maximum number of solutions, we are done
		if len(out) == maxSolutions {
			return out, nil
		}
	}
}
