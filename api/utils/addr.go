/*
Copyright 2021 Gravitational, Inc.

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

// Package webclient provides a client for the Teleport Proxy API endpoints.
package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// ParseAddr strings like "tcp://host:port/path" and returns "host:port".
func ParseAddr(addr string) (string, error) {
	if addr == "" {
		return "", trace.BadParameter("missing parameter address")
	}
	if !strings.Contains(addr, "://") {
		addr = "tcp://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", trace.BadParameter("failed to parse %q: %v", addr, err)
	}
	switch u.Scheme {
	case "tcp":
		return u.Host, nil
	case "unix":
		return u.Path, nil
	case "http", "https":
		fmt.Println(u.Host)
		return u.Host, nil
	default:
		return "", trace.BadParameter("'%v': unsupported scheme: '%v'", addr, u.Scheme)
	}
}

func ParseHost(addr string) (string, error) {
	parsed, err := ParseAddr(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	host, _, err := net.SplitHostPort(parsed)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return host, nil
}

func ParsePort(addr string) (string, error) {
	parsed, err := ParseAddr(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(parsed)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return port, nil
}
