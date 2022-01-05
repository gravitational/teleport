// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "sort"

type packageOutput struct {
	output         []string
	subtestOutput  map[string][]string
	failedSubtests map[string][]string
}

func (pkg *packageOutput) FailedTests() []string {
	result := []string{}
	for testName := range pkg.failedSubtests {
		result = append(result, testName)
	}
	sort.Strings(result)
	return result
}

type outputMap struct {
	packages     map[string]*packageOutput
	actionCounts map[string]int
}

func newOutputMap() *outputMap {
	return &outputMap{
		packages:     make(map[string]*packageOutput),
		actionCounts: make(map[string]int),
	}
}

func (m *outputMap) record(event TestEvent) {
	m.actionCounts[event.Action]++

	var pkgOutput *packageOutput
	var exists bool
	if pkgOutput, exists = m.packages[event.Package]; !exists {
		pkgOutput = &packageOutput{
			subtestOutput:  make(map[string][]string),
			failedSubtests: make(map[string][]string),
		}
		m.packages[event.Package] = pkgOutput
	}

	switch event.Action {
	case actionOutput:
		pkgOutput.output = append(pkgOutput.output, event.Output)
		if event.Test != "" {
			pkgOutput.subtestOutput[event.Test] = append(pkgOutput.subtestOutput[event.Test], event.Output)
		}

	case actionFail:
		// If this is a single test result
		if event.Test != "" {
			pkgOutput.failedSubtests[event.Test] = pkgOutput.subtestOutput[event.Test]
		} else {
			// If this is a package result, we only want to preserve the package output if
			// there are no failed subtests
			if len(pkgOutput.failedSubtests) > 0 {
				pkgOutput.output = nil
			}
		}
		fallthrough

	case actionPass, actionSkip:
		delete(pkgOutput.subtestOutput, event.Test)
	}
}

func (m *outputMap) getPkg(pkgName string) *packageOutput {
	return m.packages[pkgName]
}

func (m *outputMap) deletePkg(pkgName string) {
	delete(m.packages, pkgName)
}
