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

package server

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/utils/aws/organizations"
)

const (
	// AWSInstanceStateName represents the state of the AWS EC2
	// instance - (pending | running | shutting-down | terminated | stopping | stopped )
	// https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instances.html
	// Used for filtering instances for automatic EC2 discovery
	AWSInstanceStateName = "instance-state-name"

	awsEventPrefix = "aws/"
)

// EC2Instances contains information required to send SSM commands to EC2 instances
type EC2Instances struct {
	// Region is the AWS region where the instances are located.
	Region string
	// DocumentName is the SSM document that should be executed on the EC2
	// instances.
	DocumentName string
	// Parameters are parameters passed to the SSM document.
	Parameters map[string]string
	// AccountID is the AWS account the instances belong to.
	AccountID string
	// Instances is a list of discovered EC2 instances
	Instances []EC2Instance
	// Rotation is set so instances dont get filtered out for already
	// existing in the teleport instance
	Rotation bool

	// Integration is the integration used to fetch the Instance and should be used to access it.
	// Might be empty for instances that didn't use an Integration.
	Integration string
	// AssumeRoleARN is the ARN of the role to assume while installing.
	AssumeRoleARN string
	// ExternalID is the external ID to use when assuming a role.
	ExternalID string

	// DiscoveryConfigName is the DiscoveryConfig name which originated this Run Request.
	// Empty if using static matchers (coming from the `teleport.yaml`).
	DiscoveryConfigName string

	// EnrollMode is the mode used to enroll the instance into Teleport.
	EnrollMode types.InstallParamEnrollMode
}

// EC2Instance represents an AWS EC2 instance that has been
// discovered.
type EC2Instance struct {
	InstanceID       string
	InstanceName     string
	Tags             map[string]string
	OriginalInstance ec2types.Instance
}

func toEC2Instance(originalInst ec2types.Instance) EC2Instance {
	inst := EC2Instance{
		InstanceID:       aws.ToString(originalInst.InstanceId),
		Tags:             make(map[string]string, len(originalInst.Tags)),
		OriginalInstance: originalInst,
	}
	for _, tag := range originalInst.Tags {
		if key := aws.ToString(tag.Key); key != "" {
			inst.Tags[key] = aws.ToString(tag.Value)
			if key == "Name" {
				inst.InstanceName = aws.ToString(tag.Value)
			}
		}
	}
	return inst
}

// ToEC2Instances converts aws []*ec2.Instance to []EC2Instance
func ToEC2Instances(insts []ec2types.Instance) []EC2Instance {
	var ec2Insts []EC2Instance

	for _, inst := range insts {
		ec2Insts = append(ec2Insts, toEC2Instance(inst))
	}
	return ec2Insts
}

// ServerInfos creates a ServerInfo resource for each discovered instance.
func (i *EC2Instances) ServerInfos() ([]types.ServerInfo, error) {
	serverInfos := make([]types.ServerInfo, 0, len(i.Instances))
	for _, instance := range i.Instances {
		tags := make(map[string]string, len(instance.Tags))
		for k, v := range instance.Tags {
			tags[labels.FormatCloudLabelKey(labels.AWSLabelNamespace, k)] = v
		}

		si, err := types.NewServerInfo(types.Metadata{
			Name: types.ServerInfoNameFromAWS(i.AccountID, instance.InstanceID),
		}, types.ServerInfoSpecV1{
			NewLabels: tags,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverInfos = append(serverInfos, si)
	}

	return serverInfos, nil
}

// MakeEvents generates ResourceCreateEvents for these instances.
func (instances *EC2Instances) MakeEvents() map[string]*usageeventsv1.ResourceCreateEvent {
	resourceType := types.DiscoveredResourceNode

	switch instances.EnrollMode {
	case types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE:
		resourceType = types.DiscoveredResourceEICENode

	case types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT:
		if instances.DocumentName == types.AWSAgentlessInstallerDocument {
			resourceType = types.DiscoveredResourceAgentlessNode
		}
	}

	events := make(map[string]*usageeventsv1.ResourceCreateEvent, len(instances.Instances))
	for _, inst := range instances.Instances {
		events[awsEventPrefix+inst.InstanceID] = &usageeventsv1.ResourceCreateEvent{
			ResourceType:   resourceType,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  types.CloudAWS,
		}
	}
	return events
}

// EC2ClientGetter gets an AWS EC2 client for the given region.
type EC2ClientGetter func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error)

// RegionsListerGetter gets a list of AWS regions.
type RegionsListerGetter func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error)

// AWSOrganizationsGetter gets an AWS Organizations client used for listing accounts.
type AWSOrganizationsGetter func(ctx context.Context, opts ...awsconfig.OptionsFn) (organizations.OrganizationsClient, error)

// MatcherToEC2FetcherParams contains parameters for converting AWS EC2 Matchers
// into AWS EC2 Fetchers.
type MatcherToEC2FetcherParams struct {
	// Matchers is a list of AWS EC2 Matchers.
	Matchers []types.AWSMatcher
	// EC2ClientGetter gets an AWS EC2.
	EC2ClientGetter EC2ClientGetter
	// RegionsListerGetter gets a client that is capable of listing AWS regions.
	RegionsListerGetter RegionsListerGetter
	// AWSOrganizationsGetter gets a client that is capable of listing AWS organizations.
	AWSOrganizationsGetter AWSOrganizationsGetter
	// DiscoveryConfigName is the name of the DiscoveryConfig that contains the matchers.
	// Empty if using static matchers (coming from the `teleport.yaml`).
	DiscoveryConfigName string
	// PublicProxyAddrGetter returns the public proxy address to use for installation scripts.
	// This is only used if the matcher does not specify a ProxyAddress.
	// Example: proxy.example.com:3080 or proxy.example.com
	PublicProxyAddrGetter func(context.Context) (string, error)
	// Logger is the logger to use for the fetchers.
	Logger *slog.Logger
	// ReportIAMPermissionError is called when an AccessDenied error occurs
	// during EC2 discovery, allowing the caller to create a UserTask.
	ReportIAMPermissionError func(context.Context, *EC2IAMPermissionError)
}

// MatchersToEC2InstanceFetchers converts a list of AWS EC2 Matchers into a list of AWS EC2 Fetchers.
func MatchersToEC2InstanceFetchers(ctx context.Context, matcherParams MatcherToEC2FetcherParams) ([]Fetcher[*EC2Instances], error) {
	var ret []Fetcher[*EC2Instances]
	for _, matcher := range matcherParams.Matchers {
		fetcher := newEC2InstanceFetcher(ec2FetcherConfig{
			Matcher:                  matcher,
			ProxyPublicAddrGetter:    matcherParams.PublicProxyAddrGetter,
			EC2ClientGetter:          matcherParams.EC2ClientGetter,
			RegionsListerGetter:      matcherParams.RegionsListerGetter,
			AWSOrganizationsGetter:   matcherParams.AWSOrganizationsGetter,
			DiscoveryConfigName:      matcherParams.DiscoveryConfigName,
			Logger:                   matcherParams.Logger,
			ReportIAMPermissionError: matcherParams.ReportIAMPermissionError,
		})
		ret = append(ret, fetcher)
	}
	return ret, nil
}

// EC2IAMPermissionError represents an IAM permission error during EC2 discovery.
// This is used to report missing permissions so that UserTasks can be created.
type EC2IAMPermissionError struct {
	Integration         string
	AccountID           string
	Region              string
	IssueType           string
	DiscoveryConfigName string
	Err                 error
}

type ec2FetcherConfig struct {
	Matcher types.AWSMatcher
	// ProxyPublicAddrGetter returns the public proxy address to use for installation scripts.
	// This is only used if the matcher does not specify a ProxyAddress.
	// Example: proxy.example.com:3080 or proxy.example.com
	ProxyPublicAddrGetter  func(ctx context.Context) (string, error)
	EC2ClientGetter        EC2ClientGetter
	RegionsListerGetter    RegionsListerGetter
	AWSOrganizationsGetter AWSOrganizationsGetter
	DiscoveryConfigName    string
	Logger                 *slog.Logger
	// ReportIAMPermissionError is called when an AccessDenied error occurs
	// during EC2 discovery, allowing the caller to create a UserTask.
	ReportIAMPermissionError func(context.Context, *EC2IAMPermissionError)
}

type ec2InstanceFetcher struct {
	ec2FetcherConfig
	Filters []ec2types.Filter

	// cachedInstances keeps all of the ec2 instances that were matched
	// in the last run of GetInstances for use as a cache with
	// GetMatchingInstances
	cachedInstances *instancesCache
}

type instancesCache struct {
	sync.Mutex
	instances map[cachedInstanceKey]struct{}
}

func (ic *instancesCache) add(accountID, instanceID string) {
	ic.Lock()
	defer ic.Unlock()
	ic.instances[cachedInstanceKey{accountID: accountID, instanceID: instanceID}] = struct{}{}
}

func (ic *instancesCache) clear() {
	ic.Lock()
	defer ic.Unlock()
	ic.instances = make(map[cachedInstanceKey]struct{})
}

func (ic *instancesCache) exists(accountID, instanceID string) bool {
	ic.Lock()
	defer ic.Unlock()
	_, ok := ic.instances[cachedInstanceKey{accountID: accountID, instanceID: instanceID}]
	return ok
}

type cachedInstanceKey struct {
	accountID  string
	instanceID string
}

// SSM Run Command parameters for the TeleportDiscoveryInstaller SSM Document.
const (
	// ParamToken is the name of the invite token parameter sent in the SSM Document
	ParamToken = "token"
	// ParamScriptName is the name of the Teleport install script  sent in the SSM Document
	ParamScriptName = "scriptName"
	// ParamSSHDConfigPath is the path to the OpenSSH config file sent in the SSM Document
	ParamSSHDConfigPath = "sshdConfigPath"
	// ParamEnvVars is a parameter that contains environment variables to set before running the installation script.
	ParamEnvVars = "env"
)

// SSM Run Command parameters for the AWS-RunShellScript managed SSM Document.
const (
	// ParamCommands is the name of the commands parameter sent in the SSM Document.
	// This is a list of strings, which contain the command to execute.
	ParamCommands = "commands"
)

// awsEC2APIChunkSize is the max number of instances SSM will send commands to at a time
// This is used for limiting the number of instances for API Calls:
// ssm:SendCommand only accepts between 0 and 50.
// ssm:DescribeInstanceInformation only accepts between 5 and 50.
const awsEC2APIChunkSize = 50

func newEC2InstanceFetcher(cfg ec2FetcherConfig) *ec2InstanceFetcher {
	tagFilters := []ec2types.Filter{{
		Name:   aws.String(AWSInstanceStateName),
		Values: []string{string(ec2types.InstanceStateNameRunning)},
	}}

	if _, ok := cfg.Matcher.Tags[types.Wildcard]; !ok {
		for key, val := range cfg.Matcher.Tags {
			tagFilters = append(tagFilters, ec2types.Filter{
				Name:   aws.String("tag:" + key),
				Values: val,
			})
		}
	} else {
		slog.DebugContext(context.Background(), "Not setting any tag filters as there is a '*:...' tag present and AWS doesn't allow globbing on keys")
	}

	if cfg.Matcher.AssumeRole == nil {
		cfg.Matcher.AssumeRole = &types.AssumeRole{}
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &ec2InstanceFetcher{
		ec2FetcherConfig: cfg,
		Filters:          tagFilters,
		cachedInstances: &instancesCache{
			instances: map[cachedInstanceKey]struct{}{},
		},
	}
}

func ssmRunCommandParametersForCustomDocuments(cfg ec2FetcherConfig) map[string]string {
	if cfg.Matcher.Params == nil {
		cfg.Matcher.Params = &types.InstallerParams{}
	}

	parameters := map[string]string{
		ParamToken:      cfg.Matcher.Params.JoinToken,
		ParamScriptName: cfg.Matcher.Params.ScriptName,
	}

	envVars := envVarsFromInstallerParams(cfg.Matcher.Params)
	if len(envVars) > 0 {
		parameters[ParamEnvVars] = strings.Join(envVars, " ")
	}

	if !cfg.Matcher.Params.InstallTeleport {
		parameters[ParamSSHDConfigPath] = cfg.Matcher.Params.SSHDConfig
	}

	return parameters
}

func ssmRunCommandParameters(ctx context.Context, cfg ec2FetcherConfig) (map[string]string, error) {
	if cfg.Matcher.SSM.DocumentName == types.AWSSSMDocumentRunShellScript {
		// When using the pre-defined SSM Document AWS-RunShellScript, only the commands parameter is required.
		// It contains the full installation script that will be executed on the instance.
		script, err := installerScript(ctx, cfg.Matcher.Params, withProxyAddrGetter(cfg.ProxyPublicAddrGetter))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return map[string]string{
			ParamCommands: script,
		}, nil
	}

	// For custom SSM Documents, scriptName, token and env are required.
	return ssmRunCommandParametersForCustomDocuments(cfg), nil
}

// GetMatchingInstances returns a list of EC2 instances from a list of matching Teleport nodes
func (f *ec2InstanceFetcher) GetMatchingInstances(ctx context.Context, nodes []types.Server, rotation bool) ([]*EC2Instances, error) {
	ssmRunParams, err := ssmRunCommandParameters(ctx, f.ec2FetcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	instancesByRegion := make(map[string]EC2Instances)

	for _, node := range nodes {
		// Heartbeating and expiration keeps Teleport Agents up to date, no need to consider those nodes.
		// Agentless and EICE Nodes don't heartbeat, so they must be manually managed by the DiscoveryService.
		if !types.IsOpenSSHNodeSubKind(node.GetSubKind()) {
			continue
		}
		region, ok := node.GetLabel(types.AWSInstanceRegion)
		if !ok {
			continue
		}
		instID, ok := node.GetLabel(types.AWSInstanceIDLabel)
		if !ok {
			continue
		}
		accountID, ok := node.GetLabel(types.AWSAccountIDLabel)
		if !ok {
			continue
		}

		if !f.cachedInstances.exists(accountID, instID) {
			continue
		}

		if _, ok := instancesByRegion[region]; !ok {
			instancesByRegion[region] = EC2Instances{
				Region:              region,
				DocumentName:        f.Matcher.SSM.DocumentName,
				Parameters:          ssmRunParams,
				Rotation:            rotation,
				Integration:         f.Matcher.Integration,
				DiscoveryConfigName: f.DiscoveryConfigName,
				AccountID:           accountID,
			}
		}
		insts := instancesByRegion[region]
		insts.Instances = append(insts.Instances, EC2Instance{
			InstanceID: instID,
		})

		instancesByRegion[region] = insts
	}

	if len(instancesByRegion) == 0 {
		return nil, trace.NotFound("no ec2 instances found")
	}

	return chunkInstances(instancesByRegion), nil
}

// chunkInstances splits instances into chunks of 50.
// This is required because SSM SendCommand API calls only accept up to 50 instance IDs at a time.
func chunkInstances(instancesByRegion map[string]EC2Instances) []*EC2Instances {
	var instColl []*EC2Instances
	for _, insts := range instancesByRegion {
		for i := 0; i < len(insts.Instances); i += awsEC2APIChunkSize {
			end := min(i+awsEC2APIChunkSize, len(insts.Instances))
			inst := &EC2Instances{
				AccountID:           insts.AccountID,
				Region:              insts.Region,
				DocumentName:        insts.DocumentName,
				Parameters:          insts.Parameters,
				Instances:           insts.Instances[i:end],
				Rotation:            insts.Rotation,
				Integration:         insts.Integration,
				DiscoveryConfigName: insts.DiscoveryConfigName,
			}
			instColl = append(instColl, inst)
		}
	}
	return instColl
}

func (f *ec2InstanceFetcher) matcherRegions(ctx context.Context, awsOpts []awsconfig.OptionsFn) ([]string, error) {
	if !f.Matcher.IsRegionWildcard() {
		return f.Matcher.Regions, nil
	}

	regionsListerClient, err := f.RegionsListerGetter(ctx, awsOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	paginator := account.NewListRegionsPaginator(regionsListerClient, &account.ListRegionsInput{
		RegionOptStatusContains: []accounttypes.RegionOptStatus{
			accounttypes.RegionOptStatusEnabled,
			accounttypes.RegionOptStatusEnabledByDefault,
		},
	})

	var enabledRegions []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			convertedErr := libcloudaws.ConvertRequestFailureError(err)
			if trace.IsAccessDenied(convertedErr) {
				if f.ReportIAMPermissionError != nil {
					f.ReportIAMPermissionError(ctx, &EC2IAMPermissionError{
						Integration:         f.Matcher.Integration,
						IssueType:           usertasks.AutoDiscoverEC2IssuePermAccountListRegions,
						DiscoveryConfigName: f.DiscoveryConfigName,
						Err:                 convertedErr,
					})
				}
				return nil, trace.BadParameter("Missing account:ListRegions permission in IAM Role, which is required to iterate over all regions. " +
					"Add this permission to the IAM Role, or enumerate all the regions in the AWS matcher.")
			}
			return nil, convertedErr
		}

		for _, region := range page.Regions {
			enabledRegions = append(enabledRegions, aws.ToString(region.RegionName))
		}
	}

	return enabledRegions, nil
}

func (f *ec2InstanceFetcher) fetchAccountIDsUnderOrganization(ctx context.Context) ([]string, error) {
	awsOpts := []awsconfig.OptionsFn{
		awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{Name: f.Matcher.Integration}),
	}

	var organizationID string
	var includeOUs []string
	var excludeOUs []string
	organizationID = f.Matcher.Organization.OrganizationID
	if f.Matcher.Organization.OrganizationalUnits != nil {
		includeOUs = f.Matcher.Organization.OrganizationalUnits.Include
		excludeOUs = f.Matcher.Organization.OrganizationalUnits.Exclude
	}

	orgsClient, err := f.AWSOrganizationsGetter(ctx, awsOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accountIDs, err := organizations.MatchingAccounts(ctx, f.Logger, orgsClient, organizations.MatchingAccountsFilter{
		IncludeOUs:     includeOUs,
		ExcludeOUs:     excludeOUs,
		OrganizationID: organizationID,
	})
	if err != nil {
		convertedErr := libcloudaws.ConvertRequestFailureError(err)
		if trace.IsAccessDenied(convertedErr) {
			if f.ReportIAMPermissionError != nil {
				f.ReportIAMPermissionError(ctx, &EC2IAMPermissionError{
					Integration:         f.Matcher.Integration,
					IssueType:           usertasks.AutoDiscoverEC2IssuePermOrganizations,
					DiscoveryConfigName: f.DiscoveryConfigName,
					Err:                 convertedErr,
				})
			}
			return nil, trace.BadParameter("discovering instances under an organization requires the following permissions: [%s], add those to the IAM Role used by the Discovery Service", strings.Join(organizations.RequiredAPIs(), ", "))
		}

		return nil, trace.Wrap(convertedErr)
	}

	return accountIDs, nil
}

type assumeRoleWithExternalID struct {
	RoleARN    string
	ExternalID string
}

// allAssumeRoles returns a list of all the AWS Assume Roles that must be assumed.
// There's a special case when there is no Role to Assume, in this case an empty string is returned.
// In this situation no AssumeRole should be passed to the AWS client.
func (f *ec2InstanceFetcher) allAssumeRoles(ctx context.Context) ([]assumeRoleWithExternalID, error) {
	if !f.Matcher.HasOrganizationMatcher() {
		// When targeting a single Account (ie, no account discovery / no organization account matcher)
		// the discovery service can either use the current IAM Role or assume another IAM Role.
		// If defined, then the Assume Role ARN is returned, otherwise an empty string is returned.
		// An empty string is used to indicate that no Assume Role should be used.
		var roleARN string
		var externalID string
		if f.Matcher.AssumeRole != nil {
			roleARN = f.Matcher.AssumeRole.RoleARN
			externalID = f.Matcher.AssumeRole.ExternalID
		}
		return []assumeRoleWithExternalID{{RoleARN: roleARN, ExternalID: externalID}}, nil
	}

	if f.Matcher.AssumeRole == nil || f.Matcher.AssumeRole.RoleName == "" {
		return nil, trace.BadParameter("assume role name is required when using AWS organization discovery")
	}

	accountIDs, err := f.fetchAccountIDsUnderOrganization(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var allAssumeRoles []assumeRoleWithExternalID
	for _, accountID := range accountIDs {
		assumeRoleARN := arn.ARN{
			Partition: "aws",
			Service:   "iam",
			Region:    "",
			AccountID: accountID,
			Resource:  "role/" + f.Matcher.AssumeRole.RoleName,
		}

		allAssumeRoles = append(allAssumeRoles, assumeRoleWithExternalID{
			RoleARN:    assumeRoleARN.String(),
			ExternalID: f.Matcher.AssumeRole.ExternalID,
		})
	}

	return allAssumeRoles, nil
}

// GetInstances fetches all EC2 instances matching configured filters.
func (f *ec2InstanceFetcher) GetInstances(ctx context.Context, rotation bool) ([]*EC2Instances, error) {
	ssmRunParams, err := ssmRunCommandParameters(ctx, f.ec2FetcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.cachedInstances.clear()
	var allInstances []*EC2Instances

	accountRolesToAssume, err := f.allAssumeRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, assumeRole := range accountRolesToAssume {
		awsOpts := []awsconfig.OptionsFn{
			awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{Name: f.Matcher.Integration}),
			awsconfig.WithAssumeRole(assumeRole.RoleARN, assumeRole.ExternalID),
		}

		regions, err := f.matcherRegions(ctx, awsOpts)
		if err != nil {
			f.Logger.WarnContext(ctx, "Failed to get regions for EC2 discovery",
				"assume_role_arn", assumeRole.RoleARN,
				"error", err,
			)
			continue
		}

		for _, region := range regions {
			regionInstances, err := f.getInstancesInRegion(ctx, getInstancesInRegionParams{
				rotation:     rotation,
				region:       region,
				assumeRole:   assumeRole,
				awsOpts:      awsOpts,
				ssmRunParams: ssmRunParams,
			})
			if err != nil {
				f.Logger.WarnContext(ctx, "Failed to get instances for EC2 discovery",
					"region", region,
					"assume_role_arn", assumeRole.RoleARN,
					"error", err,
				)
				continue
			}

			allInstances = append(allInstances, regionInstances...)
		}
	}

	if len(allInstances) == 0 {
		return nil, trace.NotFound("no ec2 instances found")
	}

	return allInstances, nil
}

type getInstancesInRegionParams struct {
	rotation     bool
	region       string
	assumeRole   assumeRoleWithExternalID
	awsOpts      []awsconfig.OptionsFn
	ssmRunParams map[string]string
}

// getInstancesInRegion fetches all EC2 instances in a given region.
func (f *ec2InstanceFetcher) getInstancesInRegion(ctx context.Context, params getInstancesInRegionParams) ([]*EC2Instances, error) {
	ec2Client, err := f.EC2ClientGetter(ctx, params.region, params.awsOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var instances []*EC2Instances

	paginator := ec2.NewDescribeInstancesPaginator(ec2Client, &ec2.DescribeInstancesInput{
		Filters: f.Filters,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			convertedErr := libcloudaws.ConvertRequestFailureError(err)
			if trace.IsAccessDenied(convertedErr) {
				if f.ReportIAMPermissionError != nil {
					errInfo := &EC2IAMPermissionError{
						Integration:         f.Matcher.Integration,
						Region:              params.region,
						IssueType:           usertasks.AutoDiscoverEC2IssuePermDescribeInstances,
						DiscoveryConfigName: f.DiscoveryConfigName,
						Err:                 convertedErr,
					}
					// Derive AccountID from assume role ARN when available.
					if params.assumeRole.RoleARN != "" {
						if parsed, parseErr := arn.Parse(params.assumeRole.RoleARN); parseErr == nil {
							errInfo.AccountID = parsed.AccountID
						}
					}
					f.ReportIAMPermissionError(ctx, errInfo)
				}
			}
			return nil, convertedErr
		}

		for _, res := range page.Reservations {
			for i := 0; i < len(res.Instances); i += awsEC2APIChunkSize {
				end := min(i+awsEC2APIChunkSize, len(res.Instances))
				ownerID := aws.ToString(res.OwnerId)
				inst := &EC2Instances{
					AccountID:           ownerID,
					Region:              params.region,
					DocumentName:        f.Matcher.SSM.DocumentName,
					Instances:           ToEC2Instances(res.Instances[i:end]),
					Parameters:          params.ssmRunParams,
					Rotation:            params.rotation,
					Integration:         f.Matcher.Integration,
					AssumeRoleARN:       params.assumeRole.RoleARN,
					ExternalID:          params.assumeRole.ExternalID,
					DiscoveryConfigName: f.DiscoveryConfigName,
					EnrollMode:          f.Matcher.Params.EnrollMode,
				}
				for _, ec2inst := range res.Instances[i:end] {
					f.cachedInstances.add(ownerID, aws.ToString(ec2inst.InstanceId))
				}
				instances = append(instances, inst)
			}
		}
	}

	return instances, nil
}

// GetDiscoveryConfigName returns the discovery config name that created this fetcher.
func (f *ec2InstanceFetcher) GetDiscoveryConfigName() string {
	return f.DiscoveryConfigName
}

// IntegrationName identifies the integration name whose credentials were used to fetch the resources.
// Might be empty when the fetcher is using ambient credentials.
func (f *ec2InstanceFetcher) IntegrationName() string {
	return f.Matcher.Integration
}
