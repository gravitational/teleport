package service

import (
	"sync"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

// Supervisor implements the simple service logic
type Supervisor struct {
	state int
	sync.Mutex
	wg       *sync.WaitGroup
	services []Service
	errors   []error
}

// Register adds the service to the pool, if supervisor is in
// the started state, the service will be started immediatelly
// otherwise, it will be started after Start() has been called
func (s *Supervisor) Register(srv Service) {
	s.Lock()
	defer s.Unlock()

	s.services = append(s.services, srv)
	if s.state == stateStarted {
		s.serve(srv)
	}
}

func (s *Supervisor) RegisterFunc(fn ServiceFunc) {
	s.Register(fn)
}

func (s *Supervisor) serve(srv Service) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := srv.Serve()
		log.Infof("%v completed with %v", s, err)
	}()
}

func (s *Supervisor) Start() error {
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

func (s *Supervisor) Wait() {
	s.wg.Wait()
}

func New() *Supervisor {
	return &Supervisor{
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
