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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// requestToMessage converts a gRPC JoinRequest into a protocol-agnostic [messages.Request].
func requestToMessage(req *joinv1.JoinRequest) (messages.Request, error) {
	switch msg := req.GetPayload().(type) {
	case *joinv1.JoinRequest_ClientInit:
		return clientInitToMessage(msg.ClientInit), nil
	case *joinv1.JoinRequest_TokenInit:
		return tokenInitToMessage(msg.TokenInit)
	case *joinv1.JoinRequest_BoundKeypairInit:
		return boundKeypairInitToMessage(msg.BoundKeypairInit)
	case *joinv1.JoinRequest_IamInit:
		return iamInitToMessage(msg.IamInit)
	case *joinv1.JoinRequest_Ec2Init:
		return ec2InitToMessage(msg.Ec2Init)
	case *joinv1.JoinRequest_Solution:
		return challengeSolutionToMessage(msg.Solution)
	case *joinv1.JoinRequest_GivingUp:
		return givingUpToMessage(msg.GivingUp), nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

// requestFromMessage converts a [messages.Request] into a
// [*joinv1.JoinRequest] to be sent over gRPC.
func requestFromMessage(msg messages.Request) (*joinv1.JoinRequest, error) {
	switch typedMsg := msg.(type) {
	case *messages.ClientInit:
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_ClientInit{
				ClientInit: clientInitFromMessage(typedMsg),
			},
		}, nil
	case *messages.TokenInit:
		tokenInit, err := tokenInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_TokenInit{
				TokenInit: tokenInit,
			},
		}, nil
	case *messages.BoundKeypairInit:
		boundKeypairInit, err := boundKeypairInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_BoundKeypairInit{
				BoundKeypairInit: boundKeypairInit,
			},
		}, nil
	case *messages.IAMInit:
		iamInit, err := iamInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_IamInit{
				IamInit: iamInit,
			},
		}, nil
	case *messages.EC2Init:
		ec2Init, err := ec2InitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_Ec2Init{
				Ec2Init: ec2Init,
			},
		}, nil
	case *messages.BoundKeypairChallengeSolution,
		*messages.BoundKeypairRotationResponse,
		*messages.IAMChallengeSolution:
		solution, err := challengeSolutionFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_Solution{
				Solution: solution,
			},
		}, nil
	case *messages.GivingUp:
		return &joinv1.JoinRequest{
			Payload: &joinv1.JoinRequest_GivingUp{
				GivingUp: givingUpFromMessage(typedMsg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

func clientInitToMessage(req *joinv1.ClientInit) *messages.ClientInit {
	msg := &messages.ClientInit{
		JoinMethod:       req.JoinMethod,
		TokenName:        req.TokenName,
		SystemRole:       req.SystemRole,
		ForwardedByProxy: req.ForwardedByProxy,
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
		ForwardedByProxy: msg.ForwardedByProxy,
	}
	if proxySuppliedParams := msg.ProxySuppliedParams; proxySuppliedParams != nil {
		req.ProxySuppliedParameters = &joinv1.ClientInit_ProxySuppliedParams{
			RemoteAddr:    proxySuppliedParams.RemoteAddr,
			ClientVersion: proxySuppliedParams.ClientVersion,
		}
	}
	return req
}

func tokenInitToMessage(req *joinv1.TokenInit) (*messages.TokenInit, error) {
	clientParams, err := clientParamsToMessage(req.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.TokenInit{
		ClientParams: clientParams,
	}, nil
}

func tokenInitFromMessage(msg *messages.TokenInit) (*joinv1.TokenInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &joinv1.TokenInit{
		ClientParams: clientParams,
	}, nil
}

func clientParamsToMessage(req *joinv1.ClientParams) (messages.ClientParams, error) {
	var msg messages.ClientParams
	switch req.GetPayload().(type) {
	case *joinv1.ClientParams_HostParams:
		msg.HostParams = hostParamsToMessage(req.GetHostParams())
	case *joinv1.ClientParams_BotParams:
		msg.BotParams = botParamsToMessage(req.GetBotParams())
	default:
		return msg, trace.BadParameter("unrecognized ClientParams payload type %T", req.Payload)
	}
	return msg, nil
}

func clientParamsFromMessage(msg messages.ClientParams) (*joinv1.ClientParams, error) {
	req := &joinv1.ClientParams{}
	switch {
	case msg.HostParams != nil:
		req.Payload = &joinv1.ClientParams_HostParams{
			HostParams: hostParamsFromMessage(msg.HostParams),
		}
	case msg.BotParams != nil:
		req.Payload = &joinv1.ClientParams_BotParams{
			BotParams: botParamsFromMessage(msg.BotParams),
		}
	default:
		return nil, trace.BadParameter("ClientParams has no payload")
	}
	return req, nil
}

func hostParamsToMessage(req *joinv1.HostParams) *messages.HostParams {
	return &messages.HostParams{
		PublicKeys:           publicKeysToMessage(req.PublicKeys),
		HostName:             req.HostName,
		AdditionalPrincipals: req.AdditionalPrincipals,
		DNSNames:             req.DnsNames,
	}
}

func hostParamsFromMessage(msg *messages.HostParams) *joinv1.HostParams {
	return &joinv1.HostParams{
		PublicKeys:           publicKeysFromMessage(msg.PublicKeys),
		HostName:             msg.HostName,
		AdditionalPrincipals: msg.AdditionalPrincipals,
		DnsNames:             msg.DNSNames,
	}
}

func botParamsToMessage(req *joinv1.BotParams) *messages.BotParams {
	msg := &messages.BotParams{
		PublicKeys: publicKeysToMessage(req.PublicKeys),
	}
	if req.Expires != nil {
		expires := req.Expires.AsTime()
		msg.Expires = &expires
	}
	return msg
}

func botParamsFromMessage(msg *messages.BotParams) *joinv1.BotParams {
	req := &joinv1.BotParams{
		PublicKeys: publicKeysFromMessage(msg.PublicKeys),
	}
	if msg.Expires != nil {
		req.Expires = timestamppb.New(*msg.Expires)
	}
	return req
}

func publicKeysToMessage(req *joinv1.PublicKeys) messages.PublicKeys {
	return messages.PublicKeys{
		PublicTLSKey: req.PublicTlsKey,
		PublicSSHKey: req.PublicSshKey,
	}
}

func publicKeysFromMessage(msg messages.PublicKeys) *joinv1.PublicKeys {
	return &joinv1.PublicKeys{
		PublicTlsKey: msg.PublicTLSKey,
		PublicSshKey: msg.PublicSSHKey,
	}
}

func challengeSolutionToMessage(req *joinv1.ChallengeSolution) (messages.Request, error) {
	switch payload := req.GetPayload().(type) {
	case *joinv1.ChallengeSolution_BoundKeypairChallengeSolution:
		return boundKeypairChallengeSolutionToMessage(payload.BoundKeypairChallengeSolution), nil
	case *joinv1.ChallengeSolution_BoundKeypairRotationResponse:
		return boundKeypairRotationResponseToMessage(payload.BoundKeypairRotationResponse), nil
	case *joinv1.ChallengeSolution_IamChallengeSolution:
		return iamChallengeSolutionToMessage(payload.IamChallengeSolution), nil
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
	case *messages.IAMChallengeSolution:
		return &joinv1.ChallengeSolution{
			Payload: &joinv1.ChallengeSolution_IamChallengeSolution{
				IamChallengeSolution: iamChallengeSolutionFromMessage(typedMsg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized challenge solution message type %T", msg)
	}
}

// responseToMessage converts a gRPC JoinResponse into a protocol-agnostic [messages.Response].
func responseToMessage(resp *joinv1.JoinResponse) (messages.Response, error) {
	switch typedResp := resp.Payload.(type) {
	case *joinv1.JoinResponse_Init:
		return serverInitToMessage(typedResp.Init)
	case *joinv1.JoinResponse_Challenge:
		return challengeToMessage(typedResp.Challenge)
	case *joinv1.JoinResponse_Result:
		return resultToMessage(typedResp.Result)
	default:
		return nil, trace.BadParameter("unrecognized join response message type %T", typedResp)
	}
}

// responseFromMessage converts a [messages.Response] into a
// [*joinv1.JoinResponse] to be sent over gRPC.
func responseFromMessage(msg messages.Response) (*joinv1.JoinResponse, error) {
	switch typedMsg := msg.(type) {
	case *messages.ServerInit:
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Init{
				Init: serverInitFromMessage(typedMsg),
			},
		}, nil
	case *messages.BoundKeypairChallenge,
		*messages.BoundKeypairRotationRequest,
		*messages.IAMChallenge:
		challenge, err := challengeFromMessage(msg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Challenge{
				Challenge: challenge,
			},
		}, nil
	case *messages.HostResult:
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Result{
				Result: &joinv1.Result{
					Payload: &joinv1.Result_HostResult{
						HostResult: hostResultFromMessage(typedMsg),
					},
				},
			},
		}, nil
	case *messages.BotResult:
		return &joinv1.JoinResponse{
			Payload: &joinv1.JoinResponse_Result{
				Result: &joinv1.Result{
					Payload: &joinv1.Result_BotResult{
						BotResult: botResultFromMessage(typedMsg),
					},
				},
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized join response message type %T", msg)
	}
}

func serverInitToMessage(req *joinv1.ServerInit) (*messages.ServerInit, error) {
	sas, err := types.SignatureAlgorithmSuiteFromString(req.SignatureAlgorithmSuite)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.ServerInit{
		JoinMethod:              req.JoinMethod,
		SignatureAlgorithmSuite: sas,
	}, nil
}

func serverInitFromMessage(msg *messages.ServerInit) *joinv1.ServerInit {
	return &joinv1.ServerInit{
		JoinMethod:              msg.JoinMethod,
		SignatureAlgorithmSuite: types.SignatureAlgorithmSuiteToString(msg.SignatureAlgorithmSuite),
	}
}

func challengeToMessage(resp *joinv1.Challenge) (messages.Response, error) {
	switch payload := resp.Payload.(type) {
	case *joinv1.Challenge_BoundKeypairChallenge:
		return boundKeypairChallengeToMessage(payload.BoundKeypairChallenge), nil
	case *joinv1.Challenge_BoundKeypairRotationRequest:
		return boundKeypairRotationRequestToMessage(payload.BoundKeypairRotationRequest), nil
	case *joinv1.Challenge_IamChallenge:
		return iamChallengeToMessage(payload.IamChallenge), nil
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
	case *messages.IAMChallenge:
		return &joinv1.Challenge{
			Payload: &joinv1.Challenge_IamChallenge{
				IamChallenge: iamChallengeFromMessage(msg),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unrecognized challenge message type %T", msg)
	}
}

func resultToMessage(resp *joinv1.Result) (messages.Response, error) {
	switch resp.Payload.(type) {
	case *joinv1.Result_HostResult:
		return hostResultToMessage(resp.GetHostResult()), nil
	case *joinv1.Result_BotResult:
		return botResultToMessage(resp.GetBotResult()), nil
	default:
		return nil, trace.BadParameter("unrecodgnize result payload type %T", resp.Payload)
	}
}

func hostResultToMessage(resp *joinv1.HostResult) *messages.HostResult {
	return &messages.HostResult{
		Certificates: certificatesToMessage(resp.Certificates),
		HostID:       resp.HostId,
	}
}

func hostResultFromMessage(msg *messages.HostResult) *joinv1.HostResult {
	return &joinv1.HostResult{
		Certificates: certificatesFromMessage(&msg.Certificates),
		HostId:       msg.HostID,
	}
}

func botResultToMessage(resp *joinv1.BotResult) *messages.BotResult {
	return &messages.BotResult{
		Certificates:       certificatesToMessage(resp.Certificates),
		BoundKeypairResult: boundKeypairResultToMessage(resp.BoundKeypairResult),
	}
}

func botResultFromMessage(msg *messages.BotResult) *joinv1.BotResult {
	return &joinv1.BotResult{
		Certificates:       certificatesFromMessage(&msg.Certificates),
		BoundKeypairResult: boundKeypairResultFromMessage(msg.BoundKeypairResult),
	}
}

func certificatesToMessage(certs *joinv1.Certificates) messages.Certificates {
	return messages.Certificates{
		TLSCert:    certs.TlsCert,
		TLSCACerts: certs.TlsCaCerts,
		SSHCert:    certs.SshCert,
		SSHCAKeys:  certs.SshCaKeys,
	}
}

func certificatesFromMessage(certs *messages.Certificates) *joinv1.Certificates {
	return &joinv1.Certificates{
		TlsCert:    certs.TLSCert,
		TlsCaCerts: certs.TLSCACerts,
		SshCert:    certs.SSHCert,
		SshCaKeys:  certs.SSHCAKeys,
	}
}

func givingUpToMessage(req *joinv1.GivingUp) *messages.GivingUp {
	reason := messages.GivingUpReasonUnspecified
	switch req.Reason {
	case joinv1.GivingUp_REASON_UNSUPPORTED_JOIN_METHOD:
		reason = messages.GivingUpReasonUnsupportedJoinMethod
	case joinv1.GivingUp_REASON_UNSUPPORTED_MESSAGE_TYPE:
		reason = messages.GivingUpReasonUnsupportedMessageType
	case joinv1.GivingUp_REASON_CHALLENGE_SOLUTION_FAILED:
		reason = messages.GivingUpReasonChallengeSolutionFailed
	}
	return &messages.GivingUp{
		Reason: reason,
		Msg:    req.Msg,
	}
}

func givingUpFromMessage(msg *messages.GivingUp) *joinv1.GivingUp {
	reason := joinv1.GivingUp_REASON_UNSPECIFIED
	switch msg.Reason {
	case messages.GivingUpReasonUnsupportedJoinMethod:
		reason = joinv1.GivingUp_REASON_UNSUPPORTED_JOIN_METHOD
	case messages.GivingUpReasonUnsupportedMessageType:
		reason = joinv1.GivingUp_REASON_UNSUPPORTED_MESSAGE_TYPE
	case messages.GivingUpReasonChallengeSolutionFailed:
		reason = joinv1.GivingUp_REASON_CHALLENGE_SOLUTION_FAILED
	}
	return &joinv1.GivingUp{
		Reason: reason,
		Msg:    msg.Msg,
	}
}
