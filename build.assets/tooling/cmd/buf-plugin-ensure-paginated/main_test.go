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
package main

import (
	"testing"

	"buf.build/go/bufplugin/check/checktest"
)

func TestSpec(t *testing.T) {
	t.Parallel()
	checktest.SpecTest(t, paginationSpec)
}

func TestSimple(t *testing.T) {
	t.Parallel()

	checktest.CheckTest{
		Request: &checktest.RequestSpec{
			Files: &checktest.ProtoFileSpec{
				DirPaths:  []string{"testdata"},
				FilePaths: []string{"correct.proto"},
			},
			RuleIDs: []string{paginationRuleID},
		},
		Spec:                paginationSpec,
		ExpectedAnnotations: []checktest.ExpectedAnnotation{},
	}.Run(t)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	checktest.CheckTest{
		Request: &checktest.RequestSpec{
			Files: &checktest.ProtoFileSpec{
				DirPaths:  []string{"testdata"},
				FilePaths: []string{"bad.proto"},
			},
			RuleIDs: []string{paginationRuleID},
		},
		Spec: paginationSpec,
		ExpectedAnnotations: []checktest.ExpectedAnnotation{
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   35,
					StartColumn: 0,
					EndLine:     37,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   44,
					StartColumn: 0,
					EndLine:     46,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   59,
					StartColumn: 0,
					EndLine:     61,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   64,
					StartColumn: 0,
					EndLine:     65,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   64,
					StartColumn: 0,
					EndLine:     65,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   67,
					StartColumn: 0,
					EndLine:     69,
					EndColumn:   1,
				},
			},
		},
	}.Run(t)
}

func TestConfig(t *testing.T) {
	t.Parallel()

	checktest.CheckTest{
		Request: &checktest.RequestSpec{
			Files: &checktest.ProtoFileSpec{
				DirPaths:  []string{"testdata"},
				FilePaths: []string{"config.proto"},
			},
			RuleIDs: []string{paginationRuleID},
			Options: map[string]any{
				sizeNamesKey:  []string{"max"},
				tokenNamesKey: []string{"token"},
				nextNamesKey:  []string{"next"},
			},
		},
		Spec:                paginationSpec,
		ExpectedAnnotations: []checktest.ExpectedAnnotation{},
	}.Run(t)
}

func TestRepeatFields(t *testing.T) {
	t.Parallel()

	checktest.CheckTest{
		Request: &checktest.RequestSpec{
			Files: &checktest.ProtoFileSpec{
				DirPaths:  []string{"testdata"},
				FilePaths: []string{"repeat.proto"},
			},
			RuleIDs: []string{paginationRuleID},
			Options: map[string]any{
				checkRepeatedKey: true,
			},
		},
		Spec: paginationSpec,
		ExpectedAnnotations: []checktest.ExpectedAnnotation{
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   32,
					StartColumn: 0,
					EndLine:     33,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   32,
					StartColumn: 0,
					EndLine:     33,
					EndColumn:   1,
				},
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   35,
					StartColumn: 0,
					EndLine:     37,
					EndColumn:   1,
				},
			},
		},
	}.Run(t)
}
