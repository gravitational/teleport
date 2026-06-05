/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package testutils

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// recorder captures FailReporter calls for assertion.
type recorder struct {
	msgs []string
}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func (r *recorder) joined() string {
	return strings.Join(r.msgs, "\n")
}

func TestRunWithTimeout_NoFailureOnFastCompletion(t *testing.T) {
	r := &recorder{}
	RunWithTimeout(r, time.Second, func() {})

	require.Empty(t, r.msgs)
}

func TestRunWithTimeout_ReportsPanic(t *testing.T) {
	r := &recorder{}
	RunWithTimeout(r, time.Second, func() {
		panic("boom")
	})

	require.Len(t, r.msgs, 1)
	require.Contains(t, r.joined(), "panic: boom")
}

func TestRunWithTimeout_ReportsTimeout(t *testing.T) {
	r := &recorder{}
	RunWithTimeout(r, 50*time.Millisecond, func() {
		<-make(chan struct{}) // block forever so the timeout must fire (no wasted wall-clock sleep)
	})

	require.Len(t, r.msgs, 1)
	require.Contains(t, r.joined(), "did not return within")
}
