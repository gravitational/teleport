/*
Copyright 2023 Gravitational, Inc.

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

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadInput(t *testing.T) {
	// var passFailPass and func strToEvents in result_test.go
	expected := strToEvents(t, passFailPass)
	eventChan := make(chan TestEvent)
	errChan := make(chan error)

	actual := []TestEvent{}
	go readInput(strings.NewReader(passFailPass), eventChan, errChan)
	ok := true
	for ok {
		var event TestEvent
		select {
		case err := <-errChan:
			require.NoError(t, err)
		case event, ok = <-eventChan:
			if ok {
				actual = append(actual, event)
			}
		}
	}

	require.Equal(t, expected, actual)
}

func TestReadInputFail(t *testing.T) {
	// var passFailPass and func strToEvents in result_test.go
	eventChan := make(chan TestEvent)
	errChan := make(chan error)

	go readInput(strings.NewReader(passFailPass+"bad json data\noh no\n"), eventChan, errChan)
	ok := true
	var err error
	for ok {
		select {
		case err = <-errChan:
		case _, ok = <-eventChan:
		}
	}

	require.Error(t, err)
}
