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
	"container/list"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
)

// NewRegistry returns a Registry to track the health of tbot's services.
func NewRegistry(opts ...RegistryOpt) *Registry {
	reg := &Registry{
		clock:    clockwork.NewRealClock(),
		services: make(map[string]*ServiceStatus),
		watchers: list.New(),
	}
	for _, opt := range opts {
		opt(reg)
	}
	return reg
}

// WithClock allows you to use a fake clock.
func WithClock(clock clockwork.Clock) RegistryOpt {
	return func(reg *Registry) { reg.clock = clock }
}

// RegistryOpt is an optional parameter to NewRegistry.
type RegistryOpt func(r *Registry)

// Registry tracks the status/health of tbot's services.
type Registry struct {
	clock clockwork.Clock

	mu       sync.Mutex
	services map[string]*ServiceStatus
	watchers *list.List
}

// AddService adds a service to the registry so that its health will be reported
// from our readyz endpoints. It returns a Reporter the service can use to report
// status changes.
func (r *Registry) AddService(name, serviceType string) Reporter {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.services[name]
	if !ok {
		status = &ServiceStatus{
			name:        name,
			serviceType: serviceType,
			updatedAt:   r.clock.Now(),
		}
		r.services[name] = status
	}
	return &reporter{
		registry: r,
		mu:       &r.mu,
		status:   status,
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

// ToProto returns the protobuf representation of the current health of all
// services.
func (r *Registry) ToProto() []*machineidv1pb.BotInstanceServiceHealth {
	r.mu.Lock()
	defer r.mu.Unlock()

	proto := make([]*machineidv1pb.BotInstanceServiceHealth, 0, len(r.services))
	for _, svc := range r.services {
		proto = append(proto, svc.ToProto())
	}
	return proto
}

// Watch returns a channel you can receive from to be notified when a service's
// status changes. You must call the returned close function when you're done to
// free resources.
func (r *Registry) Watch() (<-chan struct{}, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan struct{}, 1)
	elem := r.watchers.PushBack(ch)

	return ch, sync.OnceFunc(func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.watchers.Remove(elem)
	})
}

func (r *Registry) notifyWatchersLocked() {
	for elem := r.watchers.Front(); elem != nil; elem = elem.Next() {
		select {
		case elem.Value.(chan struct{}) <- struct{}{}:
		default:
			// Already a notification buffered in the channel.
		}
	}
}

// ServiceStatus is a snapshot of the service's status.
type ServiceStatus struct {
	// Status of the service.
	Status Status `json:"status"`

	// Reason string describing why the service has its current status.
	Reason string `json:"reason,omitempty"`

	// These unexported fields are used for the protobuf representation.
	name, serviceType string
	updatedAt         time.Time
}

// ToProto returns the protobuf representation of the service status.
func (s *ServiceStatus) ToProto() *machineidv1pb.BotInstanceServiceHealth {
	proto := &machineidv1pb.BotInstanceServiceHealth{
		Service: &machineidv1pb.BotInstanceServiceIdentifier{
			Name: s.name,
			Type: s.serviceType,
		},
		Status:    s.Status.ToProto(),
		UpdatedAt: timestamppb.New(s.updatedAt),
	}
	if s.Reason != "" {
		proto.Reason = &s.Reason
	}
	return proto
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
