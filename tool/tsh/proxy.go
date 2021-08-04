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

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

func onProxyCommandSSH(cf *CLIConf) error {
	client, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	address, err := utils.ParseAddr(client.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    client.WebProxyAddr,
		Protocol:           alpnproxy.ProtocolProxySSH,
		InsecureSkipVerify: cf.InsecureSkipVerify,
		ParentContext:      cf.Context,
		SNI:                address.Host(),
		SSHUser:            cf.Username,
		SSHUserHost:        cf.UserHost,
		SSHHostKeyCallback: client.HostKeyCallback,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer lp.Close()
	if err := lp.SSHProxy(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func onProxyCommandDB(cf *CLIConf) error {
	client, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	addr := "127.0.0.1:0"
	if cf.LocalProxyPort != "" {
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}
	lp, err := mkLocalProxy(cf.Context, client.WebProxyAddr, database.Protocol, listener)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cf.Context)
	defer cancel()
	go func() {
		<-ctx.Done()
		lp.Close()
	}()

	fmt.Printf("Started DB proxy on %s\n", listener.Addr())

	defer lp.Close()
	if err := lp.Start(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
