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

import "sync"

// NewRegistry returns a Registry to track the health of tbot's services.
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*ServiceStatus),
	}
}

// Registry tracks the status/health of tbot's services.
type Registry struct {
	mu       sync.Mutex
	services map[string]*ServiceStatus
}

// AddService adds a service to the registry so that its health will be reported
// from our readyz endpoints. It returns a Reporter the service can use to report
// status changes.
func (r *Registry) AddService(name string) Reporter {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.services[name]
	if !ok {
		status = &ServiceStatus{}
		r.services[name] = status
	}
	return &reporter{
		mu:     &r.mu,
		status: status,
	}
}

// ServiceStatus reads the named service's status. The bool value will be false
// if the service has not been registered.
func (r *Registry) ServiceStatus(name string) (*ServiceStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if status, ok := r.services[name]; ok {
		return status.Clone(), true
	}

	return nil, false
}

// OverallStatus returns tbot's overall status when taking service statuses into
// account.
func (r *Registry) OverallStatus() *OverallStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	status := Healthy
	services := make(map[string]*ServiceStatus, len(r.services))

	for name, svc := range r.services {
		services[name] = svc.Clone()

		if svc.Status != Healthy {
			status = Unhealthy
		}
	}

	return &OverallStatus{
		Status:   status,
		Services: services,
	}
}

// ServiceStatus is a snapshot of the service's status.
type ServiceStatus struct {
	// Status of the service.
	Status Status `json:"status"`

	// Reason string describing why the service has its current status.
	Reason string `json:"reason,omitempty"`
}

// Clone the status to avoid data races.
func (s *ServiceStatus) Clone() *ServiceStatus {
	clone := *s
	return &clone
}

// OverallStatus is tbot's overall aggregate status.
type OverallStatus struct {
	// Status of tbot overall. If any service isn't Healthy, the overall status
	// will be Unhealthy.
	Status Status `json:"status"`

	// Services contains the service-specific statuses.
	Services map[string]*ServiceStatus `json:"services"`
}
