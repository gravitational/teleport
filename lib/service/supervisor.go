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
	"os"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
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

	// Start starts all unstarted services
	Start() error

	// Wait waits until all services exit
	Wait() error

	// Run starts and waits for the service to complete
	// it's a combinatioin Start() and Wait()
	Run() error
}

type LocalSupervisor struct {
	state int
	sync.Mutex
	wg       *sync.WaitGroup
	services []Service
	errors   []error
}

func (s *LocalSupervisor) Register(srv Service) {
	s.Lock()
	defer s.Unlock()

	s.services = append(s.services, srv)
	if s.state == stateStarted {
		s.serve(srv)
	}
}

func (s *LocalSupervisor) RegisterFunc(fn ServiceFunc) {
	s.Register(fn)
}

func (s *LocalSupervisor) serve(srv Service) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := srv.Serve()
		if err != nil {
			log.Errorf("srv.Serve() error: %v", err)
			os.Exit(1)
		}
	}()
}

func (s *LocalSupervisor) Start() error {
	s.Lock()
	defer s.Unlock()
	s.state = stateStarted

	if len(s.services) == 0 {
		log.Infof("no services registered, returning")
		return nil
	}

	for _, srv := range s.services {
		s.serve(srv)
	}

	return nil
}

func (s *LocalSupervisor) Wait() error {
	s.wg.Wait()
	return nil
}

func (s *LocalSupervisor) Run() error {
	if err := s.Start(); err != nil {
		return trace.Wrap(err)
	}
	return s.Wait()
}

func NewSupervisor() Supervisor {
	return &LocalSupervisor{
		services: []Service{},
		wg:       &sync.WaitGroup{},
	}
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
