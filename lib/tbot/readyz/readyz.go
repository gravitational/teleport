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
	"time"

	"github.com/jonboulle/clockwork"
)

// NewRegistry returns a Registry to track the health of tbot's services.
func NewRegistry(opts ...NewRegistryOpt) *Registry {
	r := &Registry{
		clock:    clockwork.NewRealClock(),
		services: make(map[string]*ServiceStatus),
		notifyCh: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// NewRegistryOpt can be passed to NewRegistry to provide optional configuration.
type NewRegistryOpt func(r *Registry)

// WithClock sets the registry's clock.
func WithClock(clock clockwork.Clock) NewRegistryOpt {
	return func(r *Registry) { r.clock = clock }
}

// Registry tracks the status/health of tbot's services.
type Registry struct {
	clock clockwork.Clock

	mu       sync.Mutex
	services map[string]*ServiceStatus
	reported int
	notifyCh chan struct{}
}

// AddService adds a service to the registry so that its health will be reported
// from our readyz endpoints. It returns a Reporter the service can use to report
// status changes.
//
// Note: you should add all of your services before any service reports its status
// otherwise AllServicesReported will unblock too early.
func (r *Registry) AddService(serviceType, name string) Reporter {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO(boxofrad): If you add the same service multiple times, you could end
	// up unblocking AllServicesReported prematurely. The impact is low, it just
	// means we'd send a heartbeat sooner than is desirable, but we should panic
	// or return an error from this method instead.
	status, ok := r.services[name]
	if !ok {
		status = &ServiceStatus{ServiceType: serviceType}
		r.services[name] = status
	}

	return &reporter{
		mu:     &r.mu,
		clock:  r.clock,
		status: status,
		notify: sync.OnceFunc(r.maybeNotifyLocked),
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

// AllServicesReported returns a channel you can receive from to be notified
// when all registered services have reported their initial status. It provides
// a way for us to hold the initial heartbeat until after the initial flurry of
// activity.
func (r *Registry) AllServicesReported() <-chan struct{} { return r.notifyCh }

// maybeNotifyLocked unblocks the AllServicesReported channel if all services
// have reported their initial status. It's called by each of the Reporters the
// first time you report a status.
//
// Caller must be holding r.mu.
func (r *Registry) maybeNotifyLocked() {
	r.reported++

	if r.reported != len(r.services) {
		return
	}

	select {
	case <-r.notifyCh:
	default:
		close(r.notifyCh)
	}
}

// ServiceStatus is a snapshot of the service's status.
type ServiceStatus struct {
	// Status of the service.
	Status Status `json:"status"`

	// Reason string describing why the service has its current status.
	Reason string `json:"reason,omitempty"`

	// UpdatedAt is the time at which the service's status last changed.
	UpdatedAt *time.Time `json:"updated_at"`

	// ServiceType is exposed in bot heartbeats, but not the `/readyz` endpoint.
	ServiceType string `json:"-"`
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
