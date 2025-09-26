// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joinv1

import (
	"github.com/gravitational/trace"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func requestToMessage(req *joinv1.JoinRequest) (messages.Request, error) {
	switch msg := req.GetPayload().(type) {
	case *joinv1.JoinRequest_ClientInit:
		return clientInitToMessage(msg.ClientInit), nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

func clientInitToMessage(req *joinv1.ClientInit) *messages.ClientInit {
	msg := &messages.ClientInit{
		JoinMethod:       req.JoinMethod,
		TokenName:        req.TokenName,
		SystemRole:       req.SystemRole,
		PublicTLSKey:     req.PublicTlsKey,
		PublicSSHKey:     req.PublicSshKey,
		ForwardedByProxy: req.ForwardedByProxy,
		HostParams:       &messages.HostParams{},
	}
	if hostParams := req.HostParams; hostParams != nil {
		msg.HostParams = &messages.HostParams{
			HostName:             hostParams.HostName,
			AdditionalPrincipals: hostParams.AdditionalPrincipals,
			DNSNames:             hostParams.DnsNames,
		}
	}
	if botParams := req.BotParams; botParams != nil {
		msg.BotParams = &messages.BotParams{}
		if botParams.Expires != nil {
			expires := botParams.Expires.AsTime()
			msg.BotParams.Expires = &expires
		}
	}
	if proxySuppliedParams := req.GetProxySuppliedParameters(); proxySuppliedParams != nil {
		msg.ProxySuppliedParams = &messages.ProxySuppliedParams{
			RemoteAddr:    proxySuppliedParams.RemoteAddr,
			ClientVersion: proxySuppliedParams.ClientVersion,
		}
	}
	return msg
}

func requestFromMessage(msg messages.Request) (*joinv1.JoinRequest, error) {
	switch typedMsg := msg.(type) {
	case *messages.ClientInit:
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_ClientInit{
				ClientInit: clientInitFromMessage(typedMsg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

func clientInitFromMessage(msg *messages.ClientInit) *joinv1.ClientInit {
	req := &joinv1.ClientInit{
		JoinMethod:       msg.JoinMethod,
		TokenName:        msg.TokenName,
		SystemRole:       msg.SystemRole,
		PublicTlsKey:     msg.PublicTLSKey,
		PublicSshKey:     msg.PublicSSHKey,
		ForwardedByProxy: msg.ForwardedByProxy,
	}
	if hostParams := msg.HostParams; hostParams != nil {
		req.HostParams = &joinv1.ClientInit_HostParams{
			HostName:             hostParams.HostName,
			AdditionalPrincipals: hostParams.AdditionalPrincipals,
			DnsNames:             hostParams.DNSNames,
		}
	}
	if botParams := msg.BotParams; botParams != nil {
		req.BotParams = &joinv1.ClientInit_BotParams{}
		if botParams.Expires != nil {
			req.BotParams.Expires = timestamppb.New(*botParams.Expires)
		}
	}
	if proxySuppliedParams := msg.ProxySuppliedParams; proxySuppliedParams != nil {
		req.ProxySuppliedParameters = &joinv1.ClientInit_ProxySuppliedParams{
			RemoteAddr:    proxySuppliedParams.RemoteAddr,
			ClientVersion: proxySuppliedParams.ClientVersion,
		}
	}
	return req
}

func responseFromMessage(resp messages.Response) (*joinv1.JoinResponse, error) {
	switch msg := resp.(type) {
	case *messages.Result:
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Result{
				Result: resultFromMessage(msg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized join response message type %T", msg)
	}
}

func resultFromMessage(msg *messages.Result) *joinv1.Result {
	return &joinv1.Result{
		TlsCert:    msg.TLSCert,
		TlsCaCerts: msg.TLSCACerts,
		SshCert:    msg.SSHCert,
		SshCaKeys:  msg.SSHCAKeys,
		HostId:     msg.HostID,
	}
}

func responseToMessage(resp *joinv1.JoinResponse) (messages.Response, error) {
	switch typedResp := resp.Payload.(type) {
	case *joinv1.JoinResponse_Result:
		return resultToMessage(typedResp.Result), nil
	default:
		return nil, trace.BadParameter("unrecognized join responsed message type %T", typedResp)
	}
}

func resultToMessage(resp *joinv1.Result) *messages.Result {
	return &messages.Result{
		TLSCert:    resp.TlsCert,
		TLSCACerts: resp.TlsCaCerts,
		SSHCert:    resp.SshCert,
		SSHCAKeys:  resp.SshCaKeys,
		HostID:     resp.HostId,
	}
}
