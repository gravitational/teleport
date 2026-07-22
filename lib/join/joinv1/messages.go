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
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// requestToMessage converts a gRPC JoinRequest into a protocol-agnostic [messages.Request].
func requestToMessage(req *joinv1.JoinRequest) (messages.Request, error) {
	switch msg := req.WhichPayload(); msg {
	case joinv1.JoinRequest_ClientInit_case:
		return clientInitToMessage(req.GetClientInit()), nil
	case joinv1.JoinRequest_TokenInit_case:
		return tokenInitToMessage(req.GetTokenInit())
	case joinv1.JoinRequest_BoundKeypairInit_case:
		return boundKeypairInitToMessage(req.GetBoundKeypairInit())
	case joinv1.JoinRequest_IamInit_case:
		return iamInitToMessage(req.GetIamInit())
	case joinv1.JoinRequest_Ec2Init_case:
		return ec2InitToMessage(req.GetEc2Init())
	case joinv1.JoinRequest_OidcInit_case:
		return oidcInitToMessage(req.GetOidcInit())
	case joinv1.JoinRequest_OracleInit_case:
		return oracleInitToMessage(req.GetOracleInit())
	case joinv1.JoinRequest_TpmInit_case:
		return tpmInitToMessage(req.GetTpmInit())
	case joinv1.JoinRequest_AzureInit_case:
		return azureInitToMessage(req.GetAzureInit())
	case joinv1.JoinRequest_Solution_case:
		return challengeSolutionToMessage(req.GetSolution())
	case joinv1.JoinRequest_GivingUp_case:
		return givingUpToMessage(req.GetGivingUp()), nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %v", msg)
	}
}

// requestFromMessage converts a [messages.Request] into a
// [*joinv1.JoinRequest] to be sent over gRPC.
func requestFromMessage(msg messages.Request) (*joinv1.JoinRequest, error) {
	switch typedMsg := msg.(type) {
	case *messages.ClientInit:
		return joinv1.JoinRequest_builder{
			ClientInit: proto.ValueOrDefault(clientInitFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.TokenInit:
		tokenInit, err := tokenInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			TokenInit: proto.ValueOrDefault(tokenInit),
		}.Build(), nil
	case *messages.BoundKeypairInit:
		boundKeypairInit, err := boundKeypairInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			BoundKeypairInit: proto.ValueOrDefault(boundKeypairInit),
		}.Build(), nil
	case *messages.IAMInit:
		iamInit, err := iamInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			IamInit: proto.ValueOrDefault(iamInit),
		}.Build(), nil
	case *messages.EC2Init:
		ec2Init, err := ec2InitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			Ec2Init: proto.ValueOrDefault(ec2Init),
		}.Build(), nil
	case *messages.OIDCInit:
		oidcInit, err := oidcInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			OidcInit: proto.ValueOrDefault(oidcInit),
		}.Build(), nil
	case *messages.OracleInit:
		oracleInit, err := oracleInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			OracleInit: proto.ValueOrDefault(oracleInit),
		}.Build(), nil
	case *messages.TPMInit:
		tpmInit, err := tpmInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			TpmInit: proto.ValueOrDefault(tpmInit),
		}.Build(), nil
	case *messages.AzureInit:
		azureInit, err := azureInitFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			AzureInit: proto.ValueOrDefault(azureInit),
		}.Build(), nil
	case *messages.BoundKeypairChallengeSolution,
		*messages.BoundKeypairRotationResponse,
		*messages.IAMChallengeSolution,
		*messages.OracleChallengeSolution,
		*messages.TPMSolution,
		*messages.AzureChallengeSolution:
		solution, err := challengeSolutionFromMessage(typedMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinRequest_builder{
			Solution: proto.ValueOrDefault(solution),
		}.Build(), nil
	case *messages.GivingUp:
		return joinv1.JoinRequest_builder{
			GivingUp: proto.ValueOrDefault(givingUpFromMessage(typedMsg)),
		}.Build(), nil
	default:
		return nil, trace.BadParameter("unrecognized join request message type %T", msg)
	}
}

func clientInitToMessage(req *joinv1.ClientInit) *messages.ClientInit {
	msg := &messages.ClientInit{
		TokenName:        req.GetTokenName(),
		SystemRole:       req.GetSystemRole(),
		ForwardedByProxy: req.GetForwardedByProxy(),
	}
	if joinMethod := req.GetJoinMethod(); joinMethod != "" {
		msg.JoinMethod = &joinMethod
	}
	if proxySuppliedParams := req.GetProxySuppliedParameters(); proxySuppliedParams != nil {
		msg.ProxySuppliedParams = &messages.ProxySuppliedParams{
			RemoteAddr:    proxySuppliedParams.GetRemoteAddr(),
			ClientVersion: proxySuppliedParams.GetClientVersion(),
		}
	}
	return msg
}

func clientInitFromMessage(msg *messages.ClientInit) *joinv1.ClientInit {
	req := joinv1.ClientInit_builder{
		JoinMethod:       msg.JoinMethod,
		TokenName:        msg.TokenName,
		SystemRole:       msg.SystemRole,
		ForwardedByProxy: msg.ForwardedByProxy,
	}.Build()
	if proxySuppliedParams := msg.ProxySuppliedParams; proxySuppliedParams != nil {
		req.SetProxySuppliedParameters(joinv1.ClientInit_ProxySuppliedParams_builder{
			RemoteAddr:    proxySuppliedParams.RemoteAddr,
			ClientVersion: proxySuppliedParams.ClientVersion,
		}.Build())
	}
	return req
}

func tokenInitToMessage(req *joinv1.TokenInit) (*messages.TokenInit, error) {
	clientParams, err := clientParamsToMessage(req.GetClientParams())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.TokenInit{
		ClientParams: clientParams,
		Secret:       req.GetSecret(),
	}, nil
}

func tokenInitFromMessage(msg *messages.TokenInit) (*joinv1.TokenInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return joinv1.TokenInit_builder{
		ClientParams: clientParams,
		Secret:       msg.Secret,
	}.Build(), nil
}

func clientParamsToMessage(req *joinv1.ClientParams) (messages.ClientParams, error) {
	var msg messages.ClientParams
	switch payload := req.WhichPayload(); payload {
	case joinv1.ClientParams_HostParams_case:
		msg.HostParams = hostParamsToMessage(req.GetHostParams())
	case joinv1.ClientParams_BotParams_case:
		msg.BotParams = botParamsToMessage(req.GetBotParams())
	default:
		return msg, trace.BadParameter("unrecognized ClientParams payload type %v", payload)
	}
	return msg, nil
}

func clientParamsFromMessage(msg messages.ClientParams) (*joinv1.ClientParams, error) {
	req := &joinv1.ClientParams{}
	switch {
	case msg.HostParams != nil:
		req.SetHostParams(proto.ValueOrDefault(hostParamsFromMessage(msg.HostParams)))
	case msg.BotParams != nil:
		req.SetBotParams(proto.ValueOrDefault(botParamsFromMessage(msg.BotParams)))
	default:
		return nil, trace.BadParameter("ClientParams has no payload")
	}
	return req, nil
}

func hostParamsToMessage(req *joinv1.HostParams) *messages.HostParams {
	return &messages.HostParams{
		PublicKeys:           publicKeysToMessage(req.GetPublicKeys()),
		HostName:             req.GetHostName(),
		AdditionalPrincipals: req.GetAdditionalPrincipals(),
		DNSNames:             req.GetDnsNames(),
	}
}

func hostParamsFromMessage(msg *messages.HostParams) *joinv1.HostParams {
	return joinv1.HostParams_builder{
		PublicKeys:           publicKeysFromMessage(msg.PublicKeys),
		HostName:             msg.HostName,
		AdditionalPrincipals: msg.AdditionalPrincipals,
		DnsNames:             msg.DNSNames,
	}.Build()
}

func botParamsToMessage(req *joinv1.BotParams) *messages.BotParams {
	msg := &messages.BotParams{
		PublicKeys: publicKeysToMessage(req.GetPublicKeys()),
	}
	if req.HasExpires() {
		expires := req.GetExpires().AsTime()
		msg.Expires = &expires
	}
	return msg
}

func botParamsFromMessage(msg *messages.BotParams) *joinv1.BotParams {
	req := joinv1.BotParams_builder{
		PublicKeys: publicKeysFromMessage(msg.PublicKeys),
	}.Build()
	if msg.Expires != nil {
		req.SetExpires(timestamppb.New(*msg.Expires))
	}
	return req
}

func publicKeysToMessage(req *joinv1.PublicKeys) messages.PublicKeys {
	return messages.PublicKeys{
		PublicTLSKey: req.GetPublicTlsKey(),
		PublicSSHKey: req.GetPublicSshKey(),
	}
}

func publicKeysFromMessage(msg messages.PublicKeys) *joinv1.PublicKeys {
	return joinv1.PublicKeys_builder{
		PublicTlsKey: msg.PublicTLSKey,
		PublicSshKey: msg.PublicSSHKey,
	}.Build()
}

func challengeSolutionToMessage(req *joinv1.ChallengeSolution) (messages.Request, error) {
	switch payload := req.WhichPayload(); payload {
	case joinv1.ChallengeSolution_BoundKeypairChallengeSolution_case:
		return boundKeypairChallengeSolutionToMessage(req.GetBoundKeypairChallengeSolution()), nil
	case joinv1.ChallengeSolution_BoundKeypairRotationResponse_case:
		return boundKeypairRotationResponseToMessage(req.GetBoundKeypairRotationResponse()), nil
	case joinv1.ChallengeSolution_IamChallengeSolution_case:
		return iamChallengeSolutionToMessage(req.GetIamChallengeSolution()), nil
	case joinv1.ChallengeSolution_OracleChallengeSolution_case:
		return oracleChallengeSolutionToMessage(req.GetOracleChallengeSolution()), nil
	case joinv1.ChallengeSolution_TpmSolution_case:
		return tpmSolutionToMessage(req.GetTpmSolution()), nil
	case joinv1.ChallengeSolution_AzureChallengeSolution_case:
		return azureChallengeSolutionToMessage(req.GetAzureChallengeSolution()), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge solution message type %v", payload)
	}
}

func challengeSolutionFromMessage(msg messages.Request) (*joinv1.ChallengeSolution, error) {
	switch typedMsg := msg.(type) {
	case *messages.BoundKeypairChallengeSolution:
		return joinv1.ChallengeSolution_builder{
			BoundKeypairChallengeSolution: proto.ValueOrDefault(boundKeypairChallengeSolutionFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.BoundKeypairRotationResponse:
		return joinv1.ChallengeSolution_builder{
			BoundKeypairRotationResponse: proto.ValueOrDefault(boundKeypairRotationResponseFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.IAMChallengeSolution:
		return joinv1.ChallengeSolution_builder{
			IamChallengeSolution: proto.ValueOrDefault(iamChallengeSolutionFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.OracleChallengeSolution:
		return joinv1.ChallengeSolution_builder{
			OracleChallengeSolution: proto.ValueOrDefault(oracleChallengeSolutionFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.TPMSolution:
		return joinv1.ChallengeSolution_builder{
			TpmSolution: proto.ValueOrDefault(tpmSolutionFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.AzureChallengeSolution:
		return joinv1.ChallengeSolution_builder{
			AzureChallengeSolution: proto.ValueOrDefault(azureChallengeSolutionFromMessage(typedMsg)),
		}.Build(), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge solution message type %T", msg)
	}
}

// responseToMessage converts a gRPC JoinResponse into a protocol-agnostic [messages.Response].
func responseToMessage(resp *joinv1.JoinResponse) (messages.Response, error) {
	switch typedResp := resp.WhichPayload(); typedResp {
	case joinv1.JoinResponse_Init_case:
		return serverInitToMessage(resp.GetInit())
	case joinv1.JoinResponse_Challenge_case:
		return challengeToMessage(resp.GetChallenge())
	case joinv1.JoinResponse_Result_case:
		return resultToMessage(resp.GetResult())
	default:
		return nil, trace.BadParameter("unrecognized join response message type %v", typedResp)
	}
}

// responseFromMessage converts a [messages.Response] into a
// [*joinv1.JoinResponse] to be sent over gRPC.
func responseFromMessage(msg messages.Response) (*joinv1.JoinResponse, error) {
	switch typedMsg := msg.(type) {
	case *messages.ServerInit:
		return joinv1.JoinResponse_builder{
			Init: proto.ValueOrDefault(serverInitFromMessage(typedMsg)),
		}.Build(), nil
	case *messages.BoundKeypairChallenge,
		*messages.BoundKeypairRotationRequest,
		*messages.IAMChallenge,
		*messages.OracleChallenge,
		*messages.TPMEncryptedCredential,
		*messages.AzureChallenge:
		challenge, err := challengeFromMessage(msg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return joinv1.JoinResponse_builder{
			Challenge: proto.ValueOrDefault(challenge),
		}.Build(), nil
	case *messages.HostResult:
		return joinv1.JoinResponse_builder{
			Result: joinv1.Result_builder{
				HostResult: proto.ValueOrDefault(hostResultFromMessage(typedMsg)),
			}.Build(),
		}.Build(), nil
	case *messages.BotResult:
		return joinv1.JoinResponse_builder{
			Result: joinv1.Result_builder{
				BotResult: proto.ValueOrDefault(botResultFromMessage(typedMsg)),
			}.Build(),
		}.Build(), nil
	default:
		return nil, trace.BadParameter("unrecognized join response message type %T", msg)
	}
}

func serverInitToMessage(resp *joinv1.ServerInit) (*messages.ServerInit, error) {
	sas, err := types.SignatureAlgorithmSuiteFromString(resp.GetSignatureAlgorithmSuite())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.ServerInit{
		JoinMethod:              resp.GetJoinMethod(),
		SignatureAlgorithmSuite: sas,
	}, nil
}

func serverInitFromMessage(msg *messages.ServerInit) *joinv1.ServerInit {
	return joinv1.ServerInit_builder{
		JoinMethod:              msg.JoinMethod,
		SignatureAlgorithmSuite: types.SignatureAlgorithmSuiteToString(msg.SignatureAlgorithmSuite),
	}.Build()
}

func challengeToMessage(resp *joinv1.Challenge) (messages.Response, error) {
	switch payload := resp.WhichPayload(); payload {
	case joinv1.Challenge_BoundKeypairChallenge_case:
		return boundKeypairChallengeToMessage(resp.GetBoundKeypairChallenge()), nil
	case joinv1.Challenge_BoundKeypairRotationRequest_case:
		return boundKeypairRotationRequestToMessage(resp.GetBoundKeypairRotationRequest()), nil
	case joinv1.Challenge_IamChallenge_case:
		return iamChallengeToMessage(resp.GetIamChallenge()), nil
	case joinv1.Challenge_OracleChallenge_case:
		return oracleChallengeToMessage(resp.GetOracleChallenge()), nil
	case joinv1.Challenge_TpmEncryptedCredential_case:
		return tpmEncryptedCredentialToMessage(resp.GetTpmEncryptedCredential()), nil
	case joinv1.Challenge_AzureChallenge_case:
		return azureChallengeToMessage(resp.GetAzureChallenge()), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge payload type %v", payload)
	}
}

func challengeFromMessage(resp messages.Response) (*joinv1.Challenge, error) {
	switch msg := resp.(type) {
	case *messages.BoundKeypairChallenge:
		return joinv1.Challenge_builder{
			BoundKeypairChallenge: proto.ValueOrDefault(boundKeypairChallengeFromMessage(msg)),
		}.Build(), nil
	case *messages.BoundKeypairRotationRequest:
		return joinv1.Challenge_builder{
			BoundKeypairRotationRequest: proto.ValueOrDefault(boundKeypairRotationRequestFromMessage(msg)),
		}.Build(), nil
	case *messages.IAMChallenge:
		return joinv1.Challenge_builder{
			IamChallenge: proto.ValueOrDefault(iamChallengeFromMessage(msg)),
		}.Build(), nil
	case *messages.OracleChallenge:
		return joinv1.Challenge_builder{
			OracleChallenge: proto.ValueOrDefault(oracleChallengeFromMessage(msg)),
		}.Build(), nil
	case *messages.TPMEncryptedCredential:
		return joinv1.Challenge_builder{
			TpmEncryptedCredential: proto.ValueOrDefault(tpmEncryptedCredentialFromMessage(msg)),
		}.Build(), nil
	case *messages.AzureChallenge:
		return joinv1.Challenge_builder{
			AzureChallenge: proto.ValueOrDefault(azureChallengeFromMessage(msg)),
		}.Build(), nil
	default:
		return nil, trace.BadParameter("unrecognized challenge message type %T", msg)
	}
}

func resultToMessage(resp *joinv1.Result) (messages.Response, error) {
	switch payload := resp.WhichPayload(); payload {
	case joinv1.Result_HostResult_case:
		return hostResultToMessage(resp.GetHostResult()), nil
	case joinv1.Result_BotResult_case:
		return botResultToMessage(resp.GetBotResult()), nil
	default:
		return nil, trace.BadParameter("unrecognized result payload type %v", payload)
	}
}

func hostResultToMessage(resp *joinv1.HostResult) *messages.HostResult {
	return &messages.HostResult{
		Certificates:       certificatesToMessage(resp.GetCertificates()),
		HostID:             resp.GetHostId(),
		ImmutableLabels:    resp.GetImmutableLabels(),
		BoundKeypairResult: boundKeypairResultToMessage(resp.GetBoundKeypairResult()),
	}
}

func hostResultFromMessage(msg *messages.HostResult) *joinv1.HostResult {
	return joinv1.HostResult_builder{
		Certificates:       certificatesFromMessage(&msg.Certificates),
		HostId:             msg.HostID,
		ImmutableLabels:    msg.ImmutableLabels,
		BoundKeypairResult: boundKeypairResultFromMessage(msg.BoundKeypairResult),
	}.Build()
}

func botResultToMessage(resp *joinv1.BotResult) *messages.BotResult {
	return &messages.BotResult{
		Certificates:       certificatesToMessage(resp.GetCertificates()),
		BoundKeypairResult: boundKeypairResultToMessage(resp.GetBoundKeypairResult()),
	}
}

func botResultFromMessage(msg *messages.BotResult) *joinv1.BotResult {
	return joinv1.BotResult_builder{
		Certificates:       certificatesFromMessage(&msg.Certificates),
		BoundKeypairResult: boundKeypairResultFromMessage(msg.BoundKeypairResult),
	}.Build()
}

func certificatesToMessage(certs *joinv1.Certificates) messages.Certificates {
	return messages.Certificates{
		TLSCert:    certs.GetTlsCert(),
		TLSCACerts: certs.GetTlsCaCerts(),
		SSHCert:    certs.GetSshCert(),
		SSHCAKeys:  certs.GetSshCaKeys(),
	}
}

func certificatesFromMessage(certs *messages.Certificates) *joinv1.Certificates {
	return joinv1.Certificates_builder{
		TlsCert:    certs.TLSCert,
		TlsCaCerts: certs.TLSCACerts,
		SshCert:    certs.SSHCert,
		SshCaKeys:  certs.SSHCAKeys,
	}.Build()
}

func givingUpToMessage(req *joinv1.GivingUp) *messages.GivingUp {
	reason := messages.GivingUpReasonUnspecified
	switch req.GetReason() {
	case joinv1.GivingUp_REASON_UNSUPPORTED_JOIN_METHOD:
		reason = messages.GivingUpReasonUnsupportedJoinMethod
	case joinv1.GivingUp_REASON_UNSUPPORTED_MESSAGE_TYPE:
		reason = messages.GivingUpReasonUnsupportedMessageType
	case joinv1.GivingUp_REASON_CHALLENGE_SOLUTION_FAILED:
		reason = messages.GivingUpReasonChallengeSolutionFailed
	}
	return &messages.GivingUp{
		Reason: reason,
		Msg:    req.GetMsg(),
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
	return joinv1.GivingUp_builder{
		Reason: reason,
		Msg:    msg.Msg,
	}.Build()
}
