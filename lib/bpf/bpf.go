/*
Copyright 2019 Gravitational, Inc.

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

package bpf

import (
	"context"

	"github.com/gravitational/trace"
)

type Service struct {
	exec *exec
	open *open
	conn *conn
}

func New(closeContext context.Context) *Service {
	return &Service{
		exec: newExec(closeContext),
		//open: newOpen(closeContext),
		conn: newConn(closeContext),
	}
}

func (s *Service) Start() error {
	err := s.exec.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	//err = s.open.Start()
	//if err != nil {
	//	return trace.Wrap(err)
	//}

	err = s.conn.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (s *Service) Close() {
	s.exec.Close()
	//s.open.Close()
	s.conn.Close()
}
