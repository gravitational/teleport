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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
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
		Protocol:           alpncommon.ProtocolProxySSH,
		InsecureSkipVerify: cf.InsecureSkipVerify,
		ParentContext:      cf.Context,
		SNI:                address.Host(),
		SSHUser:            cf.Username,
		SSHUserHost:        cf.UserHost,
		SSHHostKeyCallback: client.HostKeyCallback,
		SSHTrustedCluster:  cf.SiteName,
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

	addr := "localhost:0"
	if cf.LocalProxyPort != "" {
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.WithError(err).Warnf("Failed to close listener.")
		}
	}()
	lp, err := mkLocalProxy(cf.Context, client.WebProxyAddr, database.Protocol, listener)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-cf.Context.Done()
		lp.Close()
	}()

	fmt.Printf("Started DB proxy on %s\n", listener.Addr())

	defer lp.Close()
	if err := lp.Start(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func mkLocalProxy(ctx context.Context, remoteProxyAddr string, protocol string, listener net.Listener) (*alpnproxy.LocalProxy, error) {
	alpnProtocol, err := toALPNProtocol(protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	address, err := utils.ParseAddr(remoteProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr: remoteProxyAddr,
		Protocol:        alpnProtocol,
		Listener:        listener,
		ParentContext:   ctx,
		SNI:             address.Host(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lp, nil
}

func toALPNProtocol(dbProtocol string) (alpncommon.Protocol, error) {
	switch dbProtocol {
	case defaults.ProtocolMySQL:
		return alpncommon.ProtocolMySQL, nil
	case defaults.ProtocolPostgres:
		return alpncommon.ProtocolPostgres, nil
	case defaults.ProtocolMongoDB:
		return alpncommon.ProtocolMongoDB, nil
	default:
		return "", trace.NotImplemented("%q protocol is not supported", dbProtocol)
	}
}
