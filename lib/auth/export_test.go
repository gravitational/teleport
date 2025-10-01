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

package auth

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/circleci"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/join/boundkeypair"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
)

// The items exported here exist solely to prevent import cycles and facilitate
// preexisting tests in lib/auth which relied on unexported items. All new
// tests in lib/auth should exist in the auth_test package and not rely on
// internal state.

const (
	NumOfRecoveryCodes     = numOfRecoveryCodes
	NumWordsInRecoveryCode = numWordsInRecoveryCode

	StartRecoveryGenericErrMsg  = startRecoveryGenericErrMsg
	StartRecoveryBadAuthnErrMsg = startRecoveryBadAuthnErrMsg

	VerifyRecoveryGenericErrMsg  = verifyRecoveryGenericErrMsg
	VerifyRecoveryBadAuthnErrMsg = verifyRecoveryBadAuthnErrMsg

	CompleteRecoveryGenericErrMsg = completeRecoveryGenericErrMsg

	MFADeviceNameMaxLen = mfaDeviceNameMaxLen

	ServerHostnameMaxLen = serverHostnameMaxLen

	MaxUserAgentLen = maxUserAgentLen
	ForwardedTag    = forwardedTag

	AzureAccessTokenAudience = azureAccessTokenAudience
)

var (
	ErrDeleteRoleUser       = errDeleteRoleUser
	ErrDeleteRoleCA         = errDeleteRoleCA
	ErrDeleteRoleAccessList = errDeleteRoleAccessList

	CreateAuditStreamAcceptedTotalMetric = createAuditStreamAcceptedTotalMetric

	AWSRSA2048CertBytes = awsRSA2048CertBytes
)

func (a *Server) SetRemoteClusterRefreshLimit(limit int) {
	remoteClusterRefreshLimit = limit
}

func (a *Server) RemoteClusterRefreshBuckets(buckets int) {
	remoteClusterRefreshBuckets = buckets
}

func (a *Server) VerifyRecoveryCode(ctx context.Context, username string, recoveryCode []byte) (errResult error) {
	return a.verifyRecoveryCode(ctx, username, recoveryCode)
}

func (a *Server) CreateRecoveryToken(ctx context.Context, username, tokenType string, usage types.UserTokenUsage) (types.UserToken, error) {
	return a.createRecoveryToken(ctx, username, tokenType, usage)
}

func (a *Server) NewUserToken(req authclient.CreateUserTokenRequest) (types.UserToken, error) {
	return a.newUserToken(req)
}

func CreatePrivilegeToken(ctx context.Context, srv *Server, username, tokenKind string) (*types.UserTokenV3, error) {
	return srv.createPrivilegeToken(ctx, username, tokenKind)
}

func (a *Server) GenerateAndUpsertRecoveryCodes(ctx context.Context, username string) (*proto.RecoveryCodes, error) {
	return a.generateAndUpsertRecoveryCodes(ctx, username)
}

func (p *HostAndUserCAPoolInfo) VerifyPeerCert() func([][]byte, [][]*x509.Certificate) error {
	return p.verifyPeerCert()
}

func (a *Server) CheckPassword(ctx context.Context, user string, password []byte, otpToken string) error {
	_, err := a.checkPassword(ctx, user, password, otpToken)
	return err
}

func (a *Server) SetPrivateKey(key []byte) {
	a.privateKey = key
}

func (a *Server) SyncUpgradeWindowStartHour(ctx context.Context) error {
	return a.syncUpgradeWindowStartHour(ctx)
}

func (a *Server) ValidateTrustedCluster(ctx context.Context, req *authclient.ValidateTrustedClusterRequest) (*authclient.ValidateTrustedClusterResponse, error) {
	return a.validateTrustedCluster(ctx, req)
}

func (a *Server) CreateReverseTunnel(ctx context.Context, t types.TrustedCluster) error {
	return a.createReverseTunnel(ctx, t)
}

func (a *Server) RefreshRemoteClusters(ctx context.Context) {
	a.refreshRemoteClusters(ctx)
}

func CreateGithubConnector(ctx context.Context, srv *Server, connector types.GithubConnector) (types.GithubConnector, error) {
	return srv.createGithubConnector(ctx, connector)
}

func UpdateGithubConnector(ctx context.Context, srv *Server, connector types.GithubConnector) (types.GithubConnector, error) {
	return srv.updateGithubConnector(ctx, connector)
}

func UpsertGithubConnector(ctx context.Context, srv *Server, connector types.GithubConnector) (types.GithubConnector, error) {
	return srv.upsertGithubConnector(ctx, connector)
}

func DeleteGithubConnector(ctx context.Context, srv *Server, name string) error {
	return srv.deleteGithubConnector(ctx, name)
}

func (a *Server) CalculateGithubUser(ctx context.Context, diagCtx *SSODiagContext, connector types.GithubConnector, claims *types.GithubClaims, request *types.GithubAuthRequest) (*CreateUserParams, error) {
	return a.calculateGithubUser(ctx, diagCtx, connector, claims, request)
}

func (a *Server) SubscribeToLockTarget(ctx context.Context, targets ...types.LockTarget) (types.Watcher, error) {
	return a.lockWatcher.Subscribe(ctx, targets...)
}

func (a *Server) NewWebSession(
	ctx context.Context,
	req NewWebSessionRequest,
	opts *newWebSessionOpts,
) (types.WebSession, services.AccessChecker, error) {
	return a.newWebSession(ctx, req, opts)
}

func (a *Server) AuthenticateUser(
	ctx context.Context,
	req authclient.AuthenticateUserRequest,
	requiredExt mfav1.ChallengeExtensions,
) (verifyLocks func(verifyMFADeviceLocksParams) error, mfaDev *types.MFADevice, user string, err error) {
	return a.authenticateUser(ctx, req, requiredExt)
}

func (a *Server) Inventory() *inventory.Controller {
	return a.inventory
}

func (a *Server) CheckPasswordWOToken(ctx context.Context, user string, password []byte) error {
	return a.checkPasswordWOToken(ctx, user, password)
}

func (a *Server) ResetPassword(ctx context.Context, username string) error {
	return a.resetPassword(ctx, username)
}

func (a *Server) SetHTTPClientForAWSSTS(clt utils.HTTPDoClient) {
	a.httpClientForAWSSTS = clt
}

func (a *Server) SetJWKSValidator(clt JWKSValidator) {
	a.k8sJWKSValidator = clt
}

func (a *Server) SetAzureDevopsIDTokenValidator(validator azureDevopsIDTokenValidator) {
	a.azureDevopsIDTokenValidator = validator
}

func (a *Server) SetBitbucketIDTokenValidator(validator bitbucketIDTokenValidator) {
	a.bitbucketIDTokenValidator = validator
}

func (a *Server) SetCircleCITokenValidate(validator func(ctx context.Context, organizationID, token string) (*circleci.IDTokenClaims, error)) {
	a.circleCITokenValidate = validator
}

func (a *Server) SetGCPIDTokenValidator(validator gcpIDTokenValidator) {
	a.gcpIDTokenValidator = validator
}

func (a *Server) SetGitlabIDTokenValidator(validator gitlabIDTokenValidator) {
	a.gitlabIDTokenValidator = validator
}

func (a *Server) SetK8sTokenReviewValidator(validator k8sTokenReviewValidator) {
	a.k8sTokenReviewValidator = validator
}

func (a *Server) SetSpaceliftIDTokenValidator(validator spaceliftIDTokenValidator) {
	a.spaceliftIDTokenValidator = validator
}

func (a *Server) SetTerraformIDTokenValidator(validator terraformCloudIDTokenValidator) {
	a.terraformIDTokenValidator = validator
}

func (a *Server) SetTPMValidator(validator func(ctx context.Context, log *slog.Logger, params tpm.ValidateParams) (*tpm.ValidatedTPM, error)) {
	a.tpmValidator = validator
}

func (a *Server) SetGHAIDTokenValidator(validator ghaIDTokenValidator) {
	a.ghaIDTokenValidator = validator
}

func (a *Server) SetGHAIDTokenJWKSValidator(validator ghaIDTokenJWKSValidator) {
	a.ghaIDTokenJWKSValidator = validator
}

func (a *Server) SetCreateBoundKeypairValidator(validator boundkeypair.CreateBoundKeypairValidator) {
	a.createBoundKeypairValidator = validator
}

func (a *Server) AuthenticateUserLogin(ctx context.Context, req authclient.AuthenticateUserRequest) (services.UserState, *services.SplitAccessChecker, error) {
	return a.authenticateUserLogin(ctx, req)
}

func (a *Server) RefreshULS(ctx context.Context, user types.User, ulsService services.UserLoginStates) (*userloginstate.UserLoginState, error) {
	return a.ulsGenerator.Refresh(ctx, user, ulsService)
}

func (a *Server) CreateGithubUser(ctx context.Context, p *CreateUserParams, dryRun bool) (types.User, error) {
	return a.createGithubUser(ctx, p, dryRun)
}

func BuildAPIEndpoint(apiEndpointURLStr string) (string, error) {
	return buildAPIEndpoint(apiEndpointURLStr)
}

func FormatGithubURL(host string, path string) string {
	return formatGithubURL(host, path)
}

func CheckGithubOrgSSOSupport(ctx context.Context, conn types.GithubConnector, userTeams []GithubTeamResponse, orgCache *utils.FnCache, client httpRequester) error {
	return checkGithubOrgSSOSupport(ctx, conn, userTeams, orgCache, client)
}

func ChangeUserAuthentication(ctx context.Context, a *Server, req *proto.ChangeUserAuthenticationRequest) (types.User, error) {
	return a.changeUserAuthentication(ctx, req)
}

func ValidateOracleJoinToken(token types.ProvisionToken) error {
	return validateOracleJoinToken(token)
}

func CreatePresetUsers(ctx context.Context, um PresetUsers) error {
	return createPresetUsers(ctx, um)
}

func CreatePresetRoles(ctx context.Context, um PresetRoleManager) error {
	return createPresetRoles(ctx, um)
}

func CreatePresetHealthCheckConfig(ctx context.Context, svc services.HealthCheckConfig) error {
	return createPresetHealthCheckConfig(ctx, svc)
}

func GetPresetUsers() []types.User {
	return getPresetUsers()
}

func CreatePresetDatabaseObjectImportRule(ctx context.Context, rules services.DatabaseObjectImportRules) error {
	return createPresetDatabaseObjectImportRule(ctx, rules)
}

func ValidServerHostname(hostname string) bool {
	return validServerHostname(hostname)
}

func FormatAccountName(s proxyDomainGetter, username string, authHostname string) (string, error) {
	return formatAccountName(s, username, authHostname)
}

func ConfigureCAsForTrustedCluster(tc types.TrustedCluster, cas []types.CertAuthority) {
	configureCAsForTrustedCluster(tc, cas)
}

func UpdateAccessRequestWithAdditionalReviewers(ctx context.Context, req types.AccessRequest, accessLists services.AccessListsGetter, promotions *types.AccessRequestAllowedPromotions) {
	updateAccessRequestWithAdditionalReviewers(ctx, req, accessLists, promotions)
}

func EncodeProquint(x uint16) string {
	return encodeProquint(x)
}

func EmitSSOLoginFailureEvent(ctx context.Context, emitter apievents.Emitter, method string, err error, testFlow bool) {
	emitSSOLoginFailureEvent(ctx, emitter, method, err, testFlow)
}

type UpsertServerRawReq = upsertServerRawReq

func UpsertServer(srv *APIServer, auth presenceForAPIServer, role types.SystemRole, r *http.Request, p httprouter.Params) (any, error) {
	return srv.upsertServer(auth, role, r, p)
}

func NewServerWithRoles(srv *Server, alog events.AuditLogSessionStreamer, authzContext authz.Context) *ServerWithRoles {
	return &ServerWithRoles{
		authServer: srv,
		alog:       alog,
		context:    authzContext,
	}
}

func NewKeySet(ctx context.Context, keyStore *keystore.Manager, caID types.CertAuthID) (types.CAKeySet, error) {
	return newKeySet(ctx, keyStore, caID)
}

func ValidateIdentity(c *TransportCredentials, conn net.Conn, tlsInfo *credentials.TLSInfo) (net.Conn, IdentityInfo, error) {
	return c.validateIdentity(conn, tlsInfo)
}

func (m *Middleware) SetLastRejectedTime(t time.Time) {
	m.lastRejectedAlertTime.Store(t.UnixNano())
}

func RegisterUsingOracleMethod(
	ctx context.Context,
	srv *Server,
	tokenReq *types.RegisterUsingTokenRequest,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
	fetchClaims oracleClaimsFetcher,
) (certs *proto.Certs, err error) {
	return srv.registerUsingOracleMethod(ctx, tokenReq, challengeResponse, fetchClaims)
}

func TrimUserAgent(userAgent string) string {
	return trimUserAgent(userAgent)
}

func IsAllowedDomain(cn string, domains []string) bool {
	return isAllowedDomain(cn, domains)
}

func GetSnowflakeJWTParams(ctx context.Context, accountName, userName string, publicKey []byte) (string, string) {
	return getSnowflakeJWTParams(ctx, accountName, userName, publicKey)
}

func FilterExtensions(ctx context.Context, logger *slog.Logger, extensions []pkix.Extension, oids ...asn1.ObjectIdentifier) []pkix.Extension {
	return filterExtensions(ctx, logger, extensions, oids...)
}

func PopulateGithubClaims(user *GithubUserResponse, teams []GithubTeamResponse) (*types.GithubClaims, error) {
	return populateGithubClaims(user, teams)
}

func ValidateGithubAuthCallbackHelper(ctx context.Context, m GitHubManager, diagCtx *SSODiagContext, q url.Values, emitter apievents.Emitter, logger *slog.Logger) (*authclient.GithubAuthResponse, error) {
	return validateGithubAuthCallbackHelper(ctx, m, diagCtx, q, emitter, logger)
}

func IsGCPZoneInLocation(rawLocation, rawZone string) bool {
	return isGCPZoneInLocation(rawLocation, rawZone)
}

func JoinRuleGlobMatch(want string, got string) (bool, error) {
	return joinRuleGlobMatch(want, got)
}

func FormatHeaderFromMap(m map[string]string) http.Header {
	return formatHeaderFromMap(m)
}

func CheckHeaders(headers http.Header, challenge string, clock clockwork.Clock) error {
	return checkHeaders(headers, challenge, clock)
}

func CheckOracleAllowRules(claims oracle.Claims, token string, allowRules []*types.ProvisionTokenSpecV2Oracle_Rule) error {
	return checkOracleAllowRules(claims, token, allowRules)
}

type GitHubManager = githubManager
type AWSIdentity = awsIdentity
type AttestedData = attestedData
type SignedAttestedData = signedAttestedData
type JWKSValidator = k8sJWKSValidator
type AzureRegisterOption = azureRegisterOption
type AzureRegisterConfig = azureRegisterConfig
type AzureVMClientGetter = vmClientGetter
type AzureVerifyTokenFunc = azureVerifyTokenFunc
type AccessTokenClaims = accessTokenClaims
type EC2Client = ec2Client
type EC2ClientKey = ec2ClientKey
type IAMRegisterOption = iamRegisterOption

func WithAzureCerts(certs []*x509.Certificate) AzureRegisterOption {
	return func(cfg *AzureRegisterConfig) {
		cfg.certificateAuthorities = certs
	}
}

func WithAzureVerifyFunc(verify azureVerifyTokenFunc) AzureRegisterOption {
	return func(cfg *AzureRegisterConfig) {
		cfg.verify = verify
	}
}

func WithAzureVMClientGetter(getVMClient vmClientGetter) AzureRegisterOption {
	return func(cfg *AzureRegisterConfig) {
		cfg.getVMClient = getVMClient
	}
}

func WithFIPS(b bool) iamRegisterOption {
	return withFips(b)
}

func WithAuthVersion(v *semver.Version) iamRegisterOption {
	return withAuthVersion(v)
}

func (s *TLSServer) GRPCServer() *GRPCServer {
	return s.grpcServer
}
