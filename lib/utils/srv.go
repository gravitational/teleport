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

package utils

import (
	"net"
	"net/http"
)

func StartHTTPServer(addr NetAddr, h http.Handler) error {
	if addr.AddrNetwork == "tcp" {
		hsrv := &http.Server{
			Addr:    addr.Addr,
			Handler: h,
		}
		return hsrv.ListenAndServe()
	}
	l, err := net.Listen(addr.AddrNetwork, addr.Addr)
	if err != nil {
		return err
	}
	hsrv := &http.Server{
		Handler: h,
	}
	return hsrv.Serve(l)
}
