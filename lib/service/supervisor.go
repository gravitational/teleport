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
	"fmt"
	"sync"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Supervisor implements the simple service logic - registering
// service functions and de-registering the service goroutines
type Supervisor interface {
	// Register adds the service to the pool, if supervisor is in
	// the started state, the service will be started immediatelly
	// otherwise, it will be started after Start() has been called
	Register(srv Service)

	// RegisterFunc creates a service from function spec and registers
	// it within the system
	RegisterFunc(fn ServiceFunc)

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
	services     []*Service
	errors       []error
	events       map[string]Event
	eventsC      chan Event
	eventWaiters map[string][]*waiter
	closer       *utils.CloseBroadcaster
}

// NewSupervisor returns new instance of initialized supervisor
func NewSupervisor() Supervisor {
	srv := &LocalSupervisor{
		services:     []*Service{},
		wg:           &sync.WaitGroup{},
		events:       map[string]Event{},
		eventsC:      make(chan Event, 100),
		eventWaiters: make(map[string][]*waiter),
		closer:       utils.NewCloseBroadcaster(),
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
	return fmt.Sprintf("event(%v)", e.Name)
}

func (s *LocalSupervisor) Register(srv Service) {
	s.Lock()
	defer s.Unlock()
	s.services = append(s.services, &srv)

	log.Debugf("[SUPERVISOR] Service %v added (%v)", srv, len(s.services))

	if s.state == stateStarted {
		s.serve(&srv)
	}
}

// ServiceCount returns the number of registered and actively running services
func (s *LocalSupervisor) ServiceCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.services)
}

func (s *LocalSupervisor) RegisterFunc(fn ServiceFunc) {
	s.Register(fn)
}

func (s *LocalSupervisor) serve(srv *Service) {
	// this func will be called _after_ a service stops running:
	removeService := func() {
		s.Lock()
		defer s.Unlock()
		for i, el := range s.services {
			if el == srv {
				s.services = append(s.services[:i], s.services[i+1:]...)
				break
			}
		}
		log.Debugf("[SUPERVISOR] Service %v is done (%v)", *srv, len(s.services))
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer removeService()

		log.Debugf("[SUPERVISOR] Service %v started (%v)", *srv, s.ServiceCount())
		err := (*srv).Serve()
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
		log.Warning("supervisor.Start(): nothing to run")
		return nil
	}

	for _, srv := range s.services {
		s.serve(srv)
	}

	return nil
}

func (s *LocalSupervisor) Wait() error {
	defer s.closer.Close()
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
	log.Debugf("BroadcastEvent: %v", &event)

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
		case <-s.closer.C:
			return
		}
	}
}

type waiter struct {
	eventC  chan Event
	cancelC chan struct{}
}

type Service interface {
	Serve() error
}

type ServiceFunc func() error

func (s ServiceFunc) Serve() error {
	return s()
}

const (
	stateCreated = iota
	stateStarted = iota
)
