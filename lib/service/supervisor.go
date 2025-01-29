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

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Supervisor implements the simple service logic - registering
// service functions and de-registering the service goroutines
type Supervisor interface {
	// Register adds the service to the pool, if supervisor is in
	// the started state, the service will be started immediately
	// otherwise, it will be started after Start() has been called
	Register(srv Service)

	// RegisterFunc creates a service from function spec and registers
	// it within the system
	RegisterFunc(name string, fn Func)

	// RegisterCriticalFunc creates a critical service from function spec and registers
	// it within the system, if this service exits with error,
	// the process shuts down.
	RegisterCriticalFunc(name string, fn Func)

	// ServiceCount returns the number of registered and actively running
	// services
	ServiceCount() int

	// Start starts all unstarted services
	Start() error

	// Wait waits until all services exit
	Wait() error

	// Run starts and waits for the service to complete
	// it's a combinatioin Start() and Wait()
	Run() error

	// Services returns list of running services
	Services() []string

	// BroadcastEvent generates event and broadcasts it to all
	// subscribed parties.
	BroadcastEvent(Event)

	// WaitForEvent waits for one event with the specified name (returns the
	// latest such event if at least one has been broadcasted already, ignoring
	// the context). Returns an error if the context is canceled before an event
	// is received.
	WaitForEvent(ctx context.Context, name string) (Event, error)

	// WaitForEventTimeout waits for one event with the specified name (returns the
	// latest such event if at least one has been broadcasted already). Returns
	// an error if the timeout triggers before an event is received.
	WaitForEventTimeout(timeout time.Duration, name string) (Event, error)

	// ListenForEvents arranges for eventC to receive events with the specified
	// name; if the event was already broadcasted, eventC will receive the latest
	// value immediately. The broadcasting will stop when the context is done.
	ListenForEvents(ctx context.Context, name string, eventC chan<- Event)

	// ListenForNewEvents arranges for eventC to receive new events with the
	// specified name. The broadcasting will stop when the context is done.
	ListenForNewEvents(ctx context.Context, name string, eventC chan<- Event)

	// RegisterEventMapping registers event mapping -
	// when the sequence in the event mapping triggers, the
	// outbound event will be generated.
	RegisterEventMapping(EventMapping)

	// ExitContext returns context that will be closed when
	// a hard TeleportExitEvent is broadcasted.
	ExitContext() context.Context

	// GracefulExitContext returns context that will be closed when
	// a graceful or hard TeleportExitEvent is broadcast.
	GracefulExitContext() context.Context
}

// EventMapping maps a sequence of incoming
// events and if triggered, generates an out event.
type EventMapping struct {
	// In is the incoming event sequence.
	In []string
	// Out is the outbound event to generate.
	Out string
}

// String returns user-friendly representation of the mapping.
func (e EventMapping) String() string {
	return fmt.Sprintf("EventMapping(in=%v, out=%v)", e.In, e.Out)
}

// matches indicates whether the event mapping has been satisfied.
// If returns an error only if the mapping is unsatisified because
// we are still waiting on one or more events.
func (e EventMapping) matches(currentEvent string, m map[string]Event) (bool, error) {
	// existing events that have been fired should match
	for _, in := range e.In {
		if _, ok := m[in]; !ok {
			return false, fmt.Errorf("still waiting for %v", in)
		}
	}
	// current event that is firing should match one of the expected events
	for _, in := range e.In {
		if currentEvent == in {
			return true, nil
		}
	}

	// mapping not satisfied, and this event is not part of the mapping
	return false, nil
}

// LocalSupervisor is a Teleport's implementation of the Supervisor interface.
type LocalSupervisor struct {
	state int
	sync.Mutex
	wg           *sync.WaitGroup
	services     []Service
	events       map[string]Event
	eventsC      chan Event
	eventWaiters map[string][]*waiter

	closeContext context.Context
	signalClose  context.CancelFunc

	// exitContext is closed when someone emits a hard Exit event
	exitContext context.Context
	signalExit  context.CancelFunc

	// gracefulExitContext is closed when someone emits a graceful or hard Exit event
	gracefulExitContext context.Context
	signalGracefulExit  context.CancelFunc

	eventMappings []EventMapping
	id            string

	// log specifies the logger
	log *slog.Logger
}

// NewSupervisor returns new instance of initialized supervisor
func NewSupervisor(id string, parentLog *slog.Logger) Supervisor {
	ctx := context.TODO()

	closeContext, cancel := context.WithCancel(ctx)

	exitContext, signalExit := context.WithCancel(ctx)

	// graceful exit context is a subcontext of exit context since any work that terminates
	// in the event of graceful exit must also terminate in the event of an immediate exit.
	gracefulExitContext, signalGracefulExit := context.WithCancel(exitContext)

	srv := &LocalSupervisor{
		state:        stateCreated,
		id:           id,
		services:     []Service{},
		wg:           &sync.WaitGroup{},
		events:       map[string]Event{},
		eventsC:      make(chan Event, 1024),
		eventWaiters: make(map[string][]*waiter),
		closeContext: closeContext,
		signalClose:  cancel,

		exitContext:         exitContext,
		signalExit:          signalExit,
		gracefulExitContext: gracefulExitContext,
		signalGracefulExit:  signalGracefulExit,

		log: parentLog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentProcess, id)),
	}
	go srv.fanOut()
	return srv
}

// Event is a special service event that can be generated
// by various goroutines in the supervisor
type Event struct {
	Name    string
	Payload interface{}
}

func (e *Event) String() string {
	return e.Name
}

func (s *LocalSupervisor) Register(srv Service) {
	s.log.DebugContext(s.closeContext, "Adding service to supervisor", "service", srv.Name())
	s.Lock()
	defer s.Unlock()
	s.services = append(s.services, srv)

	if s.state == stateStarted {
		s.serve(srv)
	}
}

// ServiceCount returns the number of registered and actively running services
func (s *LocalSupervisor) ServiceCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.services)
}

// RegisterFunc creates a service from function spec and registers
// it within the system
func (s *LocalSupervisor) RegisterFunc(name string, fn Func) {
	s.Register(&LocalService{Function: fn, ServiceName: name})
}

// RegisterCriticalFunc creates a critical service from function spec and registers
// it within the system, if this service exits with error,
// the process shuts down.
func (s *LocalSupervisor) RegisterCriticalFunc(name string, fn Func) {
	s.Register(&LocalService{Function: fn, ServiceName: name, Critical: true})
}

// RemoveService removes service from supervisor tracking list
func (s *LocalSupervisor) RemoveService(srv Service) error {
	l := s.log.With("service", srv.Name())
	s.Lock()
	defer s.Unlock()
	for i, el := range s.services {
		if el == srv {
			s.services = append(s.services[:i], s.services[i+1:]...)
			l.DebugContext(s.closeContext, "Service is completed and removed")
			return nil
		}
	}
	l.WarnContext(s.closeContext, "Service is completed but not found")
	return trace.NotFound("service %v is not found", srv)
}

// ExitEventPayload contains information about service
// name, and service error if it exited with error
type ExitEventPayload struct {
	// Service is the service that exited
	Service Service
	// Error is the error of the service exit
	Error error
}

var metricsServicesRunning = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricTeleportServices,
		Help:      "Teleport services currently enabled and running",
	},
	[]string{teleport.TagServiceName},
)

var metricsServicesRunningMap = map[string]string{
	"discovery.init":       "discovery_service",
	"ssh.node":             "ssh_service",
	"auth.tls":             "auth_service",
	"proxy.web":            "proxy_service",
	"kube.init":            "kubernetes_service",
	"apps.start":           "application_service",
	"db.init":              "database_service",
	"windows_desktop.init": "windows_desktop_service",
	"okta.init":            "okta_service",
	"jamf.init":            "jamf_service",
}

func (s *LocalSupervisor) serve(srv Service) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.RemoveService(srv)

		if label, ok := metricsServicesRunningMap[srv.Name()]; ok {
			metricsServicesRunning.WithLabelValues(label).Inc()
			defer metricsServicesRunning.WithLabelValues(label).Dec()
		}

		l := s.log.With("service", srv.Name())
		l.DebugContext(s.closeContext, "Service has started")
		err := srv.Serve()
		if err != nil {
			if errors.Is(err, ErrTeleportExited) {
				l.InfoContext(s.closeContext, "Teleport process has shut down")
			} else {
				if s.ExitContext().Err() == nil {
					l.WarnContext(s.closeContext, "Teleport process has exited with error", "error", err)
				}
				s.BroadcastEvent(Event{
					Name:    ServiceExitedWithErrorEvent,
					Payload: ExitEventPayload{Service: srv, Error: err},
				})
			}
		}
	}()
}

func (s *LocalSupervisor) Start() error {
	s.Lock()
	defer s.Unlock()
	s.state = stateStarted

	if len(s.services) == 0 {
		s.log.WarnContext(s.closeContext, "Supervisor has no services to run - exiting")
		return nil
	}

	if err := metrics.RegisterPrometheusCollectors(metricsServicesRunning); err != nil {
		return trace.Wrap(err)
	}

	for _, srv := range s.services {
		s.serve(srv)
	}

	return nil
}

func (s *LocalSupervisor) Services() []string {
	s.Lock()
	defer s.Unlock()

	out := make([]string, len(s.services))

	for i, srv := range s.services {
		out[i] = srv.Name()
	}
	return out
}

func (s *LocalSupervisor) Wait() error {
	defer s.signalClose()
	s.wg.Wait()
	return nil
}

func (s *LocalSupervisor) Run() error {
	if err := s.Start(); err != nil {
		return trace.Wrap(err)
	}
	return s.Wait()
}

// ExitContext returns context that will be closed when
// a hard TeleportExitEvent is broadcasted.
func (s *LocalSupervisor) ExitContext() context.Context {
	return s.exitContext
}

// GracefulExitContext returns context that will be closed when
// a hard or graceful TeleportExitEvent is broadcasted.
func (s *LocalSupervisor) GracefulExitContext() context.Context {
	return s.gracefulExitContext
}

// BroadcastEvent generates event and broadcasts it to all
// subscribed parties.
func (s *LocalSupervisor) BroadcastEvent(event Event) {
	s.Lock()
	defer s.Unlock()

	if event.Name == TeleportExitEvent {
		// if exit event includes a context payload, it is a "graceful" exit, and
		// we need to hold off closing the supervisor's exit context until after
		// the graceful context has closed.  If not, it is an immediate exit.
		if ctx, ok := event.Payload.(context.Context); ok {
			s.signalGracefulExit()
			go func() {
				select {
				case <-s.exitContext.Done():
				case <-ctx.Done():
					s.signalExit()
				}
			}()
		} else {
			s.signalExit()
		}
	}

	s.events[event.Name] = event

	// Log all events other than recovered events to prevent the logs from
	// being flooded.
	if event.String() != TeleportOKEvent {
		s.log.DebugContext(s.closeContext, "Broadcasting event", "event", logutils.StringerAttr(&event))
	}

	go func() {
		select {
		case s.eventsC <- event:
		case <-s.closeContext.Done():
			return
		}
	}()

	for _, m := range s.eventMappings {
		if matches, err := m.matches(event.Name, s.events); matches && err == nil {
			mappedEvent := Event{Name: m.Out}
			s.events[mappedEvent.Name] = mappedEvent
			go func(e Event) {
				select {
				case s.eventsC <- e:
				case <-s.closeContext.Done():
					return
				}
			}(mappedEvent)
			s.log.DebugContext(s.closeContext, "Broadcasting mapped event",
				"in", logutils.StringerAttr(&event),
				"out", logutils.StringerAttr(m),
			)
		} else if err != nil {
			s.log.DebugContext(s.closeContext, "Teleport not yet ready", "error", err)
		}
	}
}

// RegisterEventMapping registers event mapping -
// when the sequence in the event mapping triggers, the
// outbound event will be generated.
func (s *LocalSupervisor) RegisterEventMapping(m EventMapping) {
	s.Lock()
	defer s.Unlock()

	s.eventMappings = append(s.eventMappings, m)
}

func (s *LocalSupervisor) WaitForEvent(ctx context.Context, name string) (Event, error) {
	s.Lock()

	if event, ok := s.events[name]; ok {
		s.Unlock()
		return event, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventC := make(chan Event)
	waiter := &waiter{eventC: eventC, context: ctx}
	s.eventWaiters[name] = append(s.eventWaiters[name], waiter)
	s.Unlock()

	select {
	case event := <-eventC:
		return event, nil
	case <-ctx.Done():
		return Event{}, trace.Wrap(ctx.Err())
	}
}

func (s *LocalSupervisor) WaitForEventTimeout(timeout time.Duration, name string) (Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := s.WaitForEvent(ctx, name)
	if err != nil && ctx.Err() != nil {
		return event, trace.Errorf("timeout waiting for event %q (%s)", name, timeout)
	}
	return event, trace.Wrap(err)
}

func (s *LocalSupervisor) ListenForEvents(ctx context.Context, name string, eventC chan<- Event) {
	s.Lock()
	defer s.Unlock()

	waiter := &waiter{eventC: eventC, context: ctx}
	event, ok := s.events[name]
	if ok {
		go waiter.notify(event)
	}
	s.eventWaiters[name] = append(s.eventWaiters[name], waiter)
}

func (s *LocalSupervisor) ListenForNewEvents(ctx context.Context, name string, eventC chan<- Event) {
	s.Lock()
	defer s.Unlock()

	waiter := &waiter{eventC: eventC, context: ctx}
	s.eventWaiters[name] = append(s.eventWaiters[name], waiter)
}

func (s *LocalSupervisor) fanOut() {
	for {
		select {
		case event := <-s.eventsC:
			s.Lock()
			waiters, ok := s.eventWaiters[event.Name]
			if !ok {
				s.Unlock()
				continue
			}
			aliveWaiters := waiters[:0]
			for _, waiter := range waiters {
				if waiter.context.Err() == nil {
					aliveWaiters = append(aliveWaiters, waiter)
					go waiter.notify(event)
				}
			}
			if len(aliveWaiters) == 0 {
				delete(s.eventWaiters, event.Name)
			} else {
				s.eventWaiters[event.Name] = aliveWaiters
			}
			s.Unlock()
		case <-s.closeContext.Done():
			return
		}
	}
}

type waiter struct {
	eventC  chan<- Event
	context context.Context
}

func (w *waiter) notify(event Event) {
	select {
	case w.eventC <- event:
	case <-w.context.Done():
	}
}

// Service is a running teleport service function
type Service interface {
	// Serve starts the function
	Serve() error
	// String returns user-friendly description of service
	String() string
	// Name returns service name
	Name() string
	// IsCritical returns true if the service is critical
	// and program can't continue without it
	IsCritical() bool
}

// LocalService is a locally defined service
type LocalService struct {
	// Function is a function to call
	Function Func
	// ServiceName is a service name
	ServiceName string
	// Critical is set to true
	// when the service is critical and program can't continue
	// without it
	Critical bool
}

// IsCritical returns true if the service is critical
// and program can't continue without it
func (l *LocalService) IsCritical() bool {
	return l.Critical
}

// Serve starts the function
func (l *LocalService) Serve() error {
	return l.Function()
}

// String returns user-friendly service name
func (l *LocalService) String() string {
	return l.ServiceName
}

// Name returns unique service name
func (l *LocalService) Name() string {
	return l.ServiceName
}

// Func is a service function
type Func func() error

const (
	stateCreated = iota
	stateStarted
)
