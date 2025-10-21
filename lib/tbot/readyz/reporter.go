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

package readyz

import (
	"sync"

	"github.com/jonboulle/clockwork"
)

// Reporter can be used by a service to report its status.
type Reporter interface {
	// Report the service's status.
	Report(status Status)

	// ReportReason reports the service's status including reason/description text.
	ReportReason(status Status, reason string)
}

type reporter struct {
	mu     *sync.Mutex
	status *ServiceStatus
	clock  clockwork.Clock
	notify func()
}

func (r *reporter) Report(status Status) {
	r.ReportReason(status, "")
}

func (r *reporter) ReportReason(status Status, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.status.Status = status
	r.status.Reason = reason

	updatedAt := r.clock.Now()
	r.status.UpdatedAt = &updatedAt

	r.notify()
}

// NoopReporter returns a no-op Reporter that can be used when no real reporter
// is available (e.g. in tests).
func NoopReporter() Reporter {
	return noopReporter{}
}

type noopReporter struct{}

func (noopReporter) Report(Status)               {}
func (noopReporter) ReportReason(Status, string) {}
