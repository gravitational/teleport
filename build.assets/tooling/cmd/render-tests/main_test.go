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
