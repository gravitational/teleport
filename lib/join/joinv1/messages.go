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
	switch payload := req.GetPayload().(type) {
	case *joinv1.JoinRequest_ClientInit:
		return clientInitToMessage(payload.ClientInit), nil
	case *joinv1.JoinRequest_BoundKeypairInit:
		return boundKeypairInitToMessage(payload.BoundKeypairInit), nil
	case *joinv1.JoinRequest_Solution:
		return challengeSolutionToMessage(payload.Solution)
	default:
		return nil, trace.BadParameter("unrecognized join request payload type %T", payload)
	}
}

func requestFromMessage(msg messages.Request) (*joinv1.JoinRequest, error) {
	switch typedMsg := msg.(type) {
	case *messages.ClientInit:
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_ClientInit{
				ClientInit: clientInitFromMessage(typedMsg),
			},
		}, nil
	case *messages.BoundKeypairInit:
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_BoundKeypairInit{
				BoundKeypairInit: boundKeypairInitFromMessage(typedMsg),
			},
		}, nil
	case *messages.BoundKeypairChallengeSolution, *messages.BoundKeypairRotationResponse:
		solution, err := challengeSolutionFromMessage(msg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_Solution{
				Solution: solution,
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

func challengeSolutionToMessage(req *joinv1.ChallengeSolution) (messages.Request, error) {
	switch payload := req.GetPayload().(type) {
	case *joinv1.ChallengeSolution_BoundKeypairChallengeSolution:
		return boundKeypairChallengeSolutionToMessage(payload.BoundKeypairChallengeSolution), nil
	case *joinv1.ChallengeSolution_BoundKeypairRotationResponse:
		return boundKeypairRotationResponseToMessage(payload.BoundKeypairRotationResponse), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge solution message type %T", payload)
	}
}

func challengeSolutionFromMessage(msg messages.Request) (*joinv1.ChallengeSolution, error) {
	switch typedMsg := msg.(type) {
	case *messages.BoundKeypairChallengeSolution:
		return &joinv1.ChallengeSolution{
			Payload: &joinv1.ChallengeSolution_BoundKeypairChallengeSolution{
				BoundKeypairChallengeSolution: boundKeypairChallengeSolutionFromMessage(typedMsg),
			},
		}, nil
	case *messages.BoundKeypairRotationResponse:
		return &joinv1.ChallengeSolution{
			Payload: &joinv1.ChallengeSolution_BoundKeypairRotationResponse{
				BoundKeypairRotationResponse: boundKeypairRotationResponseFromMessage(typedMsg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized challenge solution message type %T", msg)
	}
}

func responseToMessage(resp *joinv1.JoinResponse) (messages.Response, error) {
	switch typedResp := resp.Payload.(type) {
	case *joinv1.JoinResponse_Init:
		return serverInitToMessage(typedResp.Init), nil
	case *joinv1.JoinResponse_Challenge:
		return challengeToMessage(typedResp.Challenge)
	case *joinv1.JoinResponse_Result:
		return resultToMessage(typedResp.Result), nil
	default:
		return nil, trace.BadParameter("unrecognized join response message type %T", typedResp)
	}
}

func responseFromMessage(resp messages.Response) (*joinv1.JoinResponse, error) {
	switch msg := resp.(type) {
	case *messages.ServerInit:
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Init{
				Init: serverInitFromMessage(msg),
			},
		}, nil
	case *messages.BoundKeypairChallenge, *messages.BoundKeypairRotationRequest:
		challenge, err := challengeFromMessage(msg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Challenge{
				Challenge: challenge,
			},
		}, nil
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

func challengeToMessage(resp *joinv1.Challenge) (messages.Response, error) {
	switch payload := resp.Payload.(type) {
	case *joinv1.Challenge_BoundKeypairChallenge:
		return boundKeypairChallengeToMessage(payload.BoundKeypairChallenge), nil
	case *joinv1.Challenge_BoundKeypairRotationRequest:
		return boundKeypairRotationRequestToMessage(payload.BoundKeypairRotationRequest), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge payload type %T", payload)
	}
}

func challengeFromMessage(resp messages.Response) (*joinv1.Challenge, error) {
	switch msg := resp.(type) {
	case *messages.BoundKeypairChallenge:
		return &joinv1.Challenge{
			Payload: &joinv1.Challenge_BoundKeypairChallenge{
				BoundKeypairChallenge: boundKeypairChallengeFromMessage(msg),
			},
		}, nil
	case *messages.BoundKeypairRotationRequest:
		return &joinv1.Challenge{
			Payload: &joinv1.Challenge_BoundKeypairRotationRequest{
				BoundKeypairRotationRequest: boundKeypairRotationRequestFromMessage(msg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized challenge message type %T", msg)
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

func boundKeypairInitToMessage(req *joinv1.BoundKeypairInit) *messages.BoundKeypairInit {
	return &messages.BoundKeypairInit{
		InitialJoinSecret: req.InitialJoinSecret,
		PreviousJoinState: req.PreviousJoinState,
	}
}

func boundKeypairInitFromMessage(msg *messages.BoundKeypairInit) *joinv1.BoundKeypairInit {
	return &joinv1.BoundKeypairInit{
		InitialJoinSecret: msg.InitialJoinSecret,
		PreviousJoinState: msg.PreviousJoinState,
	}
}

func boundKeypairChallengeToMessage(resp *joinv1.BoundKeypairChallenge) *messages.BoundKeypairChallenge {
	return &messages.BoundKeypairChallenge{
		PublicKey: resp.PublicKey,
		Challenge: resp.Challenge,
	}
}

func boundKeypairChallengeFromMessage(msg *messages.BoundKeypairChallenge) *joinv1.BoundKeypairChallenge {
	return &joinv1.BoundKeypairChallenge{
		PublicKey: msg.PublicKey,
		Challenge: msg.Challenge,
	}
}

func boundKeypairChallengeSolutionToMessage(req *joinv1.BoundKeypairChallengeSolution) *messages.BoundKeypairChallengeSolution {
	return &messages.BoundKeypairChallengeSolution{
		Solution: req.Solution,
	}
}

func boundKeypairRotationRequestToMessage(resp *joinv1.BoundKeypairRotationRequest) *messages.BoundKeypairRotationRequest {
	return &messages.BoundKeypairRotationRequest{
		SignatureAlgorithmSuite: resp.SignatureAlgorithmSuite,
	}
}

func boundKeypairRotationRequestFromMessage(resp *messages.BoundKeypairRotationRequest) *joinv1.BoundKeypairRotationRequest {
	return &joinv1.BoundKeypairRotationRequest{
		SignatureAlgorithmSuite: resp.SignatureAlgorithmSuite,
	}
}

func boundKeypairChallengeSolutionFromMessage(msg *messages.BoundKeypairChallengeSolution) *joinv1.BoundKeypairChallengeSolution {
	return &joinv1.BoundKeypairChallengeSolution{
		Solution: msg.Solution,
	}
}

func boundKeypairRotationResponseToMessage(req *joinv1.BoundKeypairRotationResponse) *messages.BoundKeypairRotationResponse {
	return &messages.BoundKeypairRotationResponse{
		PublicKey: req.PublicKey,
	}
}

func boundKeypairRotationResponseFromMessage(msg *messages.BoundKeypairRotationResponse) *joinv1.BoundKeypairRotationResponse {
	return &joinv1.BoundKeypairRotationResponse{
		PublicKey: msg.PublicKey,
	}
}

func boundKeypairResultToMessage(req *joinv1.BoundKeypairResult) *messages.BoundKeypairResult {
	if req == nil {
		return nil
	}
	return &messages.BoundKeypairResult{
		JoinState: req.JoinState,
		PublicKey: req.PublicKey,
	}
}

func boundKeypairResultFromMessage(msg *messages.BoundKeypairResult) *joinv1.BoundKeypairResult {
	if msg == nil {
		return nil
	}
	return &joinv1.BoundKeypairResult{
		JoinState: msg.JoinState,
		PublicKey: msg.PublicKey,
	}
}

func serverInitToMessage(resp *joinv1.ServerInit) *messages.ServerInit {
	return &messages.ServerInit{
		JoinMethod: resp.JoinMethod,
	}
}

func serverInitFromMessage(resp *messages.ServerInit) *joinv1.ServerInit {
	return &joinv1.ServerInit{
		JoinMethod: resp.JoinMethod,
	}
}

func resultToMessage(resp *joinv1.Result) *messages.Result {
	return &messages.Result{
		TLSCert:            resp.TlsCert,
		TLSCACerts:         resp.TlsCaCerts,
		SSHCert:            resp.SshCert,
		SSHCAKeys:          resp.SshCaKeys,
		HostID:             resp.HostId,
		BoundKeypairResult: boundKeypairResultToMessage(resp.BoundKeypairResult),
	}
}

func resultFromMessage(msg *messages.Result) *joinv1.Result {
	return &joinv1.Result{
		TlsCert:            msg.TLSCert,
		TlsCaCerts:         msg.TLSCACerts,
		SshCert:            msg.SSHCert,
		SshCaKeys:          msg.SSHCAKeys,
		HostId:             msg.HostID,
		BoundKeypairResult: boundKeypairResultFromMessage(msg.BoundKeypairResult),
	}
}
