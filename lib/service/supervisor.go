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
	"sync"

	"github.com/gravitational/teleport/lib/utils"

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
	// interested parties
	BroadcastEvent(Event)

	// WaitForEvent waits for event to be broadcasted, if the event
	// was already broadcasted, payloadC will receive current event immediately
	// CLose 'cancelC' channel to force WaitForEvent to return prematurely
	WaitForEvent(name string, eventC chan Event, cancelC chan struct{})
}

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
}

// NewSupervisor returns new instance of initialized supervisor
func NewSupervisor() Supervisor {
	closeContext, cancel := context.WithCancel(context.TODO())
	srv := &LocalSupervisor{
		services:     []Service{},
		wg:           &sync.WaitGroup{},
		events:       map[string]Event{},
		eventsC:      make(chan Event, 1024),
		eventWaiters: make(map[string][]*waiter),
		closeContext: closeContext,
		signalClose:  cancel,
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
	log.WithFields(logrus.Fields{"service": srv.Name()}).Debugf("Adding service to supervisor.")
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

// RemoveService removes service from supervisor tracking list
func (s *LocalSupervisor) RemoveService(srv Service) error {
	l := logrus.WithFields(logrus.Fields{"service": srv.Name()})
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

func (s *LocalSupervisor) serve(srv Service) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.RemoveService(srv)
		log.WithFields(logrus.Fields{"service": srv.Name()}).Debugf("Service has started.")
		err := srv.Serve()
		if err != nil {
			utils.FatalError(err)
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

func (s *LocalSupervisor) BroadcastEvent(event Event) {
	s.Lock()
	defer s.Unlock()
	s.events[event.Name] = event
	log.WithFields(logrus.Fields{"event": event.String()}).Debugf("Broadcasting event.")

	go func() {
		s.eventsC <- event
	}()
}

func (s *LocalSupervisor) WaitForEvent(name string, eventC chan Event, cancelC chan struct{}) {
	s.Lock()
	defer s.Unlock()

	waiter := &waiter{eventC: eventC, cancelC: cancelC}
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
	case <-w.cancelC:
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
	cancelC chan struct{}
}

// Service is a running teleport service function
type Service interface {
	// Serve starts the function
	Serve() error
	// String returns user-friendly description of service
	String() string
	// Name returns service name
	Name() string
}

// LocalService is a locally defined service
type LocalService struct {
	// Function is a function to call
	Function ServiceFunc
	// ServiceName is a service name
	ServiceName string
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
