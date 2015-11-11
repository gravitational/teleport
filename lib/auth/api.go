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

/*import (
	"net"
	"net/http"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
)

func StartHTTPServer(a string, srv *AuthServer, elog events.Log, se session.SessionServer, rec recorder.Recorder) error {
	addr, err := utils.ParseAddr(a)
	if err != nil {
		return err
	}
	t, err := trace.New(
		NewAPIServer(srv, elog, se, rec),
		log.GetLogger().Writer(log.SeverityInfo))
	if err != nil {
		return err
	}
	if addr.Network == "tcp" {
		hsrv := &http.Server{
			Addr:    addr.Addr,
			Handler: t,
		}
		return hsrv.ListenAndServe()
	}
	l, err := net.Listen(addr.Network, addr.Addr)
	if err != nil {
		return err
	}
	hsrv := &http.Server{
		Handler: t,
	}
	return hsrv.Serve(l)
}
*/
