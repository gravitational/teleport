/*
Copyright 2015 Gravitational, Inc.

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

package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
	RegisterFunc(name string, fn ServiceFunc)

	// RegisterCriticalFunc creates a critical service from function spec and registers
	// it within the system, if this service exits with error,
	// the process shuts down.
	RegisterCriticalFunc(name string, fn ServiceFunc)

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

	// WaitForEvent waits for event to be broadcasted, if the event
	// was already broadcasted, eventC will receive current event immediately.
	WaitForEvent(ctx context.Context, name string, eventC chan Event)

	// RegisterEventMapping registers event mapping -
	// when the sequence in the event mapping triggers, the
	// outbound event will be generated.
	RegisterEventMapping(EventMapping)

	// ExitContext returns context that will be closed when
	// TeleportExitEvent is broadcasted.
	ExitContext() context.Context

	// ReloadContext returns context that will be closed when
	// TeleportReloadEvent is broadcasted.
	ReloadContext() context.Context
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

func (e EventMapping) matches(currentEvent string, m map[string]Event) bool {
	// existing events that have been fired should match
	for _, in := range e.In {
		if _, ok := m[in]; !ok {
			return false
		}
	}
	// current event that is firing should match one of the expected events
	for _, in := range e.In {
		if currentEvent == in {
			return true
		}
	}
	return false
}

// LocalSupervisor is a Teleport's implementation of the Supervisor interface.
type LocalSupervisor struct {
	state int
	sync.Mutex
	wg           *sync.WaitGroup
	services     []Service
	errors       []error
	events       map[string]Event
	eventsC      chan Event
	eventWaiters map[string][]*waiter

	closeContext context.Context
	signalClose  context.CancelFunc

	// exitContext is closed when someone emits Exit event
	exitContext context.Context
	signalExit  context.CancelFunc

	reloadContext context.Context
	signalReload  context.CancelFunc

	eventMappings []EventMapping
	id            string
}

// NewSupervisor returns new instance of initialized supervisor
func NewSupervisor(id string) Supervisor {
	closeContext, cancel := context.WithCancel(context.TODO())

	exitContext, signalExit := context.WithCancel(context.TODO())
	reloadContext, signalReload := context.WithCancel(context.TODO())

	srv := &LocalSupervisor{
		id:           id,
		services:     []Service{},
		wg:           &sync.WaitGroup{},
		events:       map[string]Event{},
		eventsC:      make(chan Event, 1024),
		eventWaiters: make(map[string][]*waiter),
		closeContext: closeContext,
		signalClose:  cancel,

		exitContext: exitContext,
		signalExit:  signalExit,

		reloadContext: reloadContext,
		signalReload:  signalReload,
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
	log.WithFields(logrus.Fields{"service": srv.Name(), trace.Component: teleport.ComponentProcess}).Debugf("Adding service to supervisor.")
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
func (s *LocalSupervisor) RegisterFunc(name string, fn ServiceFunc) {
	s.Register(&LocalService{Function: fn, ServiceName: name})
}

// RegisterCriticalFunc creates a critical service from function spec and registers
// it within the system, if this service exits with error,
// the process shuts down.
func (s *LocalSupervisor) RegisterCriticalFunc(name string, fn ServiceFunc) {
	s.Register(&LocalService{Function: fn, ServiceName: name, Critical: true})
}

// RemoveService removes service from supervisor tracking list
func (s *LocalSupervisor) RemoveService(srv Service) error {
	l := logrus.WithFields(logrus.Fields{"service": srv.Name(), trace.Component: teleport.Component(teleport.ComponentProcess, s.id)})
	s.Lock()
	defer s.Unlock()
	for i, el := range s.services {
		if el == srv {
			s.services = append(s.services[:i], s.services[i+1:]...)
			l.Debugf("Service is completed and removed.")
			return nil
		}
	}
	l.Warningf("Service is completed but not found.")
	return trace.NotFound("service %v is not found", srv)
}

// ServiceExit contains information about service
// name, and service error if it exited with error
type ServiceExit struct {
	// Service is the service that exited
	Service Service
	// Error is the error of the service exit
	Error error
}

func (s *LocalSupervisor) serve(srv Service) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.RemoveService(srv)
		l := log.WithFields(logrus.Fields{"service": srv.Name(), trace.Component: teleport.Component(teleport.ComponentProcess, s.id)})
		l.Debugf("Service has started.")
		err := srv.Serve()
		if err != nil {
			if err == ErrTeleportExited {
				l.Infof("Teleport process has shut down.")
			} else {
				l.Warningf("Teleport process has exited with error: %v", err)
				s.BroadcastEvent(Event{
					Name:    ServiceExitedWithErrorEvent,
					Payload: ServiceExit{Service: srv, Error: err},
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
		log.Warning("Supervisor has no services to run. Exiting.")
		return nil
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
// TeleportExitEvent is broadcasted.
func (s *LocalSupervisor) ExitContext() context.Context {
	return s.exitContext
}

// ReloadContext returns context that will be closed when
// TeleportReloadEvent is broadcasted.
func (s *LocalSupervisor) ReloadContext() context.Context {
	return s.reloadContext
}

// BroadcastEvent generates event and broadcasts it to all
// subscribed parties.
func (s *LocalSupervisor) BroadcastEvent(event Event) {
	s.Lock()
	defer s.Unlock()

	switch event.Name {
	case TeleportExitEvent:
		s.signalExit()
	case TeleportReloadEvent:
		s.signalReload()
	}

	s.events[event.Name] = event
	log.WithFields(logrus.Fields{"event": event.String(), trace.Component: teleport.Component(teleport.ComponentProcess, s.id)}).Debugf("Broadcasting event.")

	go func() {
		select {
		case s.eventsC <- event:
		case <-s.closeContext.Done():
			return
		}
	}()

	for _, m := range s.eventMappings {
		if m.matches(event.Name, s.events) {
			mappedEvent := Event{Name: m.Out}
			s.events[mappedEvent.Name] = mappedEvent
			go func(e Event) {
				select {
				case s.eventsC <- e:
				case <-s.closeContext.Done():
					return
				}
			}(mappedEvent)
			log.WithFields(logrus.Fields{"in": event.String(), "out": m.String(), trace.Component: teleport.Component(teleport.ComponentProcess, s.id)}).Debugf("Broadcasting mapped event.")
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

// WaitForEvent waits for event to be broadcasted, if the event
// was already broadcasted, eventC will receive current event immediately.
func (s *LocalSupervisor) WaitForEvent(ctx context.Context, name string, eventC chan Event) {
	s.Lock()
	defer s.Unlock()

	waiter := &waiter{eventC: eventC, context: ctx}
	event, ok := s.events[name]
	if ok {
		go s.notifyWaiter(waiter, event)
		return
	}
	s.eventWaiters[name] = append(s.eventWaiters[name], waiter)
}

func (s *LocalSupervisor) getWaiters(name string) []*waiter {
	s.Lock()
	defer s.Unlock()

	waiters := s.eventWaiters[name]
	out := make([]*waiter, len(waiters))
	for i := range waiters {
		out[i] = waiters[i]
	}
	return out
}

func (s *LocalSupervisor) notifyWaiter(w *waiter, event Event) {
	select {
	case w.eventC <- event:
	case <-w.context.Done():
	}
}

func (s *LocalSupervisor) fanOut() {
	for {
		select {
		case event := <-s.eventsC:
			waiters := s.getWaiters(event.Name)
			for _, waiter := range waiters {
				go s.notifyWaiter(waiter, event)
			}
		case <-s.closeContext.Done():
			return
		}
	}
}

type waiter struct {
	eventC  chan Event
	context context.Context
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
	Function ServiceFunc
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

// ServiceFunc is a service function
type ServiceFunc func() error

const (
	stateCreated = iota
	stateStarted = iota
)
