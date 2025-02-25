/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression

import (
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/trace"
)

// Evaluate the given boolean expression against the given attributes.
func Evaluate(expr string, attrs *workloadidentityv1.Attrs) (bool, error) {
	e, err := booleanExpressionParser.Parse(expr)
	if err != nil {
		return false, trace.Wrap(err, "parsing expression: %s", expr)
	}

	rsp, err := e.Evaluate(attrs)
	if err != nil {
		return false, trace.Wrap(err, "evaluating expression: %s", expr)
	}

	if result, ok := rsp.(bool); ok {
		return result, nil
	}

	return false, trace.Errorf("expression evaluated to %T instead of boolean: %s", rsp, expr)
}

// Validate the given boolean expression is syntactically correct, and does not
// refer to any unknown attributes.
func Validate(expr string) error {
	_, err := booleanExpressionParser.Parse(expr)
	return err
}
