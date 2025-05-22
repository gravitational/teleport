/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"cmp"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	armpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/digitorus/pkcs7"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	liboidc "github.com/gravitational/teleport/lib/oidc"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	azureAccessTokenAudience = "https://management.azure.com/"

	// azureUserAgent specifies the Azure User-Agent identification for telemetry.
	azureUserAgent = "teleport"
	// azureVirtualMachine specifies the Azure virtual machine resource type.
	azureVirtualMachine = "virtualMachines"
)

// Structs for unmarshaling attested data. Schema can be found at
// https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#response-2

type signedAttestedData struct {
	Encoding  string `json:"encoding"`
	Signature string `json:"signature"`
}

type plan struct {
	Name      string `json:"name"`
	Product   string `json:"product"`
	Publisher string `json:"publisher"`
}

type timestamp struct {
	CreatedOn string `json:"createdOn"`
	ExpiresOn string `json:"expiresOn"`
}

type attestedData struct {
	LicenseType    string    `json:"licenseType"`
	Nonce          string    `json:"nonce"`
	Plan           plan      `json:"plan"`
	Timestamp      timestamp `json:"timestamp"`
	ID             string    `json:"vmId"`
	SubscriptionID string    `json:"subscriptionId"`
	SKU            string    `json:"sku"`
}

type accessTokenClaims struct {
	oidc.TokenClaims
	TenantID string `json:"tid"`
	Version  string `json:"ver"`

	// Azure JWT tokens include two optional claims that can be used to validate
	// the subscription and resource group of a joining node. These claims hold
	// different values depending on the assigned Managed Identity of the Azure VM:
	// - xms_mirid:
	//   - For System-Assigned Identity it represents the resource id of the VM.
	//   - For User-Assigned Identity it represents the resource id of the user-assigned identity.
	// - xms_az_rid:
	//   - For System-Assigned Identity this claim is omitted.
	//   - For User-Assigned Identity it represents the resource id of the VM.
	//
	// More details at: https://learn.microsoft.com/en-us/answers/questions/1282788/existence-of-xms-az-rid-field-in-activity-logs-of

	ManangedIdentityResourceID string `json:"xms_mirid"`
	AzureResourceID            string `json:"xms_az_rid"`
}

func (c *accessTokenClaims) AsJWTClaims() jwt.Claims {
	return jwt.Claims{
		Issuer:    c.Issuer,
		Subject:   c.Subject,
		Audience:  jwt.Audience(c.Audience),
		Expiry:    jwt.NewNumericDate(c.Expiration.AsTime()),
		NotBefore: jwt.NewNumericDate(c.NotBefore.AsTime()),
		IssuedAt:  jwt.NewNumericDate(c.IssuedAt.AsTime()),
		ID:        c.JWTID,
	}
}

type azureVerifyTokenFunc func(ctx context.Context, rawIDToken string) (*accessTokenClaims, error)

type vmClientGetter func(subscriptionID string, token *azure.StaticCredential) (azure.VirtualMachinesClient, error)

type azureRegisterConfig struct {
	certificateAuthorities []*x509.Certificate
	verify                 azureVerifyTokenFunc
	getVMClient            vmClientGetter
}

func azureVerifyFuncFromOIDCVerifier(clientID string) azureVerifyTokenFunc {
	return func(ctx context.Context, rawIDToken string) (*accessTokenClaims, error) {
		token, err := jwt.ParseSigned(rawIDToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Need to get the tenant ID before we verify so we can construct the issuer URL.
		var unverifiedClaims accessTokenClaims
		if err := token.UnsafeClaimsWithoutVerification(&unverifiedClaims); err != nil {
			return nil, trace.Wrap(err)
		}
		issuer, err := url.JoinPath("https://sts.windows.net", unverifiedClaims.TenantID, "/")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return liboidc.ValidateToken[*accessTokenClaims](ctx, issuer, clientID, rawIDToken)
	}
}

func (cfg *azureRegisterConfig) CheckAndSetDefaults(ctx context.Context) error {
	if cfg.verify == nil {
		cfg.verify = azureVerifyFuncFromOIDCVerifier(azureAccessTokenAudience)
	}

	if cfg.certificateAuthorities == nil {
		certs, err := getAzureRootCerts()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.certificateAuthorities = certs
	}
	if cfg.getVMClient == nil {
		cfg.getVMClient = func(subscriptionID string, token *azure.StaticCredential) (azure.VirtualMachinesClient, error) {
			// The User-Agent is added for debugging purposes. It helps identify
			// and isolate teleport traffic.
			opts := &armpolicy.ClientOptions{
				ClientOptions: policy.ClientOptions{
					Telemetry: policy.TelemetryOptions{
						ApplicationID: azureUserAgent,
					},
				},
			}
			client, err := azure.NewVirtualMachinesClient(subscriptionID, token, opts)
			return client, trace.Wrap(err)
		}
	}
	return nil
}

type azureRegisterOption func(cfg *azureRegisterConfig)

// parseAndVeryAttestedData verifies that an attested data document was signed
// by Azure. If verification is successful, it returns the ID of the VM that
// produced the document.
func parseAndVerifyAttestedData(ctx context.Context, adBytes []byte, challenge string, certs []*x509.Certificate) (subscriptionID, vmID string, err error) {
	var signedAD signedAttestedData
	if err := utils.FastUnmarshal(adBytes, &signedAD); err != nil {
		return "", "", trace.Wrap(err)
	}
	if signedAD.Encoding != "pkcs7" {
		return "", "", trace.AccessDenied("unsupported signature type: %v", signedAD.Encoding)
	}

	sigPEM := "-----BEGIN PKCS7-----\n" + signedAD.Signature + "\n-----END PKCS7-----"
	sigBER, _ := pem.Decode([]byte(sigPEM))
	if sigBER == nil {
		return "", "", trace.AccessDenied("unable to decode attested data document")
	}

	p7, err := pkcs7.Parse(sigBER.Bytes)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	var ad attestedData
	if err := utils.FastUnmarshal(p7.Content, &ad); err != nil {
		return "", "", trace.Wrap(err)
	}
	if ad.Nonce != challenge {
		return "", "", trace.AccessDenied("challenge is missing or does not match")
	}

	if len(p7.Certificates) == 0 {
		return "", "", trace.AccessDenied("no certificates for signature")
	}
	fixAzureSigningAlgorithm(p7)

	// Azure only sends the leaf cert, so we have to fetch the intermediate.
	intermediate, err := getAzureIssuerCert(ctx, p7.Certificates[0])
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if intermediate != nil {
		p7.Certificates = append(p7.Certificates, intermediate)
	}

	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}

	if err := p7.VerifyWithChain(pool); err != nil {
		return "", "", trace.Wrap(err)
	}

	return ad.SubscriptionID, ad.ID, nil
}

// verifyVMIdentity verifies that the provided access token came from the
// correct Azure VM. Returns the Azure join attributes
func verifyVMIdentity(
	ctx context.Context,
	cfg *azureRegisterConfig,
	accessToken,
	subscriptionID,
	vmID string,
	requestStart time.Time,
	logger *slog.Logger,
) (joinAttrs *workloadidentityv1pb.JoinAttrsAzure, err error) {
	tokenClaims, err := cfg.verify(ctx, accessToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	expectedIssuer, err := url.JoinPath("https://sts.windows.net", tokenClaims.TenantID, "/")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// v2 tokens have the version appended to the issuer.
	if tokenClaims.Version == "2.0" {
		expectedIssuer, err = url.JoinPath(expectedIssuer, "2.0")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	expectedClaims := jwt.Expected{
		Issuer:   expectedIssuer,
		Audience: jwt.Audience{azureAccessTokenAudience},
		Time:     requestStart,
	}

	if err := tokenClaims.AsJWTClaims().Validate(expectedClaims); err != nil {
		return nil, trace.Wrap(err)
	}

	// Listing all VMs in an Azure subscription during the verification process
	// is problematic when there are a large number of VMs in an Azure subscription.
	// In some cases this can lead to throttling due to Azure API rate limits.
	// To address the issue, the verification process will first attempt to
	// parse required VM identifiers from the token claims. If this method fails,
	// fallback to the original method of listing VMs and parsing the VM identifiers
	// from the VM resource.
	vmSubscription, vmResourceGroup, err := claimsToIdentifiers(tokenClaims)
	if err == nil {
		if subscriptionID != vmSubscription {
			return nil, trace.AccessDenied("subscription ID mismatch between attested data and access token")
		}
		return azureJoinToAttrs(vmSubscription, vmResourceGroup), nil
	}
	logger.WarnContext(ctx, "Failed to parse VM identifiers from claims. Retrying with Azure VM API.",
		"error", err)

	tokenCredential := azure.NewStaticCredential(azcore.AccessToken{
		Token:     accessToken,
		ExpiresOn: tokenClaims.GetExpiration(),
	})
	vmClient, err := cfg.getVMClient(subscriptionID, tokenCredential)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceID, err := arm.ParseResourceID(tokenClaims.ManangedIdentityResourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var vm *azure.VirtualMachine

	// If the token is from the system-assigned managed identity, the resource ID
	// is for the VM itself and we can use it to look up the VM.
	// This will also match scale set VMs (VMSS), the vmClient is responsible
	// for properly retrieving their information.
	if slices.Contains(resourceID.ResourceType.Types, azureVirtualMachine) {
		vm, err = vmClient.Get(ctx, tokenClaims.ManangedIdentityResourceID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if vm.VMID != vmID {
			return nil, trace.AccessDenied("vm ID does not match")
		}

		// If the token is from a user-assigned managed identity, the resource ID is
		// for the identity and we need to look the VM up by VM ID.
	} else {
		vm, err = vmClient.GetByVMID(ctx, vmID)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.AccessDenied("no VM found with matching VM ID")
			}
			return nil, trace.Wrap(err)
		}
	}
	return azureJoinToAttrs(vm.Subscription, vm.ResourceGroup), nil
}

// claimsToIdentifiers returns the vm identifiers from the provided claims.
func claimsToIdentifiers(tokenClaims *accessTokenClaims) (subscriptionID, resourceGroupID string, err error) {
	// xms_az_rid claim is omitted when the VM is assigned a System-Assigned Identity.
	// The xms_mirid claim should be used instead.
	rid := cmp.Or(tokenClaims.AzureResourceID, tokenClaims.ManangedIdentityResourceID)
	resourceID, err := arm.ParseResourceID(rid)
	if err != nil {
		return "", "", trace.Wrap(err, "failed to parse resource id from claims")
	}
	if !slices.Contains(resourceID.ResourceType.Types, azureVirtualMachine) {
		return "", "", trace.BadParameter("unexpected resource type: %q", resourceID.ResourceType.Type)
	}
	return resourceID.SubscriptionID, resourceID.ResourceGroupName, nil
}

func checkAzureAllowRules(vmID string, attrs *workloadidentityv1pb.JoinAttrsAzure, token *types.ProvisionTokenV2) error {
	for _, rule := range token.Spec.Azure.Allow {
		if rule.Subscription != attrs.Subscription {
			continue
		}
		if !azureResourceGroupIsAllowed(rule.ResourceGroups, attrs.ResourceGroup) {
			continue
		}
		return nil
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", vmID, token.GetName())
}
func azureResourceGroupIsAllowed(allowedResourceGroups []string, vmResourceGroup string) bool {
	if len(allowedResourceGroups) == 0 {
		return true
	}

	// ResourceGroups are case insensitive.
	// https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/frequently-asked-questions#are-resource-group-names-case-sensitive
	// The API returns them using capital case, but docs don't mention a specific case.
	// Converting everything to the same case will ensure a proper comparison.
	resourceGroup := strings.ToUpper(vmResourceGroup)
	for _, allowedResourceGroup := range allowedResourceGroups {
		if strings.EqualFold(resourceGroup, allowedResourceGroup) {
			return true
		}
	}

	return false
}

func azureJoinToAttrs(subscriptionID, resourceGroupID string) *workloadidentityv1pb.JoinAttrsAzure {
	return &workloadidentityv1pb.JoinAttrsAzure{
		Subscription:  subscriptionID,
		ResourceGroup: resourceGroupID,
	}
}

func (a *Server) checkAzureRequest(
	ctx context.Context,
	challenge string,
	req *proto.RegisterUsingAzureMethodRequest,
	cfg *azureRegisterConfig,
) (*workloadidentityv1pb.JoinAttrsAzure, error) {
	requestStart := a.clock.Now()
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodAzure {
		return nil, trace.AccessDenied("this token does not support the Azure join method")
	}
	token, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("azure join method only supports ProvisionTokenV2, '%T' was provided", provisionToken)
	}

	subID, vmID, err := parseAndVerifyAttestedData(ctx, req.AttestedData, challenge, cfg.certificateAuthorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attrs, err := verifyVMIdentity(ctx, cfg, req.AccessToken, subID, vmID, requestStart, a.logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAzureAllowRules(vmID, attrs, token); err != nil {
		return attrs, trace.Wrap(err)
	}

	return attrs, nil
}

func generateAzureChallenge() (string, error) {
	challenge, err := generateChallenge(base64.RawURLEncoding, 24)
	return challenge, trace.Wrap(err)
}

// RegisterUsingAzureMethodWithOpts registers the caller using the Azure join method
// and returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *proto.RegisterUsingAzureMethodRequest with a signed attested data document
// including the challenge as a nonce.
func (a *Server) RegisterUsingAzureMethodWithOpts(
	ctx context.Context,
	challengeResponse client.RegisterAzureChallengeResponseFunc,
	opts ...azureRegisterOption,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, nil, joinRequest)
		}
	}()

	cfg := &azureRegisterConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if err := cfg.CheckAndSetDefaults(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	challenge, err := generateAzureChallenge()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinRequest = req.RegisterUsingTokenRequest

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	joinAttrs, err := a.checkAzureRequest(ctx, challenge, req, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		certs, _, err := a.generateCertsBot(
			ctx,
			provisionToken,
			req.RegisterUsingTokenRequest,
			nil,
			&workloadidentityv1pb.JoinAttrs{
				Azure: joinAttrs,
			},
		)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(
		ctx,
		provisionToken,
		req.RegisterUsingTokenRequest,
		nil,
	)
	return certs, trace.Wrap(err)
}

// RegisterUsingAzureMethod registers the caller using the Azure join method
// and returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *proto.RegisterUsingAzureMethodRequest with a signed attested data document
// including the challenge as a nonce.
func (a *Server) RegisterUsingAzureMethod(
	ctx context.Context,
	challengeResponse client.RegisterAzureChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	return a.RegisterUsingAzureMethodWithOpts(ctx, challengeResponse)
}

// fixAzureSigningAlgorithm fixes a mismatch between the object IDs of the
// hashing algorithm sent by Azure vs the ones expected by the pkcs7 library.
// Specifically, Azure (incorrectly?) sends a [digest encryption algorithm]
// where the pkcs7 structure's [signerInfo] expects a [digest algorithm].
//
// [signerInfo]: https://www.rfc-editor.org/rfc/rfc2315#section-6.4
// [digest algorithm]: https://www.rfc-editor.org/rfc/rfc2315#section-6.3
// [digest encryption algorithm]: https://www.rfc-editor.org/rfc/rfc2315#section-6.4
func fixAzureSigningAlgorithm(p7 *pkcs7.PKCS7) {
	for i, signer := range p7.Signers {
		if signer.DigestAlgorithm.Algorithm.Equal(pkcs7.OIDEncryptionAlgorithmRSASHA256) {
			p7.Signers[i].DigestAlgorithm.Algorithm = pkcs7.OIDDigestAlgorithmSHA256
		}
	}
}
