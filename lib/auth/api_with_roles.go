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

package auth

import (
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/log"
	"github.com/gravitational/trace"
)

type APIWithRoles struct {
	listeners map[string]*utils.MemoryListener
	servers   map[string]*APIServer
}

func NewAPIWithRoles(authServer *AuthServer, elog events.Log,
	sessions session.SessionServer, recorder recorder.Recorder,
	permChecker PermissionChecker,
	roles []string) *APIWithRoles {
	api := APIWithRoles{}
	api.listeners = make(map[string]*utils.MemoryListener)
	api.servers = make(map[string]*APIServer)

	for _, role := range roles {
		a := AuthWithRoles{
			authServer:  authServer,
			elog:        elog,
			sessions:    sessions,
			recorder:    recorder,
			permChecker: permChecker,
			role:        role,
		}
		api.servers[role] = NewAPIServer(&a)
		api.listeners[role] = utils.NewMemoryListener()
	}
	return &api
}

func (api *APIWithRoles) Serve() {
	wg := sync.WaitGroup{}
	for role, _ := range api.listeners {
		wg.Add(1)
		go func(l net.Listener, h http.Handler) {
			if err := http.Serve(l, h); (err != nil) && (err != io.EOF) {
				log.Errorf(err.Error())
			}
		}(api.listeners[role], api.servers[role])
	}
	wg.Wait()
}

func (api *APIWithRoles) HandleConn(conn net.Conn, role string) error {
	listener, ok := api.listeners[role]
	if !ok {
		conn.Close()
		return trace.Errorf("no such role '%v'", role)
	}
	return listener.Handle(conn)
}

func (api *APIWithRoles) Close() {
	for _, listener := range api.listeners {
		listener.Close()
	}

}
