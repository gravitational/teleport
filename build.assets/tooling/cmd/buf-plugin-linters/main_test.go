// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
					StartLine:   30,
					StartColumn: 2,
					EndLine:     30,
					EndColumn:   86,
				},
				Message: "repeated fields expected for RPC names starting with: \"List\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   33,
					StartColumn: 0,
					EndLine:     35,
					EndColumn:   1,
				},
				Message: "\"ListMissingPageReqFoosRequest\" taken by \"bad.FooService.ListMissingPageReqFoos\" is missing page token field name: \"page_token\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   42,
					StartColumn: 0,
					EndLine:     44,
					EndColumn:   1,
				},
				Message: "\"ListMissingPageSizeFoosRequest\" taken by \"bad.FooService.ListMissingPageSizeFoos\" is missing page token field name: \"page_token\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   57,
					StartColumn: 0,
					EndLine:     59,
					EndColumn:   1,
				},
				Message: "\"ListMissingNextpageFoosResponse\" returned by \"bad.FooService.ListMissingNextpageFoos\" is missing next page token field name: \"next_page_token\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   62,
					StartColumn: 0,
					EndLine:     63,
					EndColumn:   1,
				},
				Message: "\"ListMissingAllFoosRequest\" taken by \"bad.FooService.ListMissingAllFoos\" is missing page size field name: \"page_size\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   62,
					StartColumn: 0,
					EndLine:     63,
					EndColumn:   1,
				},
				Message: "\"ListMissingAllFoosRequest\" taken by \"bad.FooService.ListMissingAllFoos\" is missing page token field name: \"page_token\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "bad.proto",
					StartLine:   65,
					StartColumn: 0,
					EndLine:     67,
					EndColumn:   1,
				},
				Message: "\"ListMissingAllFoosResponse\" returned by \"bad.FooService.ListMissingAllFoos\" is missing next page token field name: \"next_page_token\" (RFD-0153)",
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
		},
		Spec: paginationSpec,
		ExpectedAnnotations: []checktest.ExpectedAnnotation{
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   30,
					StartColumn: 0,
					EndLine:     31,
					EndColumn:   1,
				},
				Message: "\"GetFoosRequest\" taken by \"repeat.FooService.GetFoos\" is missing page size field name: \"page_size\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   30,
					StartColumn: 0,
					EndLine:     31,
					EndColumn:   1,
				},
				Message: "\"GetFoosRequest\" taken by \"repeat.FooService.GetFoos\" is missing page token field name: \"page_token\" (RFD-0153)",
			},
			{
				RuleID: paginationRuleID,
				FileLocation: &checktest.ExpectedFileLocation{
					FileName:    "repeat.proto",
					StartLine:   33,
					StartColumn: 0,
					EndLine:     35,
					EndColumn:   1,
				},
				Message: "\"GetFoosResponse\" returned by \"repeat.FooService.GetFoos\" is missing next page token field name: \"next_page_token\" (RFD-0153)",
			},
		},
	}.Run(t)
}
